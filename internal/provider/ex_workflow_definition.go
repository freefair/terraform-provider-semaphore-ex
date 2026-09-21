package provider

// The workflow endpoint intentionally has no generated client: it is an EX
// contract and the provider keeps its definition model independent from the
// upstream OpenAPI snapshot.

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rs "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

type exWorkflowDefinitionModel struct {
	ID                types.Int64  `tfsdk:"id"`
	ProjectID         types.Int64  `tfsdk:"project_id"`
	Name              types.String `tfsdk:"name"`
	Description       types.String `tfsdk:"description"`
	StartVersion      types.String `tfsdk:"start_version"`
	VersionMessage    types.String `tfsdk:"version_message"`
	DefinitionVersion types.Int64  `tfsdk:"definition_version"`
	Revision          types.Int64  `tfsdk:"revision"`
	MaxParallelTasks  types.Int64  `tfsdk:"max_parallel_tasks"`
	Parameters        types.List   `tfsdk:"parameters"`
	AccessPolicy      types.Object `tfsdk:"access_policy"`
	Nodes             types.List   `tfsdk:"nodes"`
	Edges             types.List   `tfsdk:"edges"`
}
type exWorkflowDefinitionDataSourceModel struct {
	ID                types.Int64  `tfsdk:"id"`
	ProjectID         types.Int64  `tfsdk:"project_id"`
	Name              types.String `tfsdk:"name"`
	Description       types.String `tfsdk:"description"`
	StartVersion      types.String `tfsdk:"start_version"`
	VersionMessage    types.String `tfsdk:"version_message"`
	DefinitionVersion types.Int64  `tfsdk:"definition_version"`
	Revision          types.Int64  `tfsdk:"revision"`
	MaxParallelTasks  types.Int64  `tfsdk:"max_parallel_tasks"`
	Parameters        types.List   `tfsdk:"parameters"`
	AccessPolicy      types.Object `tfsdk:"access_policy"`
	Nodes             types.List   `tfsdk:"nodes"`
	Edges             types.List   `tfsdk:"edges"`
}
type exWorkflowDefinitionResource struct{ client *apiclient.SemaphoreUI }
type exWorkflowDefinitionDataSource struct{ client *apiclient.SemaphoreUI }

func NewWorkflowDefinitionResource() resource.Resource { return &exWorkflowDefinitionResource{} }
func NewWorkflowDefinitionDataSource() datasource.DataSource {
	return withNamedLookup(&exWorkflowDefinitionDataSource{}, "workflow_definition")
}
func (r *exWorkflowDefinitionResource) Metadata(_ context.Context, q resource.MetadataRequest, p *resource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_workflow_definition"
}
func (d *exWorkflowDefinitionDataSource) Metadata(_ context.Context, q datasource.MetadataRequest, p *datasource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_workflow_definition"
}
func (r *exWorkflowDefinitionResource) Configure(_ context.Context, q resource.ConfigureRequest, p *resource.ConfigureResponse) {
	if q.ProviderData == nil {
		return
	}
	c, ok := q.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		p.Diagnostics.AddError("Unexpected Resource Configure Type", "Expected *client.SemaphoreUI.")
		return
	}
	r.client = c
}
func (d *exWorkflowDefinitionDataSource) Configure(_ context.Context, q datasource.ConfigureRequest, p *datasource.ConfigureResponse) {
	if q.ProviderData == nil {
		return
	}
	c, ok := q.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		p.Diagnostics.AddError("Unexpected Data Source Configure Type", "Expected *client.SemaphoreUI.")
		return
	}
	d.client = c
}

func workflowNodeAttrs(computed bool) map[string]rs.Attribute { // kept native so references are validated by Terraform's type system
	opt := func() bool { return !computed }
	return map[string]rs.Attribute{
		"key": rs.StringAttribute{Required: !computed, Computed: computed, MarkdownDescription: "Unique persisted node identity; it must equal display_name."}, "server_id": rs.Int64Attribute{Computed: true},
		"template_id": rs.Int64Attribute{Optional: opt(), Computed: true}, "display_name": rs.StringAttribute{Optional: opt(), Computed: true}, "kind": rs.StringAttribute{Optional: opt(), Computed: true}, "convergence_mode": rs.StringAttribute{Optional: opt(), Computed: true}, "join_mode": rs.StringAttribute{Optional: opt(), Computed: true}, "position_x": rs.Int64Attribute{Optional: opt(), Computed: true}, "position_y": rs.Int64Attribute{Optional: opt(), Computed: true}, "note": rs.StringAttribute{Optional: opt(), Computed: true}, "delay_seconds": rs.Int64Attribute{Optional: opt(), Computed: true},
		"approval_timeout": rs.Int64Attribute{Optional: opt(), Computed: true}, "approval_message": rs.StringAttribute{Optional: opt(), Computed: true}, "approval_permission": rs.Int64Attribute{Optional: opt(), Computed: true}, "approval_timeout_outcome": rs.StringAttribute{Optional: opt(), Computed: true}, "approval_separation_of_duties": rs.BoolAttribute{Optional: opt(), Computed: true},
		"approval_role_policy":             rs.SingleNestedAttribute{Optional: opt(), Computed: true, Attributes: map[string]rs.Attribute{"revision": rs.Int64Attribute{Computed: true}, "mode": rs.StringAttribute{Optional: opt(), Computed: true}, "role_ids": rs.ListAttribute{Optional: opt(), Computed: true, ElementType: types.StringType}, "minimum_distinct_approvers": rs.Int64Attribute{Optional: opt(), Computed: true}, "initiator_separation": rs.BoolAttribute{Optional: opt(), Computed: true}}},
		"cross_project_template_reference": rs.SingleNestedAttribute{Optional: opt(), Computed: true, Attributes: map[string]rs.Attribute{"grant_id": rs.Int64Attribute{Required: !computed, Computed: computed}, "template_version_number": rs.Int64Attribute{Required: !computed, Computed: computed}, "owner_project_id": rs.Int64Attribute{Computed: true}, "template_id": rs.Int64Attribute{Computed: true}, "template_version_id": rs.Int64Attribute{Computed: true}, "content_fingerprint": rs.StringAttribute{Computed: true}, "grant_revision": rs.Int64Attribute{Computed: true}}},
		"override_policy":                  rs.SingleNestedAttribute{Optional: opt(), Computed: true, Attributes: map[string]rs.Attribute{"inventory_ids": rs.ListAttribute{Optional: opt(), Computed: true, ElementType: types.Int64Type}, "environment_ids": rs.ListAttribute{Optional: opt(), Computed: true, ElementType: types.Int64Type}, "credential_parameters": rs.ListAttribute{Optional: opt(), Computed: true, ElementType: types.StringType}, "allow_arguments": rs.BoolAttribute{Optional: opt(), Computed: true}, "allow_branch": rs.BoolAttribute{Optional: opt(), Computed: true}}},
		"task_params":                      TaskParamsAttribute().GetResource(context.Background()),
		"artifact_outputs":                 rs.ListNestedAttribute{Optional: opt(), Computed: true, NestedObject: rs.NestedAttributeObject{Attributes: map[string]rs.Attribute{"name": rs.StringAttribute{Required: !computed, Computed: computed}, "sensitive": rs.BoolAttribute{Required: !computed, Computed: computed}, "max_bytes": rs.Int64Attribute{Required: !computed, Computed: computed}, "schema": rs.SingleNestedAttribute{Required: !computed, Computed: computed, Attributes: workflowArtifactSchemaAttrs(4, computed)}}}},
		"artifact_inputs":                  rs.ListNestedAttribute{Optional: opt(), Computed: true, NestedObject: rs.NestedAttributeObject{Attributes: map[string]rs.Attribute{"name": rs.StringAttribute{Required: !computed, Computed: computed}, "source_key": rs.StringAttribute{Required: !computed, Computed: computed}, "output": rs.StringAttribute{Required: !computed, Computed: computed}, "required": rs.BoolAttribute{Required: !computed, Computed: computed}}}},
	}
}
func workflowArtifactSchemaAttrs(depth int, computed bool) map[string]rs.Attribute {
	attrs := map[string]rs.Attribute{"type": rs.StringAttribute{Required: !computed, Computed: computed}}
	if depth == 1 {
		return attrs
	}
	attrs["required"] = rs.ListAttribute{Optional: !computed, Computed: computed, ElementType: types.StringType}
	attrs["properties"] = rs.SetNestedAttribute{Optional: !computed, Computed: computed, NestedObject: rs.NestedAttributeObject{Attributes: map[string]rs.Attribute{"name": rs.StringAttribute{Required: !computed, Computed: computed}, "schema": rs.SingleNestedAttribute{Required: !computed, Computed: computed, Attributes: workflowArtifactSchemaAttrs(depth-1, computed)}}}}
	attrs["items"] = rs.SingleNestedAttribute{Optional: !computed, Computed: computed, Attributes: workflowArtifactSchemaAttrs(depth-1, computed)}
	return attrs
}
func workflowEdgeAttrs(computed bool) map[string]rs.Attribute {
	return map[string]rs.Attribute{"server_id": rs.Int64Attribute{Computed: true}, "source_key": rs.StringAttribute{Required: !computed, Computed: computed}, "destination_key": rs.StringAttribute{Required: !computed, Computed: computed}, "condition": rs.StringAttribute{Optional: !computed, Computed: true}, "label": rs.StringAttribute{Optional: !computed, Computed: true}, "condition_expression": rs.StringAttribute{Optional: !computed, Computed: true}}
}
func workflowParamAttrs(computed bool) map[string]rs.Attribute {
	return map[string]rs.Attribute{"name": rs.StringAttribute{Required: !computed, Computed: computed}, "description": rs.StringAttribute{Optional: !computed, Computed: computed}, "type": rs.StringAttribute{Required: !computed, Computed: computed}, "required": rs.BoolAttribute{Optional: !computed, Computed: computed}, "default_string": rs.StringAttribute{Optional: !computed, Computed: computed}, "default_number": rs.NumberAttribute{Optional: !computed, Computed: computed}, "default_bool": rs.BoolAttribute{Optional: !computed, Computed: computed}, "min_length": rs.Int64Attribute{Optional: !computed, Computed: computed}, "max_length": rs.Int64Attribute{Optional: !computed, Computed: computed}, "minimum": rs.Int64Attribute{Optional: !computed, Computed: computed}, "maximum": rs.Int64Attribute{Optional: !computed, Computed: computed}, "options": rs.ListAttribute{Optional: !computed, Computed: computed, ElementType: types.StringType}, "secret_options": rs.ListNestedAttribute{Optional: !computed, Computed: computed, NestedObject: rs.NestedAttributeObject{Attributes: map[string]rs.Attribute{"access_key_id": rs.Int64Attribute{Optional: !computed, Computed: computed}, "global_credential_id": rs.Int64Attribute{Optional: !computed, Computed: computed}, "label": rs.StringAttribute{Optional: !computed, Computed: computed}}}}}
}

func workflowResourceSchema() rs.Schema {
	return rs.Schema{MarkdownDescription: "Manages a persisted Semaphore EX workflow graph; it never starts a workflow.", Attributes: map[string]rs.Attribute{
		"id": rs.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}}, "project_id": rs.Int64Attribute{Required: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}}, "name": rs.StringAttribute{Required: true}, "description": rs.StringAttribute{Optional: true}, "start_version": rs.StringAttribute{Optional: true}, "version_message": rs.StringAttribute{Optional: true}, "definition_version": rs.Int64Attribute{Computed: true}, "revision": rs.Int64Attribute{Computed: true}, "max_parallel_tasks": rs.Int64Attribute{Optional: true, Computed: true},
		"parameters": rs.ListNestedAttribute{Optional: true, NestedObject: rs.NestedAttributeObject{Attributes: workflowParamAttrs(false)}}, "access_policy": rs.SingleNestedAttribute{Optional: true, Computed: true, Attributes: map[string]rs.Attribute{"revision": rs.Int64Attribute{Computed: true}, "view_role_ids": rs.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType}, "start_role_ids": rs.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType}}}, "nodes": rs.ListNestedAttribute{Required: true, NestedObject: rs.NestedAttributeObject{Attributes: workflowNodeAttrs(false)}}, "edges": rs.ListNestedAttribute{Optional: true, NestedObject: rs.NestedAttributeObject{Attributes: workflowEdgeAttrs(false)}},
	}}
}
func (r *exWorkflowDefinitionResource) Schema(_ context.Context, _ resource.SchemaRequest, p *resource.SchemaResponse) {
	p.Schema = workflowResourceSchema()
}
func (d *exWorkflowDefinitionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, p *datasource.SchemaResponse) {
	objectType, _ := workflowResourceSchema().Type().(types.ObjectType)
	resourceTypes := objectType.AttrTypes
	parametersType, _ := resourceTypes["parameters"].(types.ListType)
	nodesType, _ := resourceTypes["nodes"].(types.ListType)
	edgesType, _ := resourceTypes["edges"].(types.ListType)
	policyType, _ := resourceTypes["access_policy"].(types.ObjectType)
	p.Schema = ds.Schema{MarkdownDescription: "Reads a Semaphore EX workflow definition.", Attributes: map[string]ds.Attribute{"id": ds.Int64Attribute{Required: true}, "project_id": ds.Int64Attribute{Required: true}, "name": ds.StringAttribute{Computed: true}, "description": ds.StringAttribute{Computed: true}, "start_version": ds.StringAttribute{Computed: true}, "version_message": ds.StringAttribute{Computed: true}, "definition_version": ds.Int64Attribute{Computed: true}, "revision": ds.Int64Attribute{Computed: true}, "max_parallel_tasks": ds.Int64Attribute{Computed: true}, "parameters": ds.ListAttribute{Computed: true, ElementType: parametersType.ElemType}, "access_policy": ds.ObjectAttribute{Computed: true, AttributeTypes: policyType.AttrTypes}, "nodes": ds.ListAttribute{Computed: true, ElementType: nodesType.ElemType}, "edges": ds.ListAttribute{Computed: true, ElementType: edgesType.ElemType}}}
}

func workflowRoute(m exWorkflowDefinitionModel) map[string]string {
	return map[string]string{"project_id": strconv.FormatInt(m.ProjectID.ValueInt64(), 10), "workflow_id": strconv.FormatInt(m.ID.ValueInt64(), 10)}
}
func workflowWireObject(ctx context.Context, value types.Object, omit ...string) (map[string]any, error) {
	skipped := make(map[string]struct{}, len(omit))
	for _, name := range omit {
		skipped[name] = struct{}{}
	}
	result := map[string]any{}
	for name, item := range value.Attributes() {
		if _, ok := skipped[name]; ok || item.IsNull() {
			continue
		}
		if item.IsUnknown() {
			continue
		}
		if name == "task_params" {
			object, ok := item.(types.Object)
			if !ok {
				return nil, fmt.Errorf("workflow task parameters must be an object")
			}
			var parameters TaskParamsModel
			if diagnostics := object.As(ctx, &parameters, basetypes.ObjectAsOptions{}); diagnostics.HasError() {
				return nil, fmt.Errorf("workflow task parameters are invalid")
			}
			result[name] = convertTaskParamsModelToTaskPrams(ctx, &parameters)
			continue
		}
		wire, err := workflowWireValue(ctx, item)
		if err != nil {
			return nil, err
		}
		result[name] = wire
	}
	return result, nil
}

func workflowWireValue(ctx context.Context, value attr.Value) (any, error) {
	switch item := value.(type) {
	case types.Object:
		return workflowWireObject(ctx, item)
	case types.List:
		result := make([]any, 0, len(item.Elements()))
		for _, element := range item.Elements() {
			wire, err := workflowWireValue(ctx, element)
			if err != nil {
				return nil, err
			}
			result = append(result, wire)
		}
		return result, nil
	default:
		return exWireValue(ctx, value)
	}
}
func workflowDefinitionBody(ctx context.Context, m exWorkflowDefinitionModel, prior *exWorkflowDefinitionModel) (map[string]any, error) {
	// Use explicit fields to prevent Terraform-only identities from crossing the API boundary.
	maxParallelTasks := m.MaxParallelTasks
	if maxParallelTasks.IsUnknown() && prior != nil {
		maxParallelTasks = prior.MaxParallelTasks
	}
	body := map[string]any{"name": m.Name.ValueString(), "max_parallel_tasks": maxParallelTasks.ValueInt64()}
	if !m.Description.IsNull() {
		body["description"] = m.Description.ValueString()
	}
	if !m.StartVersion.IsNull() {
		body["start_version"] = m.StartVersion.ValueString()
	}
	if !m.VersionMessage.IsNull() {
		body["version_message"] = m.VersionMessage.ValueString()
	}
	if prior != nil {
		body["revision"] = prior.Revision.ValueInt64()
	}
	parametersValue := m.Parameters
	if parametersValue.IsUnknown() && prior != nil {
		parametersValue = prior.Parameters
	}
	if !parametersValue.IsNull() && !parametersValue.IsUnknown() {
		var parameters []types.Object
		if d := parametersValue.ElementsAs(ctx, &parameters, false); d.HasError() {
			return nil, fmt.Errorf("parameters are invalid")
		}
		priorParameters := map[string]types.Object{}
		if prior != nil && !prior.Parameters.IsNull() && !prior.Parameters.IsUnknown() {
			var values []types.Object
			if d := prior.Parameters.ElementsAs(ctx, &values, false); !d.HasError() {
				for _, value := range values {
					name, _ := value.Attributes()["name"].(types.String)
					priorParameters[name.ValueString()] = value
				}
			}
		}
		out := make([]any, 0, len(parameters))
		for _, parameter := range parameters {
			name, _ := parameter.Attributes()["name"].(types.String)
			if previous, exists := priorParameters[name.ValueString()]; exists {
				parameter = workflowMergeUnknownNode(parameter, previous)
			}
			wire, e := workflowWireObject(ctx, parameter)
			if e != nil {
				return nil, e
			}
			item := wire
			defaults := 0
			for _, valueName := range []string{"default_string", "default_number", "default_bool"} {
				if value, ok := item[valueName]; ok {
					defaults++
					item["default"] = value
					delete(item, valueName)
				}
			}
			if defaults > 1 {
				return nil, fmt.Errorf("a workflow parameter can have only one typed default")
			}
			out = append(out, item)
		}
		body["parameters"] = out
	}
	policy := m.AccessPolicy
	if (policy.IsNull() || policy.IsUnknown()) && prior != nil {
		policy = prior.AccessPolicy
	}
	if !policy.IsNull() && !policy.IsUnknown() {
		wire, e := workflowWireObject(ctx, policy)
		if e != nil {
			return nil, e
		}
		if prior != nil && !prior.AccessPolicy.IsNull() && !prior.AccessPolicy.IsUnknown() {
			priorWire, priorErr := workflowWireObject(ctx, prior.AccessPolicy)
			if priorErr != nil {
				return nil, priorErr
			}
			wire["revision"] = priorWire["revision"]
		}
		body["access_policy"] = wire
	}
	if !m.Nodes.IsNull() && !m.Nodes.IsUnknown() {
		var nodes []types.Object
		if d := m.Nodes.ElementsAs(ctx, &nodes, false); d.HasError() {
			return nil, fmt.Errorf("nodes are invalid")
		}
		nodeIDs := map[string]int64{}
		priorNodeIDs := map[string]int64{}
		priorNodesByKey := map[string]types.Object{}
		priorApprovalPolicies := map[string]types.Object{}
		if prior != nil && !prior.Nodes.IsNull() && !prior.Nodes.IsUnknown() {
			var priorNodes []types.Object
			if d := prior.Nodes.ElementsAs(ctx, &priorNodes, false); !d.HasError() {
				for _, priorNode := range priorNodes {
					key, _ := priorNode.Attributes()["key"].(types.String)
					id, _ := priorNode.Attributes()["server_id"].(types.Int64)
					if !key.IsNull() && !key.IsUnknown() && id.ValueInt64() > 0 {
						priorNodeIDs[key.ValueString()] = id.ValueInt64()
						priorNodesByKey[key.ValueString()] = priorNode
					}
					if policy, ok := priorNode.Attributes()["approval_role_policy"].(types.Object); ok && !policy.IsNull() && !policy.IsUnknown() {
						priorApprovalPolicies[key.ValueString()] = policy
					}
				}
			}
		}
		for i, node := range nodes {
			key, _ := node.Attributes()["key"].(types.String)
			if key.IsNull() || key.IsUnknown() || key.ValueString() == "" {
				return nil, fmt.Errorf("node key is required")
			}
			if _, found := nodeIDs[key.ValueString()]; found {
				return nil, fmt.Errorf("node key is duplicated")
			}
			displayName, _ := node.Attributes()["display_name"].(types.String)
			if !displayName.IsNull() && !displayName.IsUnknown() && displayName.ValueString() != key.ValueString() {
				return nil, fmt.Errorf("node display_name must equal key %q", key.ValueString())
			}
			id, _ := node.Attributes()["server_id"].(types.Int64)
			serverID := id.ValueInt64()
			if serverID <= 0 {
				if priorID, exists := priorNodeIDs[key.ValueString()]; exists {
					serverID = priorID
				} else {
					// Creation uses deterministic client IDs; updates reserve positive
					// IDs for persisted nodes and use negative IDs for additions.
					if prior == nil {
						serverID = int64(i + 1)
					} else {
						serverID = int64(-i - 1)
					}
				}
			}
			nodeIDs[key.ValueString()] = serverID
		}
		out := make([]any, 0, len(nodes))
		for _, node := range nodes {
			key, _ := node.Attributes()["key"].(types.String)
			if priorNode, exists := priorNodesByKey[key.ValueString()]; exists {
				node = workflowMergeUnknownNode(node, priorNode)
			}
			wire, e := workflowWireObject(ctx, node, "key", "server_id")
			if e != nil {
				return nil, e
			}
			item := wire
			key, _ = node.Attributes()["key"].(types.String)
			item["display_name"] = key.ValueString()
			if policy, ok := item["approval_role_policy"].(map[string]any); ok {
				if priorPolicy, exists := priorApprovalPolicies[key.ValueString()]; exists {
					if priorWire, err := workflowWireObject(ctx, priorPolicy); err == nil {
						policy["revision"] = priorWire["revision"]
					}
				}
			}
			item["id"] = nodeIDs[key.ValueString()]
			for _, field := range []string{"artifact_outputs", "artifact_inputs"} {
				if list, ok := item[field].([]any); ok {
					for _, entry := range list {
						v, ok := entry.(map[string]any)
						if !ok {
							return nil, fmt.Errorf("workflow artifact is not an object")
						}
						if field == "artifact_inputs" {
							source, _ := v["source_key"].(string)
							id, ok := nodeIDs[source]
							if !ok {
								return nil, fmt.Errorf("artifact input references unknown node key %q", source)
							}
							v["source_node_id"] = id
							delete(v, "source_key")
						}
						if field == "artifact_outputs" {
							if schema, ok := v["schema"].(map[string]any); ok {
								if err := workflowArtifactSchemaWire(schema); err != nil {
									return nil, err
								}
							}
						}
					}
				}
			}
			out = append(out, item)
		}
		body["nodes"] = out
		edgesValue := m.Edges
		if edgesValue.IsUnknown() && prior != nil {
			edgesValue = prior.Edges
		}
		if !edgesValue.IsNull() && !edgesValue.IsUnknown() {
			var edges []types.Object
			if d := edgesValue.ElementsAs(ctx, &edges, false); d.HasError() {
				return nil, fmt.Errorf("edges are invalid")
			}
			priorEdges := map[string]types.Object{}
			if prior != nil && !prior.Edges.IsNull() && !prior.Edges.IsUnknown() {
				var values []types.Object
				if d := prior.Edges.ElementsAs(ctx, &values, false); !d.HasError() {
					for _, value := range values {
						a := value.Attributes()
						source, _ := a["source_key"].(types.String)
						dest, _ := a["destination_key"].(types.String)
						priorEdges[source.ValueString()+"\x00"+dest.ValueString()] = value
					}
				}
			}
			outEdges := make([]any, 0, len(edges))
			for i, edge := range edges {
				a := edge.Attributes()
				sourceKey, _ := a["source_key"].(types.String)
				destKey, _ := a["destination_key"].(types.String)
				if previous, exists := priorEdges[sourceKey.ValueString()+"\x00"+destKey.ValueString()]; exists {
					edge = workflowMergeUnknownNode(edge, previous)
				}
				wire, e := workflowWireObject(ctx, edge, "server_id")
				if e != nil {
					return nil, e
				}
				item := wire
				source, _ := item["source_key"].(string)
				dest, _ := item["destination_key"].(string)
				sourceID, ok := nodeIDs[source]
				if !ok {
					return nil, fmt.Errorf("edge references unknown source node key %q", source)
				}
				destID, ok := nodeIDs[dest]
				if !ok {
					return nil, fmt.Errorf("edge references unknown destination node key %q", dest)
				}
				item["source_node_id"] = sourceID
				item["destination_node_id"] = destID
				if server, _ := edge.Attributes()["server_id"].(types.Int64); server.ValueInt64() > 0 {
					item["id"] = server.ValueInt64()
				} else {
					item["id"] = int64(-i - 1)
				}
				delete(item, "source_key")
				delete(item, "destination_key")
				outEdges = append(outEdges, item)
			}
			body["edges"] = outEdges
		}
	}
	return body, nil
}

func workflowMergeUnknownNode(plan, prior types.Object) types.Object {
	attributes := plan.Attributes()
	for name, value := range attributes {
		if value.IsUnknown() {
			attributes[name] = prior.Attributes()[name]
		}
	}
	result, diagnostics := types.ObjectValue(plan.AttributeTypes(context.Background()), attributes)
	if diagnostics.HasError() {
		return plan
	}
	return result
}
func workflowArtifactSchemaWire(schema map[string]any) error {
	if properties, ok := schema["properties"].([]any); ok {
		result := map[string]any{}
		for _, entry := range properties {
			property, ok := entry.(map[string]any)
			if !ok {
				return fmt.Errorf("artifact schema property is invalid")
			}
			name, _ := property["name"].(string)
			nested, ok := property["schema"].(map[string]any)
			if name == "" || !ok {
				return fmt.Errorf("artifact schema property requires name and schema")
			}
			if err := workflowArtifactSchemaWire(nested); err != nil {
				return err
			}
			if _, exists := result[name]; exists {
				return fmt.Errorf("artifact schema property %q is duplicated", name)
			}
			result[name] = nested
		}
		schema["properties"] = result
	}
	if items, ok := schema["items"].(map[string]any); ok {
		return workflowArtifactSchemaWire(items)
	}
	return nil
}
func workflowRead(ctx context.Context, c *apiclient.SemaphoreUI, m exWorkflowDefinitionModel) (exWorkflowDefinitionModel, error) {
	var raw map[string]any
	if err := exRequest(ctx, c, http.MethodGet, "/project/{project_id}/workflows/{workflow_id}", workflowRoute(m), nil, &raw); err != nil {
		return m, err
	}
	return workflowState(ctx, m, raw)
}
func workflowState(ctx context.Context, old exWorkflowDefinitionModel, raw map[string]any) (exWorkflowDefinitionModel, error) {
	next, err := workflowDecodedResource(ctx, raw, &old)
	if err != nil {
		return next, err
	}
	if next.Parameters.IsNull() && workflowListIsExplicitlyEmpty(old.Parameters) {
		next.Parameters = old.Parameters
	}
	if next.Edges.IsNull() && workflowListIsExplicitlyEmpty(old.Edges) {
		next.Edges = old.Edges
	}
	if !old.Nodes.IsNull() && !old.Nodes.IsUnknown() {
		ordered, orderErr := workflowOrderNodes(ctx, next.Nodes, old.Nodes)
		if orderErr != nil {
			return next, orderErr
		}
		next.Nodes = ordered
	}
	return next, nil
}

func workflowListIsExplicitlyEmpty(value types.List) bool {
	return !value.IsNull() && !value.IsUnknown() && len(value.Elements()) == 0
}

func workflowOrderNodes(ctx context.Context, actual, desired types.List) (types.List, error) {
	var actualNodes, desiredNodes []types.Object
	if diagnostics := actual.ElementsAs(ctx, &actualNodes, false); diagnostics.HasError() {
		return actual, fmt.Errorf("workflow API returned invalid nodes")
	}
	if diagnostics := desired.ElementsAs(ctx, &desiredNodes, false); diagnostics.HasError() {
		return actual, fmt.Errorf("workflow plan contains invalid nodes")
	}
	byKey := map[string]types.Object{}
	for _, node := range actualNodes {
		key, _ := node.Attributes()["key"].(types.String)
		byKey[key.ValueString()] = node
	}
	values := make([]attr.Value, 0, len(actualNodes))
	for _, desiredNode := range desiredNodes {
		key, _ := desiredNode.Attributes()["key"].(types.String)
		if node, exists := byKey[key.ValueString()]; exists {
			values = append(values, node)
			delete(byKey, key.ValueString())
		}
	}
	for _, node := range actualNodes {
		key, _ := node.Attributes()["key"].(types.String)
		if _, exists := byKey[key.ValueString()]; exists {
			values = append(values, node)
		}
	}
	value, diagnostics := types.ListValue(actual.ElementType(ctx), values)
	if diagnostics.HasError() {
		return actual, fmt.Errorf("workflow node state is invalid")
	}
	return value, nil
}
func (r *exWorkflowDefinitionResource) Create(ctx context.Context, q resource.CreateRequest, p *resource.CreateResponse) {
	var m exWorkflowDefinitionModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &m)...)
	if p.Diagnostics.HasError() {
		return
	}
	body, e := workflowDefinitionBody(ctx, m, nil)
	if e != nil {
		p.Diagnostics.AddError("Invalid Workflow Definition", e.Error())
		return
	}
	var raw map[string]any
	e = exRequest(ctx, r.client, http.MethodPost, "/project/{project_id}/workflows", map[string]string{"project_id": strconv.FormatInt(m.ProjectID.ValueInt64(), 10)}, body, &raw)
	if e != nil {
		p.Diagnostics.AddError("Error Creating Workflow Definition", e.Error())
		return
	}
	next, e := workflowState(ctx, m, raw)
	if e != nil {
		p.Diagnostics.AddError("Invalid Workflow Definition Response", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (r *exWorkflowDefinitionResource) Read(ctx context.Context, q resource.ReadRequest, p *resource.ReadResponse) {
	var m exWorkflowDefinitionModel
	p.Diagnostics.Append(q.State.Get(ctx, &m)...)
	if p.Diagnostics.HasError() {
		return
	}
	next, e := workflowRead(ctx, r.client, m)
	if exNotFound(e) {
		p.State.RemoveResource(ctx)
		return
	}
	if e != nil {
		p.Diagnostics.AddError("Error Reading Workflow Definition", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (r *exWorkflowDefinitionResource) Update(ctx context.Context, q resource.UpdateRequest, p *resource.UpdateResponse) {
	var m, old exWorkflowDefinitionModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &m)...)
	p.Diagnostics.Append(q.State.Get(ctx, &old)...)
	if p.Diagnostics.HasError() {
		return
	}
	m.ID = old.ID
	body, e := workflowDefinitionBody(ctx, m, &old)
	if e != nil {
		p.Diagnostics.AddError("Invalid Workflow Definition", e.Error())
		return
	}
	var raw map[string]any
	e = exRequest(ctx, r.client, http.MethodPut, "/project/{project_id}/workflows/{workflow_id}", workflowRoute(m), body, &raw)
	if e != nil {
		p.Diagnostics.AddError("Error Updating Workflow Definition", e.Error())
		return
	}
	next, e := workflowState(ctx, m, raw)
	if e != nil {
		p.Diagnostics.AddError("Invalid Workflow Definition Response", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (r *exWorkflowDefinitionResource) Delete(ctx context.Context, q resource.DeleteRequest, p *resource.DeleteResponse) {
	var m exWorkflowDefinitionModel
	p.Diagnostics.Append(q.State.Get(ctx, &m)...)
	if p.Diagnostics.HasError() {
		return
	}
	e := exRequest(ctx, r.client, http.MethodDelete, "/project/{project_id}/workflows/{workflow_id}", workflowRoute(m), nil, nil)
	if e != nil && !exNotFound(e) {
		p.Diagnostics.AddError("Error Deleting Workflow Definition", e.Error())
	}
}
func (r *exWorkflowDefinitionResource) ImportState(ctx context.Context, q resource.ImportStateRequest, p *resource.ImportStateResponse) {
	parts := strings.Split(q.ID, "/")
	if len(parts) != 4 || parts[0] != "project" || parts[2] != "workflow" {
		p.Diagnostics.AddError("Invalid Import ID", "Expected project/<id>/workflow/<id>.")
		return
	}
	projectID, e := strconv.ParseInt(parts[1], 10, 64)
	if e != nil || projectID < 1 {
		p.Diagnostics.AddError("Invalid Import ID", "Project ID must be positive.")
		return
	}
	id, e := strconv.ParseInt(parts[3], 10, 64)
	if e != nil || id < 1 {
		p.Diagnostics.AddError("Invalid Import ID", "Workflow ID must be positive.")
		return
	}
	objectType, _ := workflowResourceSchema().Type().(types.ObjectType)
	nodesType, _ := objectType.AttrTypes["nodes"].(types.ListType)
	edgesType, _ := objectType.AttrTypes["edges"].(types.ListType)
	parametersType, _ := objectType.AttrTypes["parameters"].(types.ListType)
	accessPolicyType, _ := objectType.AttrTypes["access_policy"].(types.ObjectType)
	m := exWorkflowDefinitionModel{ProjectID: types.Int64Value(projectID), ID: types.Int64Value(id), Nodes: types.ListNull(nodesType.ElemType), Edges: types.ListNull(edgesType.ElemType), Parameters: types.ListNull(parametersType.ElemType), AccessPolicy: types.ObjectNull(accessPolicyType.AttrTypes)}
	next, e := workflowRead(ctx, r.client, m)
	if e != nil {
		p.Diagnostics.AddError("Error Importing Workflow Definition", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (d *exWorkflowDefinitionDataSource) Read(ctx context.Context, q datasource.ReadRequest, p *datasource.ReadResponse) {
	var m exWorkflowDefinitionDataSourceModel
	p.Diagnostics.Append(q.Config.Get(ctx, &m)...)
	if p.Diagnostics.HasError() {
		return
	}
	var raw map[string]any
	e := exRequest(ctx, d.client, http.MethodGet, "/project/{project_id}/workflows/{workflow_id}", map[string]string{"project_id": strconv.FormatInt(m.ProjectID.ValueInt64(), 10), "workflow_id": strconv.FormatInt(m.ID.ValueInt64(), 10)}, nil, &raw)
	if e != nil {
		p.Diagnostics.AddError("Error Reading Workflow Definition", e.Error())
		return
	}
	next, err := workflowDecodedDefinition(ctx, raw)
	if err != nil {
		p.Diagnostics.AddError("Invalid Workflow Definition Response", err.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}

var _ resource.ResourceWithImportState = (*exWorkflowDefinitionResource)(nil)
