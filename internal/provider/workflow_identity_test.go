package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestWorkflowImportAllowsDuplicateAndEmptyLabels(t *testing.T) {
	model, err := workflowDecodedResource(context.Background(), map[string]any{"nodes": []any{map[string]any{"id": int64(2), "display_name": "Same"}, map[string]any{"id": int64(3), "display_name": "Same"}, map[string]any{"id": int64(4), "display_name": ""}}})
	require.NoError(t, err)
	nodes := model.Nodes.Elements()
	assert.Equal(t, types.StringValue("node-2"), nodes[0].(types.Object).Attributes()["key"])
	assert.Equal(t, types.StringValue("node-3"), nodes[1].(types.Object).Attributes()["key"])
	assert.Equal(t, types.StringValue("node-4"), nodes[2].(types.Object).Attributes()["key"])
}
func TestWorkflowReadKeepsIdentityAfterLabelChange(t *testing.T) {
	ctx := context.Background()
	previous, err := workflowDecodedResource(ctx, map[string]any{"nodes": []any{map[string]any{"id": int64(2), "display_name": "Build"}}})
	require.NoError(t, err)
	actual, err := workflowDecodedResource(ctx, map[string]any{"nodes": []any{map[string]any{"id": int64(2), "display_name": "Renamed"}}}, &previous)
	require.NoError(t, err)
	node := actual.Nodes.Elements()[0].(types.Object)
	assert.Equal(t, types.StringValue("Build"), node.Attributes()["key"])
	assert.Equal(t, types.StringValue("Renamed"), node.Attributes()["display_name"])
}

func TestAcc_WorkflowIndependentLabels(t *testing.T) {
	suffix := acctest.RandString(8)
	const address = "semaphore_ex_workflow_definition.test"
	first := testAccProjectTemplateConfig(suffix, "") + `
 resource "semaphore_ex_workflow_definition" "test" {
 project_id = semaphore_ex_project.test.id
 name = "independent labels"
 nodes = [
 { key = "Build", display_name = "Same", template_id = semaphore_ex_project_template.test.id },
 { key = "Deploy", display_name = "Same", template_id = semaphore_ex_project_template.test.id },
 ]
 edges = [{source_key="Build",destination_key="Deploy",condition="on_success"}]
 }`
	changed := testAccProjectTemplateConfig(suffix, "") + `
 resource "semaphore_ex_workflow_definition" "test" {
 project_id = semaphore_ex_project.test.id
 name = "independent labels"
 nodes = [
 { key = "Deploy", display_name = "", template_id = semaphore_ex_project_template.test.id },
 { key = "Build", display_name = "Renamed", template_id = semaphore_ex_project_template.test.id },
 { key = "Added", display_name = "Renamed", template_id = semaphore_ex_project_template.test.id },
 ]
 edges = [{source_key="Build",destination_key="Deploy",condition="on_success"},{source_key="Deploy",destination_key="Added",condition="on_success"}]
 }`
	identities := map[string]string{}
	capture := func(state *terraform.State) error {
		values := state.RootModule().Resources[address].Primary.Attributes
		for i := 0; i < 2; i++ {
			prefix := fmt.Sprintf("nodes.%d.", i)
			identities[values[prefix+"key"]] = values[prefix+"server_id"]
		}
		return nil
	}
	verify := func(state *terraform.State) error {
		values := state.RootModule().Resources[address].Primary.Attributes
		for i := 0; i < 3; i++ {
			prefix := fmt.Sprintf("nodes.%d.", i)
			key := values[prefix+"key"]
			if old, ok := identities[key]; ok && old != values[prefix+"server_id"] {
				return fmt.Errorf("node %s was replaced after a label change", key)
			}
		}
		return nil
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: first, Check: resource.ComposeAggregateTestCheckFunc(capture, resource.TestCheckResourceAttr(address, "nodes.0.display_name", "Same"))},
		{Config: first, PlanOnly: true, ExpectNonEmptyPlan: false},
		{Config: changed, Check: resource.ComposeAggregateTestCheckFunc(verify, resource.TestCheckResourceAttr(address, "nodes.0.display_name", ""), resource.TestCheckResourceAttr(address, "nodes.1.display_name", "Renamed"))},
		{Config: changed, PlanOnly: true, ExpectNonEmptyPlan: false},
		{ResourceName: address, ImportState: true, ImportStateIdFunc: func(state *terraform.State) (string, error) {
			values := state.RootModule().Resources[address].Primary.Attributes
			return fmt.Sprintf("project/%s/workflow/%s", values["project_id"], values["id"]), nil
		}, ImportStateCheck: func(states []*terraform.InstanceState) error {
			if len(states) != 1 {
				return fmt.Errorf("expected one imported workflow")
			}
			values := states[0].Attributes
			if values["nodes.#"] != "3" {
				return fmt.Errorf("all nodes must import")
			}
			for i := 0; i < 3; i++ {
				prefix := fmt.Sprintf("nodes.%d.", i)
				if values[prefix+"key"] != "node-"+values[prefix+"server_id"] {
					return fmt.Errorf("ambiguous or empty labels must use server ID keys")
				}
			}
			return nil
		}},
	}})
}
