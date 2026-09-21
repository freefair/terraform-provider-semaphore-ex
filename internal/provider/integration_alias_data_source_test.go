package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	acctestresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationAliasDataSourceUsesExactScope(t *testing.T) {
	for _, scoped := range []bool{false, true} {
		for _, found := range []bool{false, true} {
			t.Run(fmt.Sprintf("scoped=%t/found=%t", scoped, found), func(t *testing.T) {
				route := "/api/project/1/integrations/aliases"
				integrationID := types.Int64Null()
				if scoped {
					route = "/api/project/1/integrations/2/aliases"
					integrationID = types.Int64Value(2)
				}
				requests := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests++
					assert.Equal(t, http.MethodGet, r.Method)
					assert.Equal(t, route, r.URL.Path)
					w.Header().Set("Content-Type", "application/json")
					body := `[{"id":99,"url":"https://example.test/wrong"}]`
					if found {
						body = `[{"id":3,"url":"https://example.test/selected"},{"id":99,"url":"https://example.test/wrong"}]`
					}
					_, _ = w.Write([]byte(body))
				}))
				defer server.Close()
				d := NewIntegrationAliasDataSource()
				var configured datasource.ConfigureResponse
				d.(datasource.DataSourceWithConfigure).Configure(context.Background(), datasource.ConfigureRequest{ProviderData: newEXTestClient(t, server.URL)}, &configured)
				var schema datasource.SchemaResponse
				d.Schema(context.Background(), datasource.SchemaRequest{}, &schema)
				state := tfsdk.State{Schema: schema.Schema}
				require.False(t, state.Set(context.Background(), IntegrationAliasModel{ID: types.Int64Value(3), ProjectID: types.Int64Value(1), IntegrationID: integrationID, URL: types.StringNull()}).HasError())
				response := datasource.ReadResponse{State: tfsdk.State{Schema: schema.Schema}}
				d.Read(context.Background(), datasource.ReadRequest{Config: tfsdk.Config{Schema: schema.Schema, Raw: state.Raw}}, &response)
				assert.Equal(t, 1, requests)
				assert.Equal(t, !found, response.Diagnostics.HasError(), "%v", response.Diagnostics)
				if found {
					var got IntegrationAliasModel
					require.False(t, response.State.Get(context.Background(), &got).HasError())
					assert.Equal(t, "https://example.test/selected", got.URL.ValueString())
				}
			})
		}
	}
}

func TestResourceDataSourceParity(t *testing.T) {
	p := New("test")()
	names := map[string]bool{}
	for _, constructor := range p.DataSources(context.Background()) {
		var m datasource.MetadataResponse
		constructor().Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "semaphore_ex"}, &m)
		names[m.TypeName] = true
	}
	for _, constructor := range p.Resources(context.Background()) {
		var m resource.MetadataResponse
		constructor().Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "semaphore_ex"}, &m)
		// The API only returns registration tokens on a mutating POST. Exposing
		// that as a data source would rotate credentials during plan/refresh.
		if m.TypeName == "semaphore_ex_runner_registration_token" {
			continue
		}
		assert.True(t, names[m.TypeName], "missing data source for %s", m.TypeName)
	}
}

func TestAcc_IntegrationAliasDataSource(t *testing.T) {
	for _, scoped := range []bool{false, true} {
		t.Run(fmt.Sprint(scoped), func(t *testing.T) {
			suffix := acctest.RandString(8)
			scope := ""
			if scoped {
				scope = "integration_id = semaphore_ex_project_integration.test.id"
			}
			config := testAccIntegrationAliasDependencyConfig(suffix) + fmt.Sprintf(`
resource "semaphore_ex_integration_alias" "lookup" {
 project_id = semaphore_ex_project.test.id
 %s
}
data "semaphore_ex_integration_alias" "lookup" {
 id = semaphore_ex_integration_alias.lookup.id
 project_id = semaphore_ex_project.test.id
 %s
}
`, scope, scope)
			acctestresource.Test(t, acctestresource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []acctestresource.TestStep{{Config: config, Check: acctestresource.TestCheckResourceAttrPair("data.semaphore_ex_integration_alias.lookup", "url", "semaphore_ex_integration_alias.lookup", "url")}}})
		})
	}
}
