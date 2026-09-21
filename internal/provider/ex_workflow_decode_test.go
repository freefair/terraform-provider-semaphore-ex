package provider

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/require"
)

func TestWorkflowDecodedDefinitionPreservesCompleteGraph(t *testing.T) {
	ctx := context.Background()
	decoder := json.NewDecoder(strings.NewReader(`{
  "id": 8, "project_id": 3, "name": "Release", "revision": 4, "definition_version": 1,
  "max_parallel_tasks": 2, "description": "reviewed", "start_version": "v1",
  "access_policy": {"revision": 2, "view_role_ids": ["reader"], "start_role_ids": ["operator"]},
  "parameters": [
    {"name": "version", "type": "string", "default": "v1"},
    {"name": "count", "type": "integer", "default": 9007199254740993},
    {"name": "check", "type": "boolean", "default": false}
  ],
  "nodes": [
    {"id": 22, "kind": "task", "display_name": "Build", "template_id": 7,
     "task_params": {"git_branch": "main", "arguments": "[]", "params": {"debug_level": 2}},
     "override_policy": {"inventory_ids": [4], "allow_arguments": true},
     "artifact_outputs": [{"name": "report", "sensitive": true, "max_bytes": 1024,
       "schema": {"type": "object", "required": ["passed"], "properties": {
         "passed": {"type": "boolean"}, "results": {"type": "array", "items": {"type": "string"}}
       }}}]},
    {"id": 31, "kind": "approval", "display_name": "Review",
     "approval_role_policy": {"revision": 3, "mode": "roles", "role_ids": ["reviewer"], "minimum_distinct_approvers": 2, "initiator_separation": true},
     "artifact_inputs": [{"name": "report", "source_node_id": 22, "output": "report", "required": true}],
     "cross_project_template_reference": {"grant_id": 2, "template_version_number": 1, "owner_project_id": 8, "template_id": 9, "template_version_id": 10, "content_fingerprint": "fingerprint", "grant_revision": 5}}
  ],
  "edges": [{"id": 9, "source_node_id": 22, "destination_node_id": 31, "condition": "on_success"}]
}`))
	decoder.UseNumber()
	var raw map[string]any
	require.NoError(t, decoder.Decode(&raw))
	result, err := workflowDecodedDefinition(ctx, raw)
	require.NoError(t, err)
	require.Equal(t, int64(4), result.Revision.ValueInt64())
	var nodes, edges, parameters []types.Object
	require.False(t, result.Nodes.ElementsAs(ctx, &nodes, false).HasError())
	require.False(t, result.Edges.ElementsAs(ctx, &edges, false).HasError())
	require.False(t, result.Parameters.ElementsAs(ctx, &parameters, false).HasError())
	require.Len(t, nodes, 2)
	require.Equal(t, types.StringValue("node-22"), nodes[0].Attributes()["key"])
	require.Equal(t, types.StringValue("node-31"), edges[0].Attributes()["destination_key"])
	require.False(t, nodes[0].Attributes()["task_params"].IsNull())
	parametersObject := nodes[0].Attributes()["task_params"].(types.Object)
	ansible := parametersObject.Attributes()["ansible"].(types.Object)
	require.Equal(t, types.Int64Value(2), ansible.Attributes()["debug_level"])
	require.False(t, nodes[0].Attributes()["artifact_outputs"].IsNull())
	require.False(t, nodes[1].Attributes()["artifact_inputs"].IsNull())
	require.False(t, nodes[1].Attributes()["cross_project_template_reference"].IsNull())
	require.False(t, nodes[1].Attributes()["approval_role_policy"].IsNull())
	require.False(t, result.AccessPolicy.IsNull())
	require.Equal(t, types.BoolValue(false), parameters[2].Attributes()["default_bool"])
	number := parameters[1].Attributes()["default_number"].(types.Number)
	require.Equal(t, "9007199254740993", number.ValueBigFloat().Text('f', 0))
	// Conversion must not overwrite the original API representation.
	require.NotContains(t, raw["nodes"].([]any)[0].(map[string]any), "key")
}

func TestWorkflowDecodedDefinitionRejectsBrokenReference(t *testing.T) {
	_, err := workflowDecodedDefinition(context.Background(), map[string]any{
		"nodes": []any{map[string]any{"id": int64(2)}},
		"edges": []any{map[string]any{"id": int64(3), "source_node_id": int64(2), "destination_node_id": int64(99)}},
	})
	require.ErrorContains(t, err, "unknown node")
}

func TestWorkflowDecodedResourceUsesDisplayNamesForGraphIdentity(t *testing.T) {
	result, err := workflowDecodedResource(context.Background(), map[string]any{
		"id": int64(8), "project_id": int64(3), "name": "Release", "revision": int64(4), "definition_version": int64(1), "max_parallel_tasks": int64(2),
		"nodes": []any{map[string]any{"id": int64(22), "display_name": "Build"}, map[string]any{"id": int64(31), "display_name": "Deploy", "artifact_inputs": []any{map[string]any{"name": "report", "source_node_id": int64(22), "output": "report", "required": true}}}},
		"edges": []any{map[string]any{"id": int64(9), "source_node_id": int64(22), "destination_node_id": int64(31), "condition": "on_success"}},
	})
	require.NoError(t, err)
	var nodes, edges []types.Object
	require.False(t, result.Nodes.ElementsAs(context.Background(), &nodes, false).HasError())
	require.False(t, result.Edges.ElementsAs(context.Background(), &edges, false).HasError())
	require.Equal(t, types.StringValue("Build"), nodes[0].Attributes()["key"])
	require.Equal(t, types.StringValue("Build"), edges[0].Attributes()["source_key"])
	inputs := nodes[1].Attributes()["artifact_inputs"].(types.List)
	var artifacts []types.Object
	require.False(t, inputs.ElementsAs(context.Background(), &artifacts, false).HasError())
	require.Equal(t, types.StringValue("Build"), artifacts[0].Attributes()["source_key"])
}

func TestWorkflowDecodedResourceAcceptsRepeatedDisplayName(t *testing.T) {
	_, err := workflowDecodedResource(context.Background(), map[string]any{"nodes": []any{map[string]any{"id": int64(2), "display_name": "Build"}, map[string]any{"id": int64(3), "display_name": "Build"}}})
	require.NoError(t, err)
}

func TestAcc_EXWorkflowDefinitionDataSource(t *testing.T) {
	suffix := acctest.RandString(8)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{
		Config: testAccWorkflowDefinitionConfig(suffix, "Workflow "+suffix),
		Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("data.semaphore_ex_workflow_definition.test", "nodes.#", "2"),
			resource.TestCheckResourceAttr("data.semaphore_ex_workflow_definition.test", "nodes.0.display_name", "Build"),
			resource.TestCheckResourceAttr("data.semaphore_ex_workflow_definition.test", "nodes.0.task_params.ansible.debug_level", "2"),
			resource.TestCheckResourceAttr("data.semaphore_ex_workflow_definition.test", "parameters.0.default_string", "v1"),
			resource.TestCheckResourceAttr("data.semaphore_ex_workflow_definition.test", "edges.#", "1"),
			resource.TestCheckResourceAttr("data.semaphore_ex_workflow_definition.test", "edges.0.condition", "on_success"),
		),
	}}})
}
