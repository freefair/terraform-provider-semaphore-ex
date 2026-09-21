package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestCoverageTemplateReadsFlatSettings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":2,"project_id":1,"name":"existing","app":"ansible","playbook":"run.yml","repository_id":1,"inventory_id":1,"environment_ids":[],"ssh_keys":null,"task_params":{"allow_override_limit":true,"hide_diff":true,"limit":["web"],"galaxy_role_args":["--force"]}}`))
	}))
	defer server.Close()
	ctx := context.Background()
	d := &projectTemplateDataSource{client: newEXTestClient(t, server.URL)}
	var schema datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schema)
	_, present := schema.Schema.Attributes["ansible_settings"]
	require.True(t, present, "template-specific Ansible settings must be exposed")
	config := tfsdk.Config{Schema: schema.Schema, Raw: unknownConfigObject(schema.Schema.Type().TerraformType(ctx), map[string]any{"id": int64(2), "project_id": int64(1)})}
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schema.Schema}}
	d.Read(ctx, datasource.ReadRequest{Config: config}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	var enabled, hidden types.Bool
	var limit types.List
	require.False(t, resp.State.GetAttribute(ctx, path.Root("ansible_settings").AtName("allow_override_limit"), &enabled).HasError())
	require.False(t, resp.State.GetAttribute(ctx, path.Root("ansible_settings").AtName("hide_diff"), &hidden).HasError())
	require.False(t, resp.State.GetAttribute(ctx, path.Root("ansible_settings").AtName("limit"), &limit).HasError())
	assert.True(t, enabled.ValueBool())
	assert.True(t, hidden.ValueBool())
	assert.Equal(t, []types.String{types.StringValue("web")}, []types.String{limit.Elements()[0].(types.String)})
}

func TestTemplateUpdatePreservesUnmanagedSettings(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"task_params":{"allow_override_limit":true,"hide_diff":true,"limit":["old"],"future_setting":{"keep":true},"auto_approve":false}}`))
	}))
	defer server.Close()
	schema := ProjectTemplateSchema().GetResource(ctx)
	config := tfsdk.Config{Schema: schema, Raw: unknownConfigObject(schema.Type().TerraformType(ctx), map[string]any{"ansible_settings": map[string]any{"allow_override_limit": false, "limit": []tftypes.Value{}}})}
	body := map[string]any{}
	err := mergeTemplateUpdateSettings(ctx, newEXTestClient(t, server.URL), ProjectTemplateModel{ID: types.Int64Value(2), ProjectID: types.Int64Value(1)}, config, body)
	require.NoError(t, err)
	parameters := body["task_params"].(map[string]any)
	assert.Equal(t, false, parameters["allow_override_limit"])
	assert.Equal(t, true, parameters["hide_diff"])
	assert.Empty(t, parameters["limit"])
	assert.Equal(t, map[string]any{"keep": true}, parameters["future_setting"])
	assert.Equal(t, false, parameters["auto_approve"])
}

func TestTemplateLegacyDefaultsRemainInert(t *testing.T) {
	ctx := context.Background()
	legacy := &TaskParamsModel{Terraform: &TerraformTaskParamsModel{AutoApprove: types.BoolValue(true)}}
	settings := map[string]any{}
	require.NoError(t, addLegacyTemplateMetadata(ctx, settings, legacy))
	assert.NotContains(t, settings, "auto_approve")
	assert.Contains(t, settings, "params")
	model := ProjectTemplateModel{AnsibleSettings: types.ObjectNull(templateSettingTypes("ansible")), TerraformSettings: types.ObjectNull(templateSettingTypes("terraform"))}
	require.NoError(t, readTemplateApplicationSettings(ctx, map[string]any{"task_params": settings}, &model))
	assert.True(t, model.TerraformSettings.IsNull())
	require.NotNil(t, model.TaskParams.Terraform)
	assert.True(t, model.TaskParams.Terraform.AutoApprove.ValueBool())
}

func TestAcc_ProjectTemplateResource_effectiveSettings(t *testing.T) {
	suffix := acctest.RandString(8)
	const address = "semaphore_ex_project_template.test"
	first := testAccProjectTemplateConfig(suffix, `ansible_settings = {
 allow_override_limit = true
 allow_debug = true
 allow_override_inventory = true
 allow_override_tags = true
 allow_override_skip_tags = true
 allow_override_skip_galaxy_install = true
 skip_galaxy_install = true
 hide_dry_run = true
 hide_diff = true
 limit = ["web"]
 tags = ["deploy"]
 skip_tags = ["slow"]
 galaxy_role_args = ["--force"]
 galaxy_collection_args = ["--pre"]
 }`)
	cleared := testAccProjectTemplateConfig(suffix, `ansible_settings = {
 allow_override_limit = false
 hide_diff = false
 limit = []
 tags = []
 skip_tags = []
 galaxy_role_args = []
 galaxy_collection_args = []
 }`)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: first, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr(address, "ansible_settings.allow_override_limit", "true"), resource.TestCheckResourceAttr(address, "ansible_settings.limit.0", "web"), checkEffectiveTemplateSettings(address, true, []string{"web"}))},
		{ResourceName: address, ImportState: true, ImportStateVerify: true, ImportStateIdFunc: testAccProjectTemplateImportID(address)},
		{Config: first, PlanOnly: true, ExpectNonEmptyPlan: false},
		{Config: cleared, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr(address, "ansible_settings.allow_override_limit", "false"), resource.TestCheckResourceAttr(address, "ansible_settings.hide_diff", "false"), resource.TestCheckResourceAttr(address, "ansible_settings.limit.#", "0"), resource.TestCheckResourceAttr(address, "ansible_settings.allow_debug", "true"), checkEffectiveTemplateSettings(address, false, []string{}))},
		{Config: cleared, PlanOnly: true, ExpectNonEmptyPlan: false},
	}})
}

func checkEffectiveTemplateSettings(address string, allow bool, limit []string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		values := state.RootModule().Resources[address].Primary.Attributes
		var wire map[string]any
		err := exRequest(context.Background(), testClient(), http.MethodGet, "/project/{project_id}/templates/{template_id}", map[string]string{"project_id": values["project_id"], "template_id": values["id"]}, nil, &wire)
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(wire["task_params"])
		if err != nil {
			return err
		}
		// Decode exactly the flat fields consumed by db.AnsibleTemplateParams.
		var effective struct {
			AllowOverrideLimit bool     `json:"allow_override_limit"`
			Limit              []string `json:"limit"`
		}
		if err := json.Unmarshal(encoded, &effective); err != nil {
			return err
		}
		if effective.AllowOverrideLimit != allow || !slices.Equal(effective.Limit, limit) {
			return fmt.Errorf("backend effective settings differ: %s", encoded)
		}
		return nil
	}
}

func TestAcc_ProjectTemplateResource_terraformSettings(t *testing.T) {
	suffix := acctest.RandString(8)
	const address = "semaphore_ex_project_template.test"
	first := testAccProjectTemplateConfig(suffix, `app = "terraform"
 terraform_settings = {
 allow_destroy = true
 allow_auto_approve = true
 auto_approve = true
 override_backend = true
 backend_filename = "custom_backend.tf"
 }`)
	cleared := testAccProjectTemplateConfig(suffix, `app = "terraform"
 terraform_settings = {
 allow_destroy = false
 allow_auto_approve = false
 auto_approve = false
 override_backend = false
 backend_filename = ""
 }`)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: first, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr(address, "terraform_settings.auto_approve", "true"), resource.TestCheckResourceAttr(address, "terraform_settings.backend_filename", "custom_backend.tf"))},
		{ResourceName: address, ImportState: true, ImportStateVerify: true, ImportStateIdFunc: testAccProjectTemplateImportID(address)},
		{Config: first, PlanOnly: true, ExpectNonEmptyPlan: false},
		{Config: cleared, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr(address, "terraform_settings.auto_approve", "false"), resource.TestCheckResourceAttr(address, "terraform_settings.allow_destroy", "false"), resource.TestCheckResourceAttr(address, "terraform_settings.backend_filename", ""))},
		{Config: cleared, PlanOnly: true, ExpectNonEmptyPlan: false},
	}})
}
