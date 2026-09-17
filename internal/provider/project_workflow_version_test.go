package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAcc_ProjectWorkflowVersionAndRestore(t *testing.T) {
	suffix := acctest.RandString(8)
	config := testAccWorkflowDefinitionConfig(suffix, "Versioned "+suffix) + `
data "semaphore_ex_project_workflow_version" "test" {
  project_id = semaphore_ex_project.test.id
  workflow_id = semaphore_ex_workflow_definition.test.id
  version_number = 1
}
data "semaphore_ex_project_workflow_version" "latest" {
  project_id = semaphore_ex_project.test.id
  workflow_id = semaphore_ex_workflow_definition.test.id
}
`
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("data.semaphore_ex_project_workflow_version.test", "version_number", "1"),
			resource.TestCheckResourceAttr("data.semaphore_ex_project_workflow_version.latest", "version_number", "1"),
			resource.TestCheckResourceAttr("data.semaphore_ex_project_workflow_version.test", "definition.nodes.#", "2"),
			resource.TestCheckResourceAttrSet("data.semaphore_ex_project_workflow_version.test", "content_fingerprint"),
			func(state *terraform.State) error {
				workflow := state.RootModule().Resources["semaphore_ex_workflow_definition.test"]
				projectID, err := strconv.ParseInt(workflow.Primary.Attributes["project_id"], 10, 64)
				if err != nil {
					return err
				}
				workflowID, err := strconv.ParseInt(workflow.Primary.ID, 10, 64)
				if err != nil {
					return err
				}
				a := &projectWorkflowRestoreAction{client: testClient()}
				var resp action.InvokeResponse
				a.Invoke(context.Background(), action.InvokeRequest{Config: runtimeActionConfig(t, a, map[string]tftypes.Value{
					"project_id": tftypes.NewValue(tftypes.Number, projectID), "workflow_id": tftypes.NewValue(tftypes.Number, workflowID), "version_number": tftypes.NewValue(tftypes.Number, int64(1)), "message": tftypes.NewValue(tftypes.String, "restore acceptance snapshot"),
				})}, &resp)
				if resp.Diagnostics.HasError() {
					return fmt.Errorf("restore: %v", resp.Diagnostics)
				}
				latest, err := readWorkflowVersion(context.Background(), testClient(), projectID, workflowID, 0)
				if err != nil {
					return err
				}
				if latest.VersionNumber != 2 || latest.RestoredFromVersionID == nil {
					return fmt.Errorf("restore did not create a new version with provenance")
				}
				var runs []any
				if err := exRequest(context.Background(), testClient(), http.MethodGet, "/project/{project_id}/workflows/{workflow_id}/runs", map[string]string{"project_id": strconv.FormatInt(projectID, 10), "workflow_id": strconv.FormatInt(workflowID, 10)}, nil, &runs); err != nil {
					return err
				}
				if len(runs) != 0 {
					return fmt.Errorf("restore unexpectedly started a workflow run")
				}
				return nil
			},
		)},
		{Config: config, Check: resource.TestCheckResourceAttr("data.semaphore_ex_project_workflow_version.latest", "version_number", "2")},
	}})
}
