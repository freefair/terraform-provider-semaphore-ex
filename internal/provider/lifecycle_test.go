package provider

import (
	"context"
	"fmt"
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

func TestImportIdentityValidation(t *testing.T) {
	for _, id := range []string{"", "junk/project/1/template/2", "project/1/template/2/junk", "project/1/project/3/template/2", "project/0/template/2", "project/-1/template/2", "project/1/template/9223372036854775808", "template/2/project/1", "project/1/template/2/", "project/+1/template/2"} {
		t.Run(id, func(t *testing.T) {
			_, err := parseImportFields(id, []string{"project", "template"})
			require.Error(t, err)
		})
	}
	for _, id := range []string{"42", "project/42"} {
		got, err := parseImportFields(id, []string{"project"})
		require.NoError(t, err)
		assert.Equal(t, int64(42), got["project"])
	}
	fields, err := parseImportFields("project/1/integration/2/alias/3", []string{"project", "alias"}, []string{"project", "integration", "alias"})
	require.NoError(t, err)
	assert.Equal(t, map[string]int64{"project": 1, "integration": 2, "alias": 3}, fields)
}

func lifecycleState(t *testing.T, r resource.Resource) tfsdk.State {
	t.Helper()
	ctx := context.Background()
	var schema resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schema)
	require.False(t, schema.Diagnostics.HasError(), "%v", schema.Diagnostics)
	typ := schema.Schema.Type().(types.ObjectType)
	values := map[string]attr.Value{}
	for name, typ := range typ.AttrTypes {
		value, err := exTypedValue(ctx, typ, nil)
		require.NoError(t, err)
		values[name] = value
		if (name == "id" || name == "project_id" || name == "user_id" || name == "runner_id") && typ.Equal(types.Int64Type) {
			values[name] = types.Int64Value(17)
		}
	}
	value, diags := types.ObjectValue(typ.AttrTypes, values)
	require.False(t, diags.HasError())
	state := tfsdk.State{Schema: schema.Schema}
	require.False(t, state.Set(ctx, value).HasError())
	return state
}

func TestLegacyResourceMissingObjectLifecycle(t *testing.T) {
	constructors := []func() resource.Resource{NewProjectResource, NewProjectInventoryResource, NewProjectRepositoryResource, NewProjectEnvironmentResource, NewProjectTemplateResource, NewProjectViewResource, NewProjectUserResource, NewProjectKeyResource, NewProjectIntegrationResource, NewUserResource, NewIntegrationAliasResource, NewRunnerResource, NewProjectRunnerResource}
	for _, constructor := range constructors {
		r := constructor()
		var metadata resource.MetadataResponse
		r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "semaphore_ex"}, &metadata)
		t.Run(metadata.TypeName, func(t *testing.T) {
			for _, status := range []int{404, 401, 403, 500} {
				t.Run(fmt.Sprint(status), func(t *testing.T) {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(status)
						_, _ = w.Write([]byte(`{}`))
					}))
					defer server.Close()
					var configured resource.ConfigureResponse
					r.(resource.ResourceWithConfigure).Configure(context.Background(), resource.ConfigureRequest{ProviderData: newEXTestClient(t, server.URL)}, &configured)
					require.False(t, configured.Diagnostics.HasError())
					state := lifecycleState(t, r)
					read := resource.ReadResponse{State: state}
					r.Read(context.Background(), resource.ReadRequest{State: state}, &read)
					assert.Equal(t, status != 404, read.Diagnostics.HasError(), "%v", read.Diagnostics)
					assert.Equal(t, status == 404, read.State.Raw.IsNull())
					deleted := resource.DeleteResponse{State: state}
					r.Delete(context.Background(), resource.DeleteRequest{State: state}, &deleted)
					assert.Equal(t, status != 404, deleted.Diagnostics.HasError(), "%v", deleted.Diagnostics)
				})
			}
		})
	}
}

func TestListBackedResourceMissingMember(t *testing.T) {
	for _, constructor := range []func() resource.Resource{NewProjectKeyResource, NewProjectUserResource, NewIntegrationAliasResource} {
		r := constructor()
		t.Run(fmt.Sprintf("%T", r), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[]`))
			}))
			defer server.Close()
			var configured resource.ConfigureResponse
			r.(resource.ResourceWithConfigure).Configure(context.Background(), resource.ConfigureRequest{ProviderData: newEXTestClient(t, server.URL)}, &configured)
			state := lifecycleState(t, r)
			resp := resource.ReadResponse{State: state}
			r.Read(context.Background(), resource.ReadRequest{State: state}, &resp)
			require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
			assert.True(t, resp.State.Raw.IsNull())
		})
	}
}

func TestEveryRegisteredResourceSupportsImport(t *testing.T) {
	for _, constructor := range New("test")().Resources(context.Background()) {
		r := constructor()
		_, ok := r.(resource.ResourceWithImportState)
		assert.True(t, ok, "%T must support import", r)
	}
}
