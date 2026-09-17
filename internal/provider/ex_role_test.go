package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func exRoleTestClient(t *testing.T, serverURL string) *apiclient.SemaphoreUI {
	t.Helper()
	u, err := url.Parse(serverURL)
	if err != nil {
		t.Fatal(err)
	}
	return apiclient.New(httptransport.New(u.Host, "/api", []string{u.Scheme}), strfmt.Default)
}

func TestEXRoleCreateMapsNamedPermissionsFromCatalog(t *testing.T) {
	var created exRoleResponse
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/roles/permissions":
			_, _ = w.Write([]byte(`[{"id":"global_audit","permission":64}]`))
		case "/api/roles/project-permissions":
			_, _ = w.Write([]byte(`[{"id":"project_view","permission":16}]`))
		case "/api/roles":
			if err := json.NewDecoder(r.Body).Decode(&created); err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte(`{"id":"role_opaque","slug":"role_opaque","name":"Auditor","permissions":16,"global_permissions":64,"revision":1}`))
		default:
			t.Fatalf("unexpected request %s", r.URL.Path)
		}
	}))
	defer server.Close()
	projectPermissions, _ := types.SetValueFrom(context.Background(), types.StringType, []string{"project_view"})
	globalPermissions, _ := types.SetValueFrom(context.Background(), types.StringType, []string{"global_audit"})
	state, err := exCreateRole(context.Background(), exRoleTestClient(t, server.URL), false, exRoleModel{Name: types.StringValue("Auditor"), ProjectPermissions: projectPermissions, GlobalPermissions: globalPermissions})
	if err != nil {
		t.Fatal(err)
	}
	if created.Permissions != 16 || created.GlobalPermissions != 64 {
		t.Fatalf("payload masks = %d, %d", created.Permissions, created.GlobalPermissions)
	}
	if state.ID.ValueString() != "role_opaque" || state.Revision.ValueInt64() != 1 {
		t.Fatalf("state = %#v", state)
	}
}

func TestEXRoleRejectsCatalogVersionDriftAndConflicts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/roles/permissions" {
			_, _ = w.Write([]byte(`[{"id":"known","permission":1}]`))
			return
		}
		if r.URL.Path == "/api/roles/project-permissions" {
			_, _ = w.Write([]byte(`[{"id":"known","permission":1}]`))
			return
		}
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusConflict)
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()
	client := exRoleTestClient(t, server.URL)
	permissions, _ := types.SetValueFrom(context.Background(), types.StringType, []string{"known"})
	model := exRoleModel{ID: types.StringValue("role_opaque"), Slug: types.StringValue("role_opaque"), Name: types.StringValue("Role"), ProjectPermissions: permissions, GlobalPermissions: permissions, Revision: types.Int64Value(1)}
	if _, err := exRoleSet(context.Background(), 2, map[string]int64{"known": 1}); err == nil {
		t.Fatal("expected unknown server bit error")
	}
	_, err := exPutRole(context.Background(), client, false, model)
	var conflict *exAPIError
	if !errors.As(err, &conflict) || conflict.StatusCode != http.StatusConflict {
		t.Fatalf("error = %v, want revision conflict", err)
	}
}
