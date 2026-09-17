package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func testAccWorkflowDefinitionConfig(suffix, name string) string {
	return testAccProjectTemplateConfig(suffix, "") + fmt.Sprintf(`
resource "semaphore_ex_workflow_definition" "test" {
  project_id = semaphore_ex_project.test.id
  name       = %q
  parameters = [{ name = "release", type = "string", default_string = "v1" }]
  nodes = [
    { key = "Build", template_id = semaphore_ex_project_template.test.id, display_name = "Build", task_params = { git_branch = "main", ansible = { debug_level = 2 } } },
    { key = "Deploy", template_id = semaphore_ex_project_template.test.id, display_name = "Deploy" },
  ]
  edges = [{ source_key = "Build", destination_key = "Deploy", condition = "on_success" }]
}
data "semaphore_ex_workflow_definition" "test" {
  project_id = semaphore_ex_project.test.id
  id         = semaphore_ex_workflow_definition.test.id
}
`, name)
}

func TestAcc_EXWorkflowDefinition(t *testing.T) {
	suffix := acctest.RandString(8)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: testAccWorkflowDefinitionConfig(suffix, "Workflow "+suffix), Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttrSet("semaphore_ex_workflow_definition.test", "id"),
			resource.TestCheckResourceAttrSet("semaphore_ex_workflow_definition.test", "revision"),
			resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.test", "nodes.#", "2"),
			resource.TestCheckResourceAttr("data.semaphore_ex_workflow_definition.test", "name", "Workflow "+suffix),
		)},
		{ResourceName: "semaphore_ex_workflow_definition.test", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: func(s *terraform.State) (string, error) {
			r := s.RootModule().Resources["semaphore_ex_workflow_definition.test"]
			return fmt.Sprintf("project/%s/workflow/%s", r.Primary.Attributes["project_id"], r.Primary.ID), nil
		}},
		{Config: testAccWorkflowDefinitionConfig(suffix, "Workflow "+suffix+" updated"), Check: resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.test", "name", "Workflow "+suffix+" updated")},
	}})
}

func testAccWorkflowDefinitionRichConfig(suffix string, reordered, cleared bool) string {
	base := testAccProjectTemplateConfig(suffix, "") + `
resource "semaphore_ex_project_role" "reviewer" {
  project_id = semaphore_ex_project.test.id
  name = "workflow-reviewer"
  project_permissions = ["workflow.view", "workflow.start"]
}
`
	if cleared {
		return base + `
resource "semaphore_ex_workflow_definition" "rich" {
  project_id = semaphore_ex_project.test.id
  name = "rich workflow"
  nodes = [{ key = "Producer", template_id = semaphore_ex_project_template.test.id, display_name = "Producer" }]
  edges = []
  parameters = []
}`
	}
	nodes := `
    { key = "Consumer", template_id = semaphore_ex_project_template.test.id, display_name = "Consumer", artifact_inputs = [{ name = "report", source_key = "Producer", output = "report", required = true }] },
    { key = "Producer", kind = "task", template_id = semaphore_ex_project_template.test.id, display_name = "Producer", artifact_outputs = [{ name = "report", sensitive = false, max_bytes = 1024, schema = { type = "object", properties = [{ name = "passed", schema = { type = "boolean" } }] } }] },
    { key = "Approval", kind = "approval", display_name = "Approval", approval_role_policy = { mode = "any_of", role_ids = ["role:${semaphore_ex_project_role.reviewer.id}"], minimum_distinct_approvers = 1, initiator_separation = true } },`
	if reordered {
		nodes = `
    { key = "Producer", kind = "task", template_id = semaphore_ex_project_template.test.id, display_name = "Producer", artifact_outputs = [{ name = "report", sensitive = false, max_bytes = 1024, schema = { type = "object", properties = [{ name = "passed", schema = { type = "boolean" } }] } }] },
    { key = "Consumer", template_id = semaphore_ex_project_template.test.id, display_name = "Consumer", artifact_inputs = [{ name = "report", source_key = "Producer", output = "report", required = true }] },
    { key = "Approval", kind = "approval", display_name = "Approval", approval_role_policy = { mode = "any_of", role_ids = ["role:${semaphore_ex_project_role.reviewer.id}"], minimum_distinct_approvers = 1, initiator_separation = true } },`
	}
	edges := `
    { source_key = "Producer", destination_key = "Consumer", condition = "on_success" },
    { source_key = "Consumer", destination_key = "Approval", condition = "on_success" },`
	if reordered {
		nodes += `
    { key = "AddedConsumer", template_id = semaphore_ex_project_template.test.id, display_name = "AddedConsumer", artifact_inputs = [{ name = "added_report", source_key = "AddedProducer", output = "added_report", required = true }] },
    { key = "AddedProducer", template_id = semaphore_ex_project_template.test.id, display_name = "AddedProducer", artifact_outputs = [{ name = "added_report", sensitive = false, max_bytes = 1024, schema = { type = "string" } }] },`
		edges += `
    { source_key = "Consumer", destination_key = "AddedProducer", condition = "on_success" },
    { source_key = "AddedProducer", destination_key = "AddedConsumer", condition = "on_success" },`
	}
	return base + fmt.Sprintf(`
resource "semaphore_ex_workflow_definition" "rich" {
  project_id = semaphore_ex_project.test.id
  name = "rich workflow"
  parameters = [{ name = "confirmed", type = "boolean", default_bool = false }]
  access_policy = { view_role_ids = ["role:${semaphore_ex_project_role.reviewer.id}"], start_role_ids = ["role:${semaphore_ex_project_role.reviewer.id}"] }
  nodes = [%s]
  edges = [%s]
}
`, nodes, edges)
}

func TestAcc_EXWorkflowDefinitionRichGraph(t *testing.T) {
	suffix := acctest.RandString(8)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: testAccWorkflowDefinitionRichConfig(suffix, false, false), Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.rich", "nodes.#", "3"),
			resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.rich", "edges.#", "2"),
			resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.rich", "parameters.#", "1"),
			resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.rich", "access_policy.view_role_ids.#", "1"),
			resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.rich", "nodes.0.artifact_inputs.#", "1"),
		)},
		{ResourceName: "semaphore_ex_workflow_definition.rich", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: func(s *terraform.State) (string, error) {
			r := s.RootModule().Resources["semaphore_ex_workflow_definition.rich"]
			return fmt.Sprintf("project/%s/workflow/%s", r.Primary.Attributes["project_id"], r.Primary.ID), nil
		}},
		{Config: testAccWorkflowDefinitionRichConfig(suffix, true, false), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.rich", "nodes.#", "5"), resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.rich", "edges.#", "4"))},
		{Config: testAccWorkflowDefinitionRichConfig(suffix, false, true), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.rich", "nodes.#", "1"), resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.rich", "edges.#", "0"), resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.rich", "parameters.#", "0"))},
	}})
}
