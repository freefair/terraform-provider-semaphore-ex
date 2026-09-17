package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/require"
)

func TestWorkflowDecodedResourceKeepsOmittedEdgesNull(t *testing.T) {
	result, err := workflowDecodedResource(context.Background(), map[string]any{
		"id": int64(1), "project_id": int64(2), "name": "single", "revision": int64(1), "definition_version": int64(1), "max_parallel_tasks": int64(1),
		"nodes": []any{map[string]any{"id": int64(3), "display_name": "Only"}},
	})
	require.NoError(t, err)
	require.True(t, result.Edges.IsNull(), "an omitted optional edges collection must remain null, not become an explicit empty list")
}

func TestAcc_EXWorkflowDefinitionOmittedEdgesAndEmptyNodeTaskParams(t *testing.T) {
	suffix := acctest.RandString(8)
	config := testAccProjectTemplateConfig(suffix, "") + fmt.Sprintf(`
resource "semaphore_ex_workflow_definition" "regression" {
  project_id = semaphore_ex_project.test.id
  name       = "omitted-edges-%s"
  nodes = [{
    key          = "Only"
    display_name = "Only"
    template_id  = semaphore_ex_project_template.test.id
    task_params  = { git_branch = "", ansible = { tags = [] } }
  }]
}
	`, suffix)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: config, Check: resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.regression", "nodes.#", "1"),
		resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.regression", "edges.#", "0"),
		resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.regression", "nodes.0.task_params.git_branch", ""),
		resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.regression", "nodes.0.task_params.ansible.tags.#", "0"),
	)}}})
}

func TestAcc_EXWorkflowDefinitionPreservesOmittedComputedFields(t *testing.T) {
	suffix := acctest.RandString(8)
	base := testAccProjectTemplateConfig(suffix, "")
	full := base + `
resource "semaphore_ex_workflow_definition" "preserve" {
  project_id = semaphore_ex_project.test.id
  name = "preserve-before"
  max_parallel_tasks = 7
  parameters = [{ name = "release", type = "string", description = "release label", default_string = "v1" }]
  nodes = [
    { key = "Build", display_name = "Build", template_id = semaphore_ex_project_template.test.id, position_x = 120, position_y = 40, note = "build note" },
    { key = "Deploy", display_name = "Deploy", template_id = semaphore_ex_project_template.test.id, position_x = 360, position_y = 40 },
  ]
  edges = [{ source_key = "Build", destination_key = "Deploy", condition = "on_success", label = "deploy after build" }]
}`
	omitted := base + `
resource "semaphore_ex_workflow_definition" "preserve" {
  project_id = semaphore_ex_project.test.id
  name = "preserve-after"
  parameters = [{ name = "release", type = "string", description = "release label", default_string = "v1" }]
  nodes = [
    { key = "Build", display_name = "Build", template_id = semaphore_ex_project_template.test.id },
    { key = "Deploy", display_name = "Deploy", template_id = semaphore_ex_project_template.test.id },
  ]
  edges = [{ source_key = "Build", destination_key = "Deploy" }]
}`
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: full, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.preserve", "max_parallel_tasks", "7"), resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.preserve", "nodes.0.position_x", "120"), resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.preserve", "nodes.0.note", "build note"), resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.preserve", "edges.0.label", "deploy after build"), resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.preserve", "parameters.0.description", "release label"))},
		{Config: omitted, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.preserve", "name", "preserve-after"), resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.preserve", "max_parallel_tasks", "7"), resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.preserve", "nodes.0.position_x", "120"), resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.preserve", "nodes.0.note", "build note"), resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.preserve", "edges.0.label", "deploy after build"), resource.TestCheckResourceAttr("semaphore_ex_workflow_definition.preserve", "parameters.0.description", "release label"))},
	}})
}

func TestWorkflowTaskParamsStatePreservesExplicitEmptyPriorValues(t *testing.T) {
	prior := &TaskParamsModel{GitBranch: types.StringValue(""), Ansible: &AnsibleTaskParamsModel{Tags: types.ListValueMust(types.StringType, nil)}}
	value, err := workflowTaskParamsState(context.Background(), &models.TaskPrams{}, prior)
	require.NoError(t, err)
	require.NotNil(t, value)
}

func TestWorkflowRefreshDetectsRemovedParametersAndEdges(t *testing.T) {
	ctx := context.Background()
	definition := map[string]any{
		"id": int64(1), "project_id": int64(2), "name": "drift", "revision": int64(1), "definition_version": int64(1), "max_parallel_tasks": int64(1),
		"nodes":      []any{map[string]any{"id": int64(3), "display_name": "First"}, map[string]any{"id": int64(4), "display_name": "Second"}},
		"edges":      []any{map[string]any{"id": int64(5), "source_node_id": int64(3), "destination_node_id": int64(4), "condition": "on_success"}},
		"parameters": []any{map[string]any{"name": "release", "type": "string", "default": "v1"}},
	}
	prior, err := workflowDecodedResource(ctx, definition)
	require.NoError(t, err)
	require.Len(t, prior.Parameters.Elements(), 1)
	require.Len(t, prior.Edges.Elements(), 1)
	delete(definition, "parameters")
	delete(definition, "edges")
	current, err := workflowState(ctx, prior, definition)
	require.NoError(t, err)
	require.True(t, current.Parameters.IsNull(), "removed nonempty parameters must be visible as drift")
	require.True(t, current.Edges.IsNull(), "removed nonempty edges must be visible as drift")
}

func TestAcc_EXWorkflowArtifactPropertyOrder(t *testing.T) {
	config := testAccWorkflowDefinitionRichConfig(acctest.RandString(8), false, false)
	config = strings.ReplaceAll(config, `properties = [{ name = "passed", schema = { type = "boolean" } }]`, `properties = [{ name = "zeta", schema = { type = "string" } }, { name = "passed", schema = { type = "boolean" } }]`)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: config}}})
}

func TestWorkflowArtifactSchemaRejectsDuplicatePropertyNames(t *testing.T) {
	schema := map[string]any{"type": "object", "properties": []any{
		map[string]any{"name": "value", "schema": map[string]any{"type": "string"}},
		map[string]any{"name": "value", "schema": map[string]any{"type": "boolean"}},
	}}
	require.ErrorContains(t, workflowArtifactSchemaWire(schema), "duplicated")
}
