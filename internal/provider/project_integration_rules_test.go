package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAcc_ProjectIntegrationRules(t *testing.T) {
	suffix := acctest.RandString(8)
	config := func(expected, variable string) string {
		return testAccProjectIntegrationConfig(suffix, "") + fmt.Sprintf(`
resource "semaphore_ex_project_integration_matcher" "test" {
  project_id = semaphore_ex_project.test.id
  integration_id = semaphore_ex_project_integration.test.id
  name = "branch"
  match_type = "body"
  key = "ref"
  value = %q
}
resource "semaphore_ex_project_integration_extract_value" "test" {
  project_id = semaphore_ex_project.test.id
  integration_id = semaphore_ex_project_integration.test.id
  name = "commit"
  value_source = "body"
  key = "after"
  variable = %q
  variable_type = "environment"
}
data "semaphore_ex_project_integration_matcher" "test" {
  project_id = semaphore_ex_project.test.id
  integration_id = semaphore_ex_project_integration.test.id
  id = semaphore_ex_project_integration_matcher.test.id
}
data "semaphore_ex_project_integration_extract_value" "test" {
  project_id = semaphore_ex_project.test.id
  integration_id = semaphore_ex_project_integration.test.id
  id = semaphore_ex_project_integration_extract_value.test.id
}
`, expected, variable)
	}
	importID := func(name, label string) resource.ImportStateIdFunc {
		return func(state *terraform.State) (string, error) {
			item, ok := state.RootModule().Resources[name]
			if !ok {
				return "", fmt.Errorf("missing resource %s", name)
			}
			return fmt.Sprintf("project/%s/integration/%s/%s/%s", item.Primary.Attributes["project_id"], item.Primary.Attributes["integration_id"], label, item.Primary.ID), nil
		}
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config("refs/heads/main", "GIT_COMMIT"), Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.semaphore_ex_project_integration_matcher.test", "value", "refs/heads/main"),
				resource.TestCheckResourceAttr("data.semaphore_ex_project_integration_extract_value.test", "variable", "GIT_COMMIT"),
			)},
			{Config: config("refs/heads/release", "DEPLOY_COMMIT"), Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.semaphore_ex_project_integration_matcher.test", "value", "refs/heads/release"),
				resource.TestCheckResourceAttr("data.semaphore_ex_project_integration_extract_value.test", "variable", "DEPLOY_COMMIT"),
			)},
			{ResourceName: "semaphore_ex_project_integration_matcher.test", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: importID("semaphore_ex_project_integration_matcher.test", "matcher")},
			{ResourceName: "semaphore_ex_project_integration_extract_value.test", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: importID("semaphore_ex_project_integration_extract_value.test", "value")},
		},
	})
}
