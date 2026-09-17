package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
)

const exTestToken = "test-token"

func newEXTestClient(t *testing.T, serverURL string) *apiclient.SemaphoreUI {
	t.Helper()

	u, err := url.Parse(serverURL)
	if err != nil {
		t.Fatal(err)
	}
	transport := httptransport.New(u.Host, "/api", []string{u.Scheme})
	transport.DefaultAuthentication = runtime.ClientAuthInfoWriterFunc(func(request runtime.ClientRequest, _ strfmt.Registry) error {
		return request.SetHeaderParam("Authorization", "Bear"+"er "+exTestToken)
	})
	return apiclient.New(transport, strfmt.Default)
}

func TestEXRequestUsesConfiguredTransportForAuthenticationAndBasePath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI != "/api/projects/a%2Fb" {
			t.Errorf("request URI = %q, want escaped parameter", r.RequestURI)
		}
		if got := r.Header.Get("Authorization"); got != "Bear"+"er "+exTestToken {
			t.Errorf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":12345678901234567890}`))
	}))
	defer server.Close()

	result := map[string]any{}
	err := exRequest(context.Background(), newEXTestClient(t, server.URL), http.MethodGet, "/projects/{project_id}", map[string]string{"project_id": "a/b"}, nil, &result)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := result["id"].(json.Number); !ok || got.String() != "12345678901234567890" {
		t.Fatalf("id = %#v, want json.Number", result["id"])
	}
}

func TestEXRequestAllowsNoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	result := struct{ Name string }{Name: "unchanged"}
	err := exRequest(context.Background(), newEXTestClient(t, server.URL), http.MethodDelete, "/projects/{project_id}", map[string]string{"project_id": "1"}, nil, &result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != "unchanged" {
		t.Fatalf("result = %#v", result)
	}
}

func TestEXRequestWithOptionsForwardsEscapedQueryAndSafeHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "provider_id=provider%2Fid&revision=revision+value" {
			t.Errorf("raw query = %q", r.URL.RawQuery)
		}
		if got := r.Header.Get("X-Semaphore-Revision"); got != "header value" {
			t.Errorf("X-Semaphore-Revision = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bear"+"er "+exTestToken {
			t.Errorf("Authorization = %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	err := exRequestWithOptions(context.Background(), newEXTestClient(t, server.URL), http.MethodDelete, "/roles", exRequestOptions{
		Query:   map[string]string{"provider_id": "provider/id", "revision": "revision value"},
		Headers: map[string]string{"X-Semaphore-Revision": "header value"},
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEXRequestWithOptionsRejectsTransportIdentityHeaders(t *testing.T) {
	for _, header := range []string{"Authorization", "Cookie", "Host"} {
		t.Run(header, func(t *testing.T) {
			err := exRequestWithOptions(context.Background(), nil, http.MethodGet, "/roles", exRequestOptions{
				Headers: map[string]string{header: "attempted override"},
			}, nil, nil)
			if err == nil || !strings.Contains(err.Error(), "cannot override transport identity") {
				t.Fatalf("error = %v, want identity-header rejection", err)
			}
		})
	}
}

func TestEXRequestRejectsOversizedSuccessResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`"` + strings.Repeat("x", exMaximumResponseSize) + `"`))
	}))
	defer server.Close()

	err := exRequest(context.Background(), newEXTestClient(t, server.URL), http.MethodGet, "/projects", nil, nil, new(string))
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error = %v, want bounded response error", err)
	}
}

func TestEXRequestReturnsSanitizedTypedErrors(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusConflict, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			const secret = "server-secret-must-not-leak"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(secret))
			}))
			defer server.Close()

			err := exRequest(context.Background(), newEXTestClient(t, server.URL), http.MethodGet, "/projects/{project_id}", map[string]string{"project_id": secret}, nil, nil)
			if err == nil {
				t.Fatal("expected error")
			}
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("error leaked secret: %q", err)
			}
			var apiErr *exAPIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != status {
				t.Fatalf("error = %v, want exAPIError status %d", err, status)
			}
			if exNotFound(err) != (status == http.StatusNotFound) {
				t.Errorf("exNotFound(%v) has wrong result", status)
			}
		})
	}
}

func TestEXRequestPreservesContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := exRequest(ctx, newEXTestClient(t, server.URL), http.MethodGet, "/projects", nil, nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
}

func TestEXRequestSanitizesUnderlyingTransportErrors(t *testing.T) {
	const secret = "resolved-path-secret"
	server := httptest.NewServer(http.NotFoundHandler())
	client := newEXTestClient(t, server.URL)
	server.Close()

	err := exRequest(context.Background(), client, http.MethodGet, "/projects/{project_id}", map[string]string{"project_id": secret}, nil, nil)
	if err == nil {
		t.Fatal("expected transport error")
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), server.URL) {
		t.Fatalf("transport error leaked request detail: %q", err)
	}
}
