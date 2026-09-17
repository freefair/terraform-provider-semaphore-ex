package provider

import (
	"context"

	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	schemaD "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	schemaR "github.com/hashicorp/terraform-plugin-framework/resource/schema"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	superschema "github.com/orange-cloudavenue/terraform-plugin-framework-superschema"
)

type ProjectTemplateJWTModel struct {
	Enabled  types.Bool   `tfsdk:"enabled"`
	Audience types.List   `tfsdk:"audience"`
	TTL      types.String `tfsdk:"ttl"`
}

func templateJWTAttribute() superschema.SingleNestedAttribute {
	return superschema.SingleNestedAttribute{
		Common: &schemaR.SingleNestedAttribute{
			MarkdownDescription: "Controls task JWT issuance. JWT material is never stored in Terraform state.",
		},
		Resource: &schemaR.SingleNestedAttribute{
			Optional: true,
			Computed: true,
			PlanModifiers: []planmodifier.Object{
				preserveOptionalObject{},
			},
		},
		DataSource: &schemaD.SingleNestedAttribute{Computed: true},
		Attributes: map[string]superschema.Attribute{
			"enabled": superschema.BoolAttribute{
				Common:     &schemaR.BoolAttribute{MarkdownDescription: "Enables JWT issuance for this template."},
				Resource:   &schemaR.BoolAttribute{Optional: true, Computed: true},
				DataSource: &schemaD.BoolAttribute{Computed: true},
			},
			"audience": superschema.ListAttribute{
				Common:     &schemaR.ListAttribute{MarkdownDescription: "JWT audiences accepted by task consumers.", ElementType: types.StringType},
				Resource:   &schemaR.ListAttribute{Optional: true, Computed: true},
				DataSource: &schemaD.ListAttribute{Computed: true},
			},
			"ttl": superschema.StringAttribute{
				Common:     &schemaR.StringAttribute{MarkdownDescription: "Positive Go duration for issued JWTs; server limits remain authoritative."},
				Resource:   &schemaR.StringAttribute{Optional: true, Computed: true},
				DataSource: &schemaD.StringAttribute{Computed: true},
			},
		},
	}
}

func templateJWTToAPI(ctx context.Context, value *ProjectTemplateJWTModel) *models.TemplateJWTParams {
	if value == nil {
		return nil
	}
	params := &models.TemplateJWTParams{Enabled: value.Enabled.ValueBool(), TTL: value.TTL.ValueString()}
	if !value.Audience.IsNull() && !value.Audience.IsUnknown() {
		var audiences []string
		_ = value.Audience.ElementsAs(ctx, &audiences, false)
		params.Audience = audiences
	}
	return params
}

func templateJWTFromAPI(ctx context.Context, value *models.TemplateJWTParams) *ProjectTemplateJWTModel {
	if value == nil {
		return nil
	}
	audiences := []string{}
	switch raw := value.Audience.(type) {
	case string:
		audiences = []string{raw}
	case []string:
		audiences = raw
	case []any:
		for _, item := range raw {
			if text, ok := item.(string); ok {
				audiences = append(audiences, text)
			}
		}
	}
	audience, diagnostics := types.ListValueFrom(ctx, types.StringType, audiences)
	if diagnostics.HasError() {
		audience = types.ListValueMust(types.StringType, []attr.Value{})
	}
	return &ProjectTemplateJWTModel{Enabled: types.BoolValue(value.Enabled), Audience: audience, TTL: types.StringValue(value.TTL)}
}
