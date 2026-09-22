package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestEXRequestErrorMessages(t *testing.T) {
	for _, tc := range []struct {
		name    string
		body    string
		message string
	}{
		{"validation", `{"error":"new workflow approval nodes require a role policy"}`, "new workflow approval nodes require a role policy"},
		{"extra fields", `{"error":"revision conflict","token":"server-secret-must-not-leak"}`, "revision conflict"},
		{"whitespace", `{"error":"  invalid policy  "}`, "invalid policy"},
		{"control characters", `{"error":"invalid\npolicy\u001b[31m"}`, "invalid\npolicy\x1b[31m"},
		{"empty body", "", ""},
		{"empty message", `{"error":" \n "}`, ""},
		{"missing message", `{"token":"server-secret-must-not-leak"}`, ""},
		{"null message", `{"error":null}`, ""},
		{"object message", `{"error":{"token":"server-secret-must-not-leak"}}`, ""},
		{"plain text", "server-secret-must-not-leak", ""},
		{"html", "<html>server-secret-must-not-leak</html>", ""},
		{"invalid json", `{"error":"server-secret-must-not-leak"`, ""},
		{"trailing content", `{"error":"server-secret-must-not-leak"} {}`, ""},
		{"size boundary", `{"error":"` + strings.Repeat("x", exMaximumErrorResponseSize-12) + `"}`, strings.Repeat("x", exMaximumErrorResponseSize-12)},
		{"oversized", `{"error":"` + strings.Repeat("x", exMaximumErrorResponseSize-11) + `"}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			err := exRequest(context.Background(), newEXTestClient(t, server.URL), http.MethodPost, "/projects/{project_id}/workflows", map[string]string{"project_id": "resolved-secret"}, nil, nil)
			require.Error(t, err)
			var apiErr *exAPIError
			require.ErrorAs(t, err, &apiErr)
			assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
			expected := "Semaphore EX API request POST /projects/{project_id}/workflows returned status 400"
			if tc.message != "" {
				expected += fmt.Sprintf(": %q", tc.message)
			}
			assert.Equal(t, expected, err.Error())
		})
	}
}

func TestWorkflowCreateReportsApprovalPolicyValidation(t *testing.T) {
	ctx := context.Background()
	const message = "new workflow approval nodes require a role policy"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/project/1/workflows", r.URL.Path)
		var body struct {
			Nodes []map[string]any `json:"nodes"`
		}
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		if assert.Len(t, body.Nodes, 1) {
			assert.Equal(t, "approval", body.Nodes[0]["kind"])
			assert.NotContains(t, body.Nodes[0], "approval_role_policy")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
	}))
	defer server.Close()
	instance := &exWorkflowDefinitionResource{client: newEXTestClient(t, server.URL)}
	var schema resource.SchemaResponse
	instance.Schema(ctx, resource.SchemaRequest{}, &schema)
	typ := schema.Schema.Type().TerraformType(ctx)
	nodeType := typ.(tftypes.Object).AttributeTypes["nodes"].(tftypes.List).ElementType
	raw := unknownConfigObject(typ, map[string]any{
		"project_id": int64(1), "name": "approval validation",
		"nodes": []tftypes.Value{unknownConfigObject(nodeType, map[string]any{"key": "Approval", "kind": "approval"})},
	})
	response := resource.CreateResponse{State: tfsdk.State{Schema: schema.Schema}}
	instance.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: schema.Schema, Raw: raw}}, &response)
	require.Len(t, response.Diagnostics, 1)
	assert.Equal(t, "Error Creating Workflow Definition", response.Diagnostics[0].Summary())
	assert.Contains(t, response.Diagnostics[0].Detail(), "status 400")
	assert.Contains(t, response.Diagnostics[0].Detail(), message)
}
