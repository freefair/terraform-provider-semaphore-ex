package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProviderHasEnvironmentCollection(t *testing.T) {
	p := &SemaphoreUIProvider{}
	for _, factory := range p.DataSources(context.Background()) {
		var m datasource.MetadataResponse
		factory().Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "semaphore_ex"}, &m)
		if m.TypeName == "semaphore_ex_project_environments" {
			return
		}
	}
	require.Fail(t, "project_environments collection is missing")
}

func TestCollectionFiltersOrdersAndExcludesSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/project/1/keys", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":10,"name":"shared","type":"ssh","secret":"synthetic-must-not-enter-state"},{"id":2,"name":"shared","type":"ssh"},{"id":3,"name":"other","type":"none"}]`))
	}))
	defer server.Close()
	ctx := context.Background()
	d := &collectionDataSource{spec: lookupSpecs()["project_key"], client: newEXTestClient(t, server.URL)}
	var schema datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schema)
	response := datasource.ReadResponse{State: tfsdk.State{Schema: schema.Schema}}
	d.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schema.Schema, Raw: unknownConfigObject(schema.Schema.Type().TerraformType(ctx), map[string]any{"project_id": int64(1), "name_filter": "shared", "type_filter": "ssh"})}}, &response)
	require.False(t, response.Diagnostics.HasError(), "%v", response.Diagnostics)
	var ids types.List
	require.False(t, response.State.GetAttribute(ctx, path.Root("ids"), &ids).HasError())
	assert.Equal(t, []attr.Value{types.Int64Value(2), types.Int64Value(10)}, ids.Elements())
	assert.NotContains(t, response.State.Raw.String(), "synthetic-must-not-enter-state")
	var revision types.Int64
	require.False(t, response.State.GetAttribute(ctx, path.Root("items").AtListIndex(0).AtName("revision"), &revision).HasError())
	assert.True(t, revision.IsNull())
}

func TestAcc_Collections(t *testing.T) {
	suffix := acctest.RandString(8)
	config := testAccProjectTemplateConfig(suffix, "")
	for _, name := range []string{"project_environments", "project_templates", "project_inventories", "project_repositories", "project_keys", "project_integrations", "project_schedules", "project_runners", "project_views", "project_secret_storages", "project_workflows", "project_roles", "project_notification_destinations"} {
		config += fmt.Sprintf("\ndata \"semaphore_ex_%s\" \"all\" {\n project_id = semaphore_ex_project.test.id\n depends_on = [semaphore_ex_project_template.test]\n}\n", name)
	}
	for _, name := range []string{"global_roles", "global_credentials", "global_notification_destinations", "runners"} {
		config += fmt.Sprintf("\ndata \"semaphore_ex_%s\" \"all\" {}\n", name)
	}
	config += fmt.Sprintf("\ndata \"semaphore_ex_project_environments\" \"filtered\" {\n project_id = semaphore_ex_project.test.id\n name_filter = %q\n depends_on = [semaphore_ex_project_template.test]\n}\n", "Env-"+suffix)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: config, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("data.semaphore_ex_project_environments.filtered", "ids.#", "1"), resource.TestCheckResourceAttrPair("data.semaphore_ex_project_environments.filtered", "ids.0", "semaphore_ex_project_environment.test", "id"), resource.TestCheckResourceAttr("data.semaphore_ex_project_templates.all", "items.#", "1"))}}})
}
