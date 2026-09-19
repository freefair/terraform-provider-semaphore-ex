package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"
)

func TestProjectSSHKeyPolicyDeleteMissingProject(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/api/project/7", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	ctx := context.Background()
	state := tfsdk.State{Schema: sshKeyPolicyResourceSchema()}
	require.False(t, state.Set(ctx, projectSSHKeyPolicyModel{ProjectID: types.Int64Value(7), DefaultSSHKeys: types.ListNull(sshKeyBindingType), AlwaysSSHKeys: types.ListNull(sshKeyBindingType)}).HasError())
	instance := &projectSSHKeyPolicyResource{client: newEXTestClient(t, server.URL)}
	var response resource.DeleteResponse
	instance.Delete(ctx, resource.DeleteRequest{State: state}, &response)
	require.False(t, response.Diagnostics.HasError(), "%v", response.Diagnostics)
	require.Equal(t, 1, calls)
}

func TestTaskSSHOverrideDistinguishesOmittedAndEmpty(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		name := "inherit"
		if explicit {
			name = "explicit-empty"
		}
		t.Run(name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, "/api/project/7/tasks", r.URL.Path)
				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				keys, present := body["ssh_keys"]
				require.Equal(t, explicit, present)
				if explicit {
					require.Equal(t, []any{}, keys)
				}
				_, err := w.Write([]byte(`{"id":11}`))
				require.NoError(t, err)
			}))
			defer server.Close()
			config := map[string]tftypes.Value{
				"project_id":  tftypes.NewValue(tftypes.Number, int64(7)),
				"template_id": tftypes.NewValue(tftypes.Number, int64(3)),
			}
			if explicit {
				config["ssh_keys"] = tftypes.NewValue(tftypes.List{ElementType: sshKeyBindingType.TerraformType(context.Background())}, []tftypes.Value{})
			}
			instance := NewProjectTaskStartAction().(*exRuntimeAction)
			instance.client = newEXTestClient(t, server.URL)
			var response action.InvokeResponse
			instance.Invoke(context.Background(), action.InvokeRequest{Config: runtimeActionConfig(t, instance, config)}, &response)
			require.False(t, response.Diagnostics.HasError(), "%v", response.Diagnostics)
			require.Equal(t, 1, calls)
		})
	}
}
