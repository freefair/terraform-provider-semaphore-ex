package provider

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_ProjectSchedule_oneOffAndCronTransition(t *testing.T) {
	suffix := acctest.RandString(8)
	runAt := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339)
	config := func(timing string, deleteAfter bool) string {
		return testAccProjectScheduleDependencyConfig(suffix) + fmt.Sprintf(`
resource "semaphore_ex_project_schedule" "test" {
  project_id = semaphore_ex_project.test.id
  template_id = semaphore_ex_project_template.test.id
  name = "one-off"
  enabled = false
  %s
  delete_after_run = %t
  repository_id = semaphore_ex_project_repository.test.id
  task_params = {
    git_branch = "release"
    ansible = { debug_level = 2, skip_galaxy_install = true }
  }
}
`, timing, deleteAfter)
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config(fmt.Sprintf("run_at = %q", runAt), true), Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("semaphore_ex_project_schedule.test", "run_at", runAt),
				resource.TestCheckResourceAttr("semaphore_ex_project_schedule.test", "type", "run_at"),
				resource.TestCheckResourceAttr("semaphore_ex_project_schedule.test", "task_params.ansible.debug_level", "2"),
			)},
			{ResourceName: "semaphore_ex_project_schedule.test", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: testAccProjectScheduleImportID("semaphore_ex_project_schedule.test")},
			{Config: config(`cron_format = "0 0 * * *"`, false), Check: resource.TestCheckResourceAttr("semaphore_ex_project_schedule.test", "type", "cron")},
		},
	})
}

func TestAcc_ProjectInventory_terragruntPlacement(t *testing.T) {
	suffix := acctest.RandString(8)
	config := func(tag string) string {
		return testAccProjectInventoryConfig(suffix, fmt.Sprintf(`terragrunt_workspace = { workspace = "production" }
runner_tag = %q`, tag))
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config("isolated"), Check: resource.ComposeAggregateTestCheckFunc(testAccProjectInventoryExists("semaphore_ex_project_inventory.test", "terragrunt-workspace"), resource.TestCheckResourceAttr("semaphore_ex_project_inventory.test", "runner_tag", "isolated"))},
			{Config: config(""), Check: resource.TestCheckResourceAttr("semaphore_ex_project_inventory.test", "runner_tag", "")},
		},
	})
}

func TestAcc_ProjectTemplate_exSettings(t *testing.T) {
	suffix := acctest.RandString(8)
	config := testAccProjectTemplateConfig(suffix, `
allow_parallel_tasks = true
allow_override_branch_in_task = true
jwt_params = { enabled = true, audience = ["test-runner"], ttl = "5m" }
survey_vars = [
  { name = "Description", title = "Description", type = "text", target = "env", default_value = "deploy" },
  { name = "Regions", title = "Regions", type = "select", enum_values = {EU="eu",US="us"}, default_values = ["eu"] }
]
task_params = { version = "release-candidate", inventory_id = semaphore_ex_project_inventory.test.id, ansible = { debug_level = 3, skip_galaxy_install = true } }
`)
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config, Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("semaphore_ex_project_template.test", "allow_parallel_tasks", "true"),
				resource.TestCheckResourceAttr("semaphore_ex_project_template.test", "jwt_params.ttl", "5m"),
				resource.TestCheckResourceAttr("semaphore_ex_project_template.test", "survey_vars.1.default_values.0", "eu"),
				resource.TestCheckResourceAttr("semaphore_ex_project_template.test", "task_params.ansible.debug_level", "3"),
			)},
			{ResourceName: "semaphore_ex_project_template.test", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: testAccProjectTemplateImportID("semaphore_ex_project_template.test")},
		},
	})
}
