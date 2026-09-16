package provider

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestTemplateVariableGroupsRoundTrip(t *testing.T) {
	ctx := context.Background()
	for _, ids := range [][]int64{{8, 2}, {2}, {}} {
		t.Run(fmt.Sprint(ids), func(t *testing.T) {
			groups, diags := types.SetValueFrom(ctx, types.Int64Type, ids)
			if diags.HasError() {
				t.Fatal(diags)
			}
			plan := ProjectTemplateModel{Vaults: types.ListNull(ProjectTemplateVaultType), EnvironmentIDs: groups, WorkingDirectory: types.StringValue("ansible"), ExecutorImage: types.StringValue("example.invalid/job:v1"), SuppressErrorAlerts: types.BoolValue(true)}
			req := convertProjectTemplateModelToTemplateRequest(ctx, plan)
			if len(req.EnvironmentIds) != len(ids) {
				t.Fatalf("groups lost: %v", req.EnvironmentIds)
			}
			if req.EnvironmentID != 0 {
				t.Fatalf("plural request sent legacy ID %d", req.EnvironmentID)
			}
			out := convertTemplateResponseToProjectTemplateModel(ctx, &models.Template{EnvironmentIds: req.EnvironmentIds, WorkingDirectory: req.WorkingDirectory, ExecutorImage: req.ExecutorImage, SuppressErrorAlerts: req.SuppressErrorAlerts}, &plan)
			if !out.EnvironmentIDs.Equal(groups) {
				t.Fatalf("groups changed: %v", out.EnvironmentIDs)
			}
			if len(ids) == 0 && !out.EnvironmentID.IsNull() {
				t.Fatal("empty groups acquired legacy value")
			}
			if !out.WorkingDirectory.Equal(plan.WorkingDirectory) || !out.ExecutorImage.Equal(plan.ExecutorImage) || !out.SuppressErrorAlerts.ValueBool() {
				t.Fatal("EX settings were lost")
			}
		})
	}
}

func TestTemplateLegacyGroupDoesNotDiscardAdditionalGroups(t *testing.T) {
	ctx := context.Background()
	groups, _ := types.SetValueFrom(ctx, types.Int64Type, []int64{2, 8})
	req := convertProjectTemplateModelToTemplateRequest(ctx, ProjectTemplateModel{Vaults: types.ListNull(ProjectTemplateVaultType), EnvironmentID: types.Int64Value(2), EnvironmentIDs: groups})
	if !reflect.DeepEqual(req.EnvironmentIds, []int64{2, 8}) {
		t.Fatalf("lost groups: %v", req.EnvironmentIds)
	}
}

func TestAcc_ProjectTemplateResource_multipleVariableGroups(t *testing.T) {
	suffix := acctest.RandString(8)
	config := func(groups, extra string) string {
		base := testAccProjectTemplateConfig(suffix, extra)
		base = strings.Replace(base, "environment_id = semaphore_ex_project_environment.test.id", "environment_ids = "+groups, 1)
		return base + fmt.Sprintf(`
resource "semaphore_ex_project_environment" "second" {
 project_id = semaphore_ex_project.test.id
 name = "second-%s"
}
data "semaphore_ex_project_template" "read" {
 project_id = semaphore_ex_project.test.id
 id = semaphore_ex_project_template.test.id
}
`, suffix)
	}
	extras := `working_directory = "ansible"
suppress_error_alerts = true
runner_tags = ["east", "docker"]
runner_tag_match_mode = "any"`
	both := "[semaphore_ex_project_environment.second.id, semaphore_ex_project_environment.test.id]"
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config(both, extras), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_project_template.test", "environment_ids.#", "2"), resource.TestCheckResourceAttr("data.semaphore_ex_project_template.read", "environment_ids.#", "2"), resource.TestCheckResourceAttr("semaphore_ex_project_template.test", "working_directory", "ansible"))},
			{ResourceName: "semaphore_ex_project_template.test", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: testAccProjectTemplateImportID("semaphore_ex_project_template.test")},
			{Config: config(both, `description = "unrelated update"`), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_project_template.test", "working_directory", "ansible"), resource.TestCheckResourceAttr("semaphore_ex_project_template.test", "suppress_error_alerts", "true"), resource.TestCheckResourceAttr("semaphore_ex_project_template.test", "runner_tags.#", "2"))},
			{Config: config("[semaphore_ex_project_environment.second.id]", extras), Check: resource.TestCheckResourceAttr("semaphore_ex_project_template.test", "environment_ids.#", "1")},
			{Config: config("[]", extras), Check: resource.TestCheckResourceAttr("semaphore_ex_project_template.test", "environment_ids.#", "0")},
		},
	})
}

func TestAcc_ProjectScheduleResource_timezone(t *testing.T) {
	suffix := acctest.RandString(8)
	base := testAccProjectScheduleConfig(suffix, false)
	withZone := strings.Replace(base, `cron_format = "0 0 * * *"`, `cron_format = "0 0 * * *"`+"\n timezone = \"Europe/Berlin\"", 1)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: withZone, Check: resource.TestCheckResourceAttr("semaphore_ex_project_schedule.test", "timezone", "Europe/Berlin")},
		{ResourceName: "semaphore_ex_project_schedule.test", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: testAccProjectScheduleImportID("semaphore_ex_project_schedule.test")},
		{Config: strings.Replace(base, `cron_format = "0 0 * * *"`, `cron_format = "30 1 * * *"`, 1), Check: resource.TestCheckResourceAttr("semaphore_ex_project_schedule.test", "timezone", "Europe/Berlin")},
	}})
}

func TestAcc_ProjectTemplateResource_changeLegacyVariableGroup(t *testing.T) {
	suffix := acctest.RandString(8)
	initial := testAccProjectTemplateConfig(suffix, "") + fmt.Sprintf(`
resource "semaphore_ex_project_environment" "second" {
 project_id = semaphore_ex_project.test.id
 name = "second-%s"
}
`, suffix)
	updated := strings.Replace(initial, "environment_id = semaphore_ex_project_environment.test.id", "environment_id = semaphore_ex_project_environment.second.id", 1)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: initial},
		{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttrPair("semaphore_ex_project_template.test", "environment_id", "semaphore_ex_project_environment.second", "id"), resource.TestCheckResourceAttr("semaphore_ex_project_template.test", "environment_ids.#", "1"))},
	}})
}

func TestTemplateRunnerTagsRequireCanonicalInput(t *testing.T) {
	for _, tt := range []struct {
		value   string
		invalid bool
	}{{"docker", false}, {"Docker", true}, {" docker", true}, {"", true}} {
		t.Run(tt.value, func(t *testing.T) {
			response := validator.StringResponse{}
			canonicalRunnerTagValidator{}.ValidateString(context.Background(), validator.StringRequest{Path: path.Root("runner_tags"), ConfigValue: types.StringValue(tt.value)}, &response)
			if response.Diagnostics.HasError() != tt.invalid {
				t.Fatalf("unexpected validation for %q: %v", tt.value, response.Diagnostics)
			}
		})
	}
}
