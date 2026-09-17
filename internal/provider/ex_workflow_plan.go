package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	rs "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ModifyPlan preserves omitted optional/computed settings by graph identity.
// Collection indices cannot identify nodes after a reorder or insertion.
func (r *exWorkflowDefinitionResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var planned, configured, prior types.Object
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planned)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &configured)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	attributes := workflowResourceSchema().Attributes
	planned = workflowPreservePlanObject(planned, configured, prior, attributes, false)
	// Revisions remain unknown for real writes. If only computed outputs differ,
	// there is no mutation and keeping current outputs avoids a perpetual plan.
	unchanged := workflowPreservePlanObject(planned, configured, prior, attributes, true)
	if unchanged.Equal(prior) {
		planned = prior
	}
	resp.Diagnostics.Append(resp.Plan.Set(ctx, planned)...)
}

func workflowPreservePlanObject(planned, configured, prior types.Object, attributes map[string]rs.Attribute, compareOutputs bool) types.Object {
	if planned.IsNull() || planned.IsUnknown() || configured.IsNull() || configured.IsUnknown() || prior.IsNull() || prior.IsUnknown() {
		return planned
	}
	values, configValues, oldValues := planned.Attributes(), configured.Attributes(), prior.Attributes()
	for name, schema := range attributes {
		value, exists := values[name]
		if !exists {
			continue
		}
		configValue, oldValue := configValues[name], oldValues[name]
		if configValue == nil || oldValue == nil {
			continue
		}
		if (value.IsUnknown() || value.IsNull()) && configValue.IsNull() && schema.IsComputed() && (schema.IsOptional() || name == "id" || name == "server_id" || compareOutputs) {
			values[name] = oldValue
			continue
		}
		switch nested := schema.(type) {
		case rs.SingleNestedAttribute:
			p, pok := value.(types.Object)
			c, cok := configValue.(types.Object)
			s, sok := oldValue.(types.Object)
			if pok && cok && sok {
				values[name] = workflowPreservePlanObject(p, c, s, nested.Attributes, compareOutputs)
			}
		case rs.ListNestedAttribute:
			p, pok := value.(types.List)
			c, cok := configValue.(types.List)
			s, sok := oldValue.(types.List)
			if !pok || !cok || !sok || p.IsNull() || p.IsUnknown() || c.IsNull() || c.IsUnknown() || s.IsNull() || s.IsUnknown() {
				continue
			}
			oldByKey := map[string]types.Object{}
			for _, element := range s.Elements() {
				if object, ok := element.(types.Object); ok {
					if key := workflowPlanIdentity(name, object); key != "" {
						oldByKey[key] = object
					}
				}
			}
			elements, configs := p.Elements(), c.Elements()
			for index, element := range elements {
				object, ok := element.(types.Object)
				if !ok || index >= len(configs) {
					continue
				}
				configObject, ok := configs[index].(types.Object)
				previous, exists := oldByKey[workflowPlanIdentity(name, object)]
				if ok && exists {
					elements[index] = workflowPreservePlanObject(object, configObject, previous, nested.NestedObject.Attributes, compareOutputs)
				}
			}
			if list, diagnostics := types.ListValue(p.ElementType(context.Background()), elements); !diagnostics.HasError() {
				values[name] = list
			}
		}
	}
	result, diagnostics := types.ObjectValue(planned.AttributeTypes(context.Background()), values)
	if diagnostics.HasError() {
		return planned
	}
	return result
}

func workflowPlanIdentity(collection string, object types.Object) string {
	if object.IsNull() || object.IsUnknown() {
		return ""
	}
	values := object.Attributes()
	field := "name"
	if collection == "nodes" {
		field = "key"
	}
	if collection == "edges" {
		source, sourceOK := values["source_key"].(types.String)
		destination, destinationOK := values["destination_key"].(types.String)
		if sourceOK && destinationOK && !source.IsUnknown() && !destination.IsUnknown() && !source.IsNull() && !destination.IsNull() {
			return source.ValueString() + "\x00" + destination.ValueString()
		}
		return ""
	}
	if value, ok := values[field].(types.String); ok && !value.IsNull() && !value.IsUnknown() {
		return value.ValueString()
	}
	return ""
}

var _ resource.ResourceWithModifyPlan = (*exWorkflowDefinitionResource)(nil)
