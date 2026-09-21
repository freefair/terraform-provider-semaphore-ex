package provider

import (
	"context"
	"sort"

	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func surveyChoiceType() types.ObjectType {
	return types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "value": types.StringType}}
}
func surveyMapFromChoices(choices types.List) types.Map {
	if choices.IsNull() {
		return types.MapNull(types.StringType)
	}
	if choices.IsUnknown() {
		return types.MapUnknown(types.StringType)
	}
	values := map[string]attr.Value{}
	for _, element := range choices.Elements() {
		object, ok := element.(types.Object)
		if !ok || object.IsNull() {
			return types.MapNull(types.StringType)
		}
		if object.IsUnknown() {
			return types.MapUnknown(types.StringType)
		}
		name, nameOK := object.Attributes()["name"].(types.String)
		value, valueOK := object.Attributes()["value"].(types.String)
		if !nameOK || !valueOK || name.IsNull() || value.IsNull() {
			return types.MapNull(types.StringType)
		}
		if name.IsUnknown() || value.IsUnknown() {
			return types.MapUnknown(types.StringType)
		}
		if _, duplicate := values[name.ValueString()]; duplicate {
			return types.MapNull(types.StringType)
		}
		values[name.ValueString()] = value
	}
	return types.MapValueMust(types.StringType, values)
}
func surveyChoicesFromMap(values types.Map, previous types.List) types.List {
	if values.IsNull() {
		return types.ListNull(surveyChoiceType())
	}
	if values.IsUnknown() {
		return types.ListUnknown(surveyChoiceType())
	}
	if !previous.IsNull() && !previous.IsUnknown() && surveyMapFromChoices(previous).Equal(values) {
		return previous
	}
	names := make([]string, 0, len(values.Elements()))
	for name, value := range values.Elements() {
		if value.IsUnknown() {
			return types.ListUnknown(surveyChoiceType())
		}
		names = append(names, name)
	}
	sort.Strings(names)
	choices := make([]attr.Value, 0, len(names))
	for _, name := range names {
		choices = append(choices, types.ObjectValueMust(surveyChoiceType().AttrTypes, map[string]attr.Value{"name": types.StringValue(name), "value": values.Elements()[name]}))
	}
	return types.ListValueMust(surveyChoiceType(), choices)
}
func surveyChoicesFromAPI(values []*models.TemplateSurveyVarValue) types.List {
	choices := make([]attr.Value, 0, len(values))
	for _, value := range values {
		if value == nil {
			choices = append(choices, types.ObjectNull(surveyChoiceType().AttrTypes))
			continue
		}
		choices = append(choices, types.ObjectValueMust(surveyChoiceType().AttrTypes, map[string]attr.Value{"name": types.StringValue(value.Name), "value": types.StringValue(value.Value)}))
	}
	return types.ListValueMust(surveyChoiceType(), choices)
}
func surveyChoicesToAPI(model ProjectTemplateSurveyVarModel) []*models.TemplateSurveyVarValue {
	choices := model.Choices
	if choices.IsNull() || choices.IsUnknown() {
		choices = surveyChoicesFromMap(model.EnumValues, types.ListNull(surveyChoiceType()))
	}
	result := make([]*models.TemplateSurveyVarValue, 0, len(choices.Elements()))
	for _, element := range choices.Elements() {
		object, ok := element.(types.Object)
		if !ok || object.IsNull() {
			result = append(result, nil)
			continue
		}
		name, _ := object.Attributes()["name"].(types.String)
		value, _ := object.Attributes()["value"].(types.String)
		result = append(result, &models.TemplateSurveyVarValue{Name: name.ValueString(), Value: value.ValueString()})
	}
	return result
}

func planSurveyChoices(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var configured, planned, prior types.List
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("survey_vars"), &configured)...)
	resp.Diagnostics.Append(resp.Plan.GetAttribute(ctx, path.Root("survey_vars"), &planned)...)
	if !req.State.Raw.IsNull() {
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("survey_vars"), &prior)...)
	}
	if resp.Diagnostics.HasError() || configured.IsNull() || configured.IsUnknown() || planned.IsNull() || planned.IsUnknown() {
		return
	}
	previousByName := map[string]types.Object{}
	for _, value := range prior.Elements() {
		if object, ok := value.(types.Object); ok {
			if name, ok := object.Attributes()["name"].(types.String); ok {
				previousByName[name.ValueString()] = object
			}
		}
	}
	elements := planned.Elements()
	for index, value := range configured.Elements() {
		config, ok := value.(types.Object)
		if !ok || config.IsUnknown() || config.IsNull() || index >= len(elements) {
			continue
		}
		plan, ok := elements[index].(types.Object)
		if !ok || plan.IsUnknown() || plan.IsNull() {
			continue
		}
		name, _ := config.Attributes()["name"].(types.String)
		kind, _ := config.Attributes()["type"].(types.String)
		enum, _ := config.Attributes()["enum_values"].(types.Map)
		choices, _ := config.Attributes()["choices"].(types.List)
		oldChoices := types.ListNull(surveyChoiceType())
		oldEnum := types.MapNull(types.StringType)
		if old, exists := previousByName[name.ValueString()]; exists {
			oldChoices, _ = old.Attributes()["choices"].(types.List)
			oldEnum, _ = old.Attributes()["enum_values"].(types.Map)
		}
		attributes := plan.Attributes()
		switch {
		case !choices.IsNull():
			attributes["choices"] = choices
			attributes["enum_values"] = surveyMapFromChoices(choices)
		case !enum.IsNull():
			attributes["enum_values"] = enum
			attributes["choices"] = surveyChoicesFromMap(enum, oldChoices)
		case kind.IsUnknown():
			attributes["choices"] = types.ListUnknown(surveyChoiceType())
			attributes["enum_values"] = types.MapUnknown(types.StringType)
		case kind.ValueString() == "enum" || kind.ValueString() == "select":
			if oldChoices.IsNull() {
				oldChoices = types.ListValueMust(surveyChoiceType(), []attr.Value{})
				oldEnum = types.MapValueMust(types.StringType, map[string]attr.Value{})
			}
			attributes["choices"] = oldChoices
			attributes["enum_values"] = oldEnum
		default:
			attributes["choices"] = types.ListNull(surveyChoiceType())
			attributes["enum_values"] = types.MapNull(types.StringType)
		}
		updated, diags := types.ObjectValue(plan.AttributeTypes(ctx), attributes)
		resp.Diagnostics.Append(diags...)
		elements[index] = updated
	}
	result, diags := types.ListValue(planned.ElementType(ctx), elements)
	resp.Diagnostics.Append(diags...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("survey_vars"), result)...)
	}
}

type surveyChoicesValidator struct{}

func (surveyChoicesValidator) Description(context.Context) string {
	return "Survey choices require enum/select type and non-null names and values."
}
func (v surveyChoicesValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (surveyChoicesValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var survey types.List
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("survey_vars"), &survey)...)
	if resp.Diagnostics.HasError() || survey.IsNull() || survey.IsUnknown() {
		return
	}
	for index, element := range survey.Elements() {
		object, ok := element.(types.Object)
		if !ok || object.IsNull() || object.IsUnknown() {
			continue
		}
		choices, _ := object.Attributes()["choices"].(types.List)
		enum, _ := object.Attributes()["enum_values"].(types.Map)
		kind, _ := object.Attributes()["type"].(types.String)
		itemPath := path.Root("survey_vars").AtListIndex(index)
		if (!choices.IsNull() || !enum.IsNull()) && !kind.IsUnknown() && kind.ValueString() != "enum" && kind.ValueString() != "select" {
			resp.Diagnostics.AddAttributeError(itemPath, "Invalid Survey Options", "choices and enum_values require type = enum or select.")
		}
		for _, entry := range choices.Elements() {
			choice, ok := entry.(types.Object)
			if !ok || choice.IsNull() {
				resp.Diagnostics.AddAttributeError(itemPath.AtName("choices"), "Null Survey Choice", "Each choice must contain a name and value.")
			}
		}
		for _, value := range enum.Elements() {
			if value.IsNull() {
				resp.Diagnostics.AddAttributeError(itemPath.AtName("enum_values"), "Null Survey Value", "Enum values must be strings; use an empty string explicitly if intended.")
			}
		}
	}
}
