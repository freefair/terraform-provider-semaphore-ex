package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func workflowResourceNodeKeys(nodes []map[string]any, previous *exWorkflowDefinitionModel) (map[int64]string, error) {
	prior := map[int64]string{}
	if previous != nil && !previous.Nodes.IsNull() && !previous.Nodes.IsUnknown() {
		for _, value := range previous.Nodes.Elements() {
			node, ok := value.(types.Object)
			if !ok {
				continue
			}
			id, idOK := node.Attributes()["server_id"].(types.Int64)
			key, keyOK := node.Attributes()["key"].(types.String)
			if idOK && keyOK && id.ValueInt64() > 0 && !key.IsNull() && !key.IsUnknown() {
				prior[id.ValueInt64()] = key.ValueString()
			}
		}
	}
	keys := map[int64]string{}
	seenIDs := map[int64]bool{}
	used := map[string]bool{}
	labels := map[string]int{}
	for _, node := range nodes {
		label, _ := node["display_name"].(string)
		labels[label]++
	}
	for _, node := range nodes {
		id, err := identityNumber(node["id"])
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("workflow node has no valid server ID")
		}
		if seenIDs[id] {
			return nil, fmt.Errorf("workflow contains duplicate node IDs")
		}
		seenIDs[id] = true
		if key := prior[id]; key != "" {
			if used[key] {
				return nil, fmt.Errorf("prior workflow state contains duplicate keys")
			}
			keys[id] = key
			used[key] = true
		}
	}
	// Preserve the published import convention where possible. Labels only seed a
	// new Terraform key; existing state always follows server identity thereafter.
	for _, node := range nodes {
		id, _ := identityNumber(node["id"])
		if keys[id] != "" {
			continue
		}
		label, _ := node["display_name"].(string)
		if label != "" && labels[label] == 1 && !used[label] {
			keys[id] = label
			used[label] = true
		}
	}
	for _, node := range nodes {
		id, _ := identityNumber(node["id"])
		if keys[id] != "" {
			continue
		}
		key := "node-" + strconv.FormatInt(id, 10)
		for used[key] {
			key = "node-" + key
		}
		keys[id] = key
		used[key] = true
	}
	return keys, nil
}

// POST/PUT preserve the submitted node slice while replacing temporary client
// IDs with database IDs (WorkflowStoreImpl.replaceWorkflowGraph). Bind that
// response once; subsequent reads use server IDs and never rely on list order.
func workflowBindMutationIDs(ctx context.Context, plan exWorkflowDefinitionModel, raw map[string]any) (exWorkflowDefinitionModel, error) {
	nodes, err := workflowObjects(raw["nodes"])
	if err != nil {
		return plan, err
	}
	if len(nodes) != len(plan.Nodes.Elements()) {
		return plan, fmt.Errorf("workflow mutation returned a different number of nodes")
	}
	values := make([]attr.Value, 0, len(nodes))
	seen := map[int64]bool{}
	for index, element := range plan.Nodes.Elements() {
		node, ok := element.(types.Object)
		if !ok {
			return plan, fmt.Errorf("planned workflow node is invalid")
		}
		id, err := identityNumber(nodes[index]["id"])
		if err != nil || id <= 0 || seen[id] {
			return plan, fmt.Errorf("workflow mutation returned invalid node identities")
		}
		seen[id] = true
		attributes := node.Attributes()
		existing, _ := attributes["server_id"].(types.Int64)
		if existing.ValueInt64() > 0 && existing.ValueInt64() != id {
			return plan, fmt.Errorf("workflow mutation changed an existing node identity or order")
		}
		attributes["server_id"] = types.Int64Value(id)
		updated, diags := types.ObjectValue(node.AttributeTypes(ctx), attributes)
		if diags.HasError() {
			return plan, fmt.Errorf("workflow node identity could not be recorded")
		}
		values = append(values, updated)
	}
	list, diags := types.ListValue(plan.Nodes.ElementType(ctx), values)
	if diags.HasError() {
		return plan, fmt.Errorf("workflow mutation identities do not match the planned graph")
	}
	plan.Nodes = list
	return plan, nil
}
