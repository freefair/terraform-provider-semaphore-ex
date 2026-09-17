package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func surveyTypeToAPI(value string) string {
	switch value {
	case "string":
		return ""
	case "integer":
		return "int"
	default:
		return value
	}
}

func surveyTypeFromAPI(value string) string {
	switch value {
	case "":
		return "string"
	case "int":
		return "integer"
	default:
		return value
	}
}

func surveyDefaultToAPI(ctx context.Context, value ProjectTemplateSurveyVarModel) any {
	if !value.DefaultValues.IsNull() && !value.DefaultValues.IsUnknown() {
		values := []string{}
		value.DefaultValues.ElementsAs(ctx, &values, false)
		return values
	}
	if !value.DefaultValue.IsNull() && !value.DefaultValue.IsUnknown() {
		return value.DefaultValue.ValueString()
	}
	return nil
}

func readSurveyDefault(ctx context.Context, value any, model *ProjectTemplateSurveyVarModel) {
	switch v := value.(type) {
	case string:
		model.DefaultValue = types.StringValue(v)
	case []any:
		model.DefaultValues, _ = types.ListValueFrom(ctx, types.StringType, v)
	case []string:
		model.DefaultValues, _ = types.ListValueFrom(ctx, types.StringType, v)
	}
}
