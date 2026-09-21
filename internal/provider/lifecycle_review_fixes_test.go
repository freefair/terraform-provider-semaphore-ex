package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	providerfw "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func configuredLifecycleClient(t *testing.T, baseURL string, tokens ...string) *apiclient.SemaphoreUI {
	t.Helper()
	ctx := context.Background()
	p := New("test")()
	var s providerfw.SchemaResponse
	p.Schema(ctx, providerfw.SchemaRequest{}, &s)
	token := exTestToken
	if len(tokens) > 0 {
		token = tokens[0]
	}
	raw := unknownConfigObject(s.Schema.Type().TerraformType(ctx), map[string]any{"api_token": token, "api_base_url": baseURL, "tls_skip_verify": false})
	var resp providerfw.ConfigureResponse
	p.Configure(ctx, providerfw.ConfigureRequest{Config: tfsdk.Config{Schema: s.Schema, Raw: raw}}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	return resp.ResourceData.(*apiclient.SemaphoreUI)
}

func TestProjectAccessLossPreservesState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/user" {
			_, _ = w.Write([]byte(`{"id":9,"admin":false}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	r := NewProjectResource()
	var configured resource.ConfigureResponse
	r.(resource.ResourceWithConfigure).Configure(context.Background(), resource.ConfigureRequest{ProviderData: configuredLifecycleClient(t, server.URL+"/api")}, &configured)
	state := lifecycleState(t, r)
	resp := resource.ReadResponse{State: state}
	r.Read(context.Background(), resource.ReadRequest{State: state}, &resp)
	require.True(t, resp.Diagnostics.HasError(), "ambiguous project absence must produce an error")
	assert.Equal(t, state.Raw, resp.State.Raw)
}

func TestImportedTokenAdoptsInitialKeepers(t *testing.T) {
	ctx := context.Background()
	s := RunnerRegistrationTokenSchema().GetResource(ctx)
	state := tfsdk.State{Schema: s}
	require.False(t, state.Set(ctx, RunnerRegistrationTokenModel{ID: types.StringValue("runner/7"), RunnerID: types.Int64Value(7), ProjectID: types.Int64Null(), Keepers: types.MapNull(types.StringType), RegistrationToken: types.StringNull()}).HasError())
	keepers, diags := types.MapValueFrom(ctx, types.StringType, map[string]string{"rotation": "1"})
	require.False(t, diags.HasError())
	req := planmodifier.MapRequest{State: state, Plan: tfsdk.Plan{Schema: s, Raw: state.Raw}, StateValue: types.MapNull(types.StringType), PlanValue: keepers, ConfigValue: keepers}
	resp := planmodifier.MapResponse{}
	for _, modifier := range s.Attributes["keepers"].(schema.MapAttribute).PlanModifiers {
		modifier.PlanModifyMap(ctx, req, &resp)
	}
	require.False(t, resp.Diagnostics.HasError())
	assert.False(t, resp.RequiresReplace, "first imported keepers must establish a baseline without token issuance")
}

func TestProjectAbsenceTransportClassifies404(t *testing.T) {
	for _, test := range []struct {
		name         string
		parentStatus int
		parent, user string
		userStatus   int
		wantMissing  bool
	}{
		{"lost membership", 404, "", `{"id":9,"admin":false}`, 200, false},
		{"deleted project admin", 404, "", `{"id":9,"admin":true}`, 200, true},
		{"deleted child accessible project", 200, `{"id":17}`, `{"id":9,"admin":false}`, 200, true},
		{"wrong project response", 200, `{"id":18}`, `{"id":9,"admin":true}`, 200, false},
		{"parent forbidden", 403, "", `{"id":9,"admin":true}`, 200, false},
		{"parent failure", 500, "", `{"id":9,"admin":true}`, 200, false},
		{"wrong API prefix", 404, "", "", 404, false},
		{"expired token", 404, "", "", 401, false},
		{"unknown privilege", 404, "", `{"id":9}`, 200, false},
		{"malformed user", 404, "", `not-json`, 200, false},
		{"invalid user identity", 404, "", `{"id":0,"admin":true}`, 200, false},
	} {
		for _, constructor := range []func() resource.Resource{NewProjectEnvironmentResource, NewProjectGeneratedSSHKeyResource, NewProjectScheduleResource, NewProjectRunnerResource} {
			r := constructor()
			var meta resource.MetadataResponse
			r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "semaphore_ex"}, &meta)
			t.Run(test.name+"/"+meta.TypeName, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
					assert.Contains(t, q.Header.Get("Authorization"), exTestToken)
					w.Header().Set("Content-Type", "application/json")
					switch q.URL.Path {
					case "/api/project/17":
						assert.Equal(t, http.MethodGet, q.Method)
						w.WriteHeader(test.parentStatus)
						_, _ = w.Write([]byte(test.parent))
					case "/api/user":
						assert.Equal(t, http.MethodGet, q.Method)
						w.WriteHeader(test.userStatus)
						_, _ = w.Write([]byte(test.user))
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
				defer server.Close()
				var configured resource.ConfigureResponse
				r.(resource.ResourceWithConfigure).Configure(context.Background(), resource.ConfigureRequest{ProviderData: configuredLifecycleClient(t, server.URL+"/api/")}, &configured)
				state := lifecycleState(t, r)
				read := resource.ReadResponse{State: state}
				r.Read(context.Background(), resource.ReadRequest{State: state}, &read)
				assert.Equal(t, !test.wantMissing, read.Diagnostics.HasError(), "%v", read.Diagnostics)
				assert.Equal(t, test.wantMissing, read.State.Raw.IsNull())
				deleted := resource.DeleteResponse{State: state}
				r.Delete(context.Background(), resource.DeleteRequest{State: state}, &deleted)
				assert.Equal(t, !test.wantMissing, deleted.Diagnostics.HasError(), "%v", deleted.Diagnostics)
			})
		}
	}
}

func TestKeeperAdoptionDoesNotSuppressLaterRotation(t *testing.T) {
	ctx := context.Background()
	s := RunnerRegistrationTokenSchema().GetResource(ctx)
	for _, test := range []struct {
		name                                    string
		hasCredential, hasBaseline, wantReplace bool
	}{
		{"imported first baseline", false, false, false}, {"imported established baseline", false, true, true}, {"issued token first keeper", true, false, true}, {"issued token keeper change", true, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := types.MapNull(types.StringType)
			if test.hasBaseline {
				var d diag.Diagnostics
				before, d = types.MapValueFrom(ctx, types.StringType, map[string]string{"rotation": "1"})
				require.False(t, d.HasError())
			}
			credential := types.StringNull()
			if test.hasCredential {
				credential = types.StringValue(exTestToken)
			}
			state := tfsdk.State{Schema: s}
			require.False(t, state.Set(ctx, RunnerRegistrationTokenModel{ID: types.StringValue("runner/7"), RunnerID: types.Int64Value(7), ProjectID: types.Int64Null(), Keepers: before, RegistrationToken: credential}).HasError())
			after, d := types.MapValueFrom(ctx, types.StringType, map[string]string{"rotation": "2"})
			require.False(t, d.HasError())
			req := planmodifier.MapRequest{State: state, Plan: tfsdk.Plan{Schema: s, Raw: state.Raw}, StateValue: before, PlanValue: after, ConfigValue: after}
			resp := planmodifier.MapResponse{}
			for _, modifier := range s.Attributes["keepers"].(schema.MapAttribute).PlanModifiers {
				modifier.PlanModifyMap(ctx, req, &resp)
			}
			require.False(t, resp.Diagnostics.HasError())
			assert.Equal(t, test.wantReplace, resp.RequiresReplace)
		})
	}
}

func TestProjectAbsenceProbePreservesCancellation(t *testing.T) {
	reached := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/project/17" {
			close(reached)
			<-r.Context().Done()
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := configuredLifecycleClient(t, server.URL+"/api")
	done := make(chan error, 1)
	go func() {
		done <- exRequest(ctx, client, http.MethodGet, "/project/{project_id}/keys/{key_id}", map[string]string{"project_id": "17", "key_id": "2"}, nil, nil)
	}()
	select {
	case <-reached:
	case <-time.After(5 * time.Second):
		t.Fatal("probe did not start")
	}
	cancel()
	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("probe did not honor cancellation")
	}
}
