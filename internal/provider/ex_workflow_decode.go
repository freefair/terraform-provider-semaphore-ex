package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// workflowDecodedDefinition maps the complete API graph, including references,
// without requiring a previously configured Terraform state.
func workflowDecodedDefinition(ctx context.Context, raw map[string]any) (exWorkflowDefinitionDataSourceModel, error) {
	var result exWorkflowDefinitionDataSourceModel
	wire := workflowCopyObject(raw)
	nodes, err := workflowObjects(raw["nodes"])
	if err != nil {
		return result, err
	}
	keys := make(map[int64]string, len(nodes))
	for _, node := range nodes {
		id, err := identityNumber(node["id"])
		if err != nil || id <= 0 {
			return result, fmt.Errorf("workflow node has no valid server ID")
		}
		if _, duplicate := keys[id]; duplicate {
			return result, fmt.Errorf("workflow contains duplicate node IDs")
		}
		keys[id] = "node-" + strconv.FormatInt(id, 10)
	}
	decodedNodes := make([]any, 0, len(nodes))
	for _, rawNode := range nodes {
		node := workflowCopyObject(rawNode)
		if node["display_name"] == nil {
			node["display_name"] = ""
		}
		id, _ := identityNumber(node["id"])
		node["server_id"], node["key"] = id, keys[id]
		if node["task_params"] != nil {
			parameters, err := workflowTaskParamsState(ctx, node["task_params"])
			if err != nil {
				return result, err
			}
			node["task_params"] = parameters
		}
		inputs, err := workflowObjects(node["artifact_inputs"])
		if err != nil {
			return result, err
		}
		if node["artifact_inputs"] != nil {
			mapped := make([]any, 0, len(inputs))
			for _, rawInput := range inputs {
				input := workflowCopyObject(rawInput)
				key, err := workflowReferencedKey(input["source_node_id"], keys)
				if err != nil {
					return result, err
				}
				input["source_key"] = key
				mapped = append(mapped, input)
			}
			node["artifact_inputs"] = mapped
		}
		outputs, err := workflowObjects(node["artifact_outputs"])
		if err != nil {
			return result, err
		}
		if node["artifact_outputs"] != nil {
			mapped := make([]any, 0, len(outputs))
			for _, rawOutput := range outputs {
				output := workflowCopyObject(rawOutput)
				schema, ok := output["schema"].(map[string]any)
				if !ok {
					return result, fmt.Errorf("workflow artifact has no schema")
				}
				output["schema"], err = workflowArtifactSchemaState(schema, 4)
				if err != nil {
					return result, err
				}
				mapped = append(mapped, output)
			}
			node["artifact_outputs"] = mapped
		}
		decodedNodes = append(decodedNodes, node)
	}
	wire["nodes"] = decodedNodes
	edges, err := workflowObjects(raw["edges"])
	if err != nil {
		return result, err
	}
	if raw["edges"] != nil {
		decodedEdges := make([]any, 0, len(edges))
		for _, rawEdge := range edges {
			edge := workflowCopyObject(rawEdge)
			edge["server_id"] = edge["id"]
			for _, field := range []struct{ source, target string }{{"source_node_id", "source_key"}, {"destination_node_id", "destination_key"}} {
				key, err := workflowReferencedKey(edge[field.source], keys)
				if err != nil {
					return result, err
				}
				edge[field.target] = key
			}
			decodedEdges = append(decodedEdges, edge)
		}
		wire["edges"] = decodedEdges
	}
	parameters, err := workflowObjects(raw["parameters"])
	if err != nil {
		return result, err
	}
	if raw["parameters"] != nil {
		mapped := make([]any, 0, len(parameters))
		for _, rawParameter := range parameters {
			parameter := workflowCopyObject(rawParameter)
			switch value := parameter["default"].(type) {
			case string:
				parameter["default_string"] = value
			case bool:
				parameter["default_bool"] = value
			case json.Number, int, int64:
				parameter["default_number"] = value
			case nil:
			default:
				return result, fmt.Errorf("workflow parameter has an unsupported default type")
			}
			mapped = append(mapped, parameter)
		}
		wire["parameters"] = mapped
	}
	decoded, err := exTypedValue(ctx, workflowResourceSchema().Type(), wire)
	if err != nil {
		return result, err
	}
	object, ok := decoded.(types.Object)
	if !ok {
		return result, fmt.Errorf("workflow API returned an invalid definition")
	}
	if diagnostics := object.As(ctx, &result, basetypes.ObjectAsOptions{}); diagnostics.HasError() {
		return result, fmt.Errorf("workflow definition does not match its declared schema")
	}
	return result, nil
}

func workflowDecodedResource(ctx context.Context, raw map[string]any, previous ...*exWorkflowDefinitionModel) (exWorkflowDefinitionModel, error) {
	var result exWorkflowDefinitionModel
	wire := workflowCopyObject(raw)
	nodes, err := workflowObjects(raw["nodes"])
	if err != nil {
		return result, err
	}
	var prior *exWorkflowDefinitionModel
	if len(previous) > 0 {
		prior = previous[0]
	}
	keys, err := workflowResourceNodeKeys(nodes, prior)
	if err != nil {
		return result, err
	}
	priorTaskParams := map[string]*TaskParamsModel{}
	if len(previous) > 0 && previous[0] != nil && !previous[0].Nodes.IsNull() && !previous[0].Nodes.IsUnknown() {
		var priorNodes []types.Object
		if d := previous[0].Nodes.ElementsAs(ctx, &priorNodes, false); !d.HasError() {
			for _, priorNode := range priorNodes {
				name, _ := priorNode.Attributes()["key"].(types.String)
				params, _ := priorNode.Attributes()["task_params"].(types.Object)
				if name.IsNull() || name.IsUnknown() || params.IsNull() || params.IsUnknown() {
					continue
				}
				var model TaskParamsModel
				if d := params.As(ctx, &model, basetypes.ObjectAsOptions{}); !d.HasError() {
					priorTaskParams[name.ValueString()] = &model
				}
			}
		}
	}
	decodedNodes := make([]any, 0, len(nodes))
	for _, rawNode := range nodes {
		node := workflowCopyObject(rawNode)
		if node["display_name"] == nil {
			node["display_name"] = ""
		}
		id, _ := identityNumber(node["id"])
		node["server_id"], node["key"] = id, keys[id]
		if node["task_params"] != nil || priorTaskParams[keys[id]] != nil {
			parameters, err := workflowTaskParamsState(ctx, node["task_params"], priorTaskParams[keys[id]])
			if err != nil {
				return result, err
			}
			node["task_params"] = parameters
		}
		if node["artifact_inputs"] != nil {
			inputs, err := workflowObjects(node["artifact_inputs"])
			if err != nil {
				return result, err
			}
			mapped := make([]any, 0, len(inputs))
			for _, rawInput := range inputs {
				input := workflowCopyObject(rawInput)
				key, err := workflowReferencedKey(input["source_node_id"], keys)
				if err != nil {
					return result, err
				}
				input["source_key"] = key
				mapped = append(mapped, input)
			}
			node["artifact_inputs"] = mapped
		}
		if node["artifact_outputs"] != nil {
			outputs, err := workflowObjects(node["artifact_outputs"])
			if err != nil {
				return result, err
			}
			mapped := make([]any, 0, len(outputs))
			for _, rawOutput := range outputs {
				output := workflowCopyObject(rawOutput)
				schema, ok := output["schema"].(map[string]any)
				if !ok {
					return result, fmt.Errorf("workflow artifact has no schema")
				}
				output["schema"], err = workflowArtifactSchemaState(schema, 4)
				if err != nil {
					return result, err
				}
				mapped = append(mapped, output)
			}
			node["artifact_outputs"] = mapped
		}
		decodedNodes = append(decodedNodes, node)
	}
	wire["nodes"] = decodedNodes
	edges, err := workflowObjects(raw["edges"])
	if err != nil {
		return result, err
	}
	decodedEdges := make([]any, 0, len(edges))
	for _, rawEdge := range edges {
		edge := workflowCopyObject(rawEdge)
		edge["server_id"] = edge["id"]
		for _, field := range []struct{ source, target string }{{"source_node_id", "source_key"}, {"destination_node_id", "destination_key"}} {
			key, err := workflowReferencedKey(edge[field.source], keys)
			if err != nil {
				return result, err
			}
			edge[field.target] = key
		}
		decodedEdges = append(decodedEdges, edge)
	}
	if len(decodedEdges) > 0 {
		wire["edges"] = decodedEdges
	} else {
		wire["edges"] = nil
	}
	parameters, err := workflowObjects(raw["parameters"])
	if err != nil {
		return result, err
	}
	if raw["parameters"] != nil {
		mapped := make([]any, 0, len(parameters))
		for _, rawParameter := range parameters {
			parameter := workflowCopyObject(rawParameter)
			switch value := parameter["default"].(type) {
			case string:
				parameter["default_string"] = value
			case bool:
				parameter["default_bool"] = value
			case json.Number, int, int64:
				parameter["default_number"] = value
			case nil:
			default:
				return result, fmt.Errorf("workflow parameter has an unsupported default type")
			}
			mapped = append(mapped, parameter)
		}
		wire["parameters"] = mapped
	}
	decoded, err := exTypedValue(ctx, workflowResourceSchema().Type(), wire)
	if err != nil {
		return result, err
	}
	object, ok := decoded.(types.Object)
	if !ok {
		return result, fmt.Errorf("workflow API returned an invalid definition")
	}
	if diagnostics := object.As(ctx, &result, basetypes.ObjectAsOptions{}); diagnostics.HasError() {
		return result, fmt.Errorf("workflow definition does not match its declared schema")
	}
	return result, nil
}

func workflowTaskParamsState(ctx context.Context, raw any, previous ...*TaskParamsModel) (any, error) {
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("workflow API task parameters are invalid")
	}
	var wire models.TaskPrams
	if err := json.Unmarshal(encoded, &wire); err != nil {
		return nil, fmt.Errorf("workflow API task parameters have invalid field types")
	}
	model := convertTaskPramsToTaskParamsModel(ctx, &wire, previous...)
	target, ok := TaskParamsAttribute().GetResource(ctx).GetType().(types.ObjectType)
	if !ok {
		return nil, fmt.Errorf("workflow task parameter schema is invalid")
	}
	object, diagnostics := types.ObjectValueFrom(ctx, target.AttrTypes, model)
	if diagnostics.HasError() {
		return nil, fmt.Errorf("workflow task parameters do not match their schema")
	}
	return exWireValue(ctx, object)
}

func workflowCopyObject(raw map[string]any) map[string]any {
	result := make(map[string]any, len(raw))
	for key, value := range raw {
		result[key] = value
	}
	return result
}

func workflowObjects(raw any) ([]map[string]any, error) {
	if raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("workflow API returned an invalid collection")
	}
	result := make([]map[string]any, len(items))
	for index, item := range items {
		value, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("workflow API returned an invalid collection item")
		}
		result[index] = value
	}
	return result, nil
}

func workflowReferencedKey(raw any, keys map[int64]string) (string, error) {
	id, err := identityNumber(raw)
	key, exists := keys[id]
	if err != nil || !exists {
		return "", fmt.Errorf("workflow API returned a reference to an unknown node")
	}
	return key, nil
}

func workflowArtifactSchemaState(raw map[string]any, depth int) (map[string]any, error) {
	result := workflowCopyObject(raw)
	if depth == 1 && (raw["properties"] != nil || raw["items"] != nil || raw["required"] != nil) {
		return nil, fmt.Errorf("workflow artifact schema exceeds the supported nesting depth")
	}
	if raw["properties"] != nil {
		properties, ok := raw["properties"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("workflow artifact properties are invalid")
		}
		names := make([]string, 0, len(properties))
		for name := range properties {
			names = append(names, name)
		}
		sort.Strings(names)
		mapped := make([]any, 0, len(names))
		for _, name := range names {
			property, ok := properties[name].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("workflow artifact property schema is invalid")
			}
			schema, err := workflowArtifactSchemaState(property, depth-1)
			if err != nil {
				return nil, err
			}
			mapped = append(mapped, map[string]any{"name": name, "schema": schema})
		}
		result["properties"] = mapped
	}
	if raw["items"] != nil {
		items, ok := raw["items"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("workflow artifact item schema is invalid")
		}
		mapped, err := workflowArtifactSchemaState(items, depth-1)
		if err != nil {
			return nil, err
		}
		result["items"] = mapped
	}
	return result, nil
}
