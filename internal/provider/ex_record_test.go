package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEXRecordRejectsResponseFromDifferentParent(t *testing.T) {
	ctx := context.Background()
	state, diagnostics := types.ObjectValue(map[string]attr.Type{"id": types.Int64Type, "integration_id": types.Int64Type}, map[string]attr.Value{"id": types.Int64Value(5), "integration_id": types.Int64Value(8)})
	require.False(t, diagnostics.HasError())
	spec := projectIntegrationMatcherSpec()
	_, err := spec.state(ctx, state, map[string]any{"id": json.Number("5"), "integration_id": json.Number("9")})
	require.ErrorContains(t, err, "different parent identity")
}

func TestEXRecordCreateRetainsIdentityAfterResponseFailure(t *testing.T) {
	for _, test := range []struct {
		name     string
		spec     exRecordSpec
		id       attr.Value
		response string
	}{
		{name: "invalid response field", spec: projectIntegrationMatcherSpec(), id: types.Int64Unknown(), response: `{"id":4,"name":123}`},
		{name: "empty app response and failed read", spec: appSpec(), id: types.StringValue("test_app"), response: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			schema := test.spec.schema.GetResource(ctx)
			values := map[string]attr.Value{}
			attributes := map[string]attr.Type{}
			for name, attribute := range schema.Attributes {
				attributes[name] = attribute.GetType()
				value, err := exTypedValue(ctx, attribute.GetType(), nil)
				require.NoError(t, err)
				values[name] = value
			}
			values["id"] = test.id
			if _, present := values["project_id"]; present {
				values["project_id"] = types.Int64Value(1)
				values["integration_id"] = types.Int64Value(2)
				values["name"] = types.StringValue("matcher")
			}
			object, diagnostics := types.ObjectValue(attributes, values)
			require.False(t, diagnostics.HasError())
			plan := tfsdk.Plan{Schema: schema}
			require.False(t, plan.Set(ctx, object).HasError())
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					w.Header().Set("Content-Type", "application/json")
					_, err := w.Write([]byte(test.response))
					assert.NoError(t, err)
					return
				}
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
			defer server.Close()
			r := exRecordResource{spec: test.spec, client: newEXTestClient(t, server.URL)}
			resp := resource.CreateResponse{State: tfsdk.State{Schema: schema}}
			r.Create(ctx, resource.CreateRequest{Plan: plan}, &resp)
			require.True(t, resp.Diagnostics.HasError())
			var retained types.Object
			require.False(t, resp.State.Get(ctx, &retained).HasError())
			if test.id.IsUnknown() {
				assert.Equal(t, types.Int64Value(4), retained.Attributes()["id"])
			} else {
				assert.Equal(t, test.id, retained.Attributes()["id"])
			}
		})
	}
}

func TestEXRecordOnlySendsDeclaredWritableFields(t *testing.T) {
	ctx := context.Background()
	state, diagnostics := types.ObjectValue(map[string]attr.Type{"id": types.Int64Type, "name": types.StringType, "revision": types.StringType}, map[string]attr.Value{"id": types.Int64Value(5), "name": types.StringValue("rule"), "revision": types.StringUnknown()})
	require.False(t, diagnostics.HasError())
	spec := exRecordSpec{bodyFields: []string{"name"}}
	body, err := spec.body(ctx, state)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"name": "rule"}, body)
}
