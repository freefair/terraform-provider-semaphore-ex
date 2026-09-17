package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type projectIntegrationExtractValueResource struct{ exRecordResource }

var _ resource.ResourceWithValidateConfig = &projectIntegrationExtractValueResource{}

func NewProjectIntegrationExtractValueResource() resource.Resource {
	return &projectIntegrationExtractValueResource{exRecordResource: exRecordResource{spec: projectIntegrationExtractValueSpec()}}
}

func (r *projectIntegrationExtractValueResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var source, bodyType, key types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("value_source"), &source)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("body_data_type"), &bodyType)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("key"), &key)...)
	if resp.Diagnostics.HasError() || source.IsUnknown() || bodyType.IsUnknown() || key.IsUnknown() {
		return
	}
	if (source.ValueString() == "header" || (source.ValueString() == "body" && bodyType.ValueString() != "string")) && key.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(path.Root("key"), "Missing Extraction Key", "Set key when extracting a header or JSON body field.")
	}
}
