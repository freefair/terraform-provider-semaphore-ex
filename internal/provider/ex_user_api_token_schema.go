package provider

import (
	schemaD "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	schemaR "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type exUserAPITokenModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	ExpiresAt  types.String `tfsdk:"expires_at"`
	Keepers    types.Map    `tfsdk:"keepers"`
	Credential types.String `tfsdk:"credential"`
	Created    types.String `tfsdk:"created"`
	Expired    types.Bool   `tfsdk:"expired"`
	UserID     types.Int64  `tfsdk:"user_id"`
}

type exUserAPITokenDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	TokenID   types.String `tfsdk:"token_id"`
	Name      types.String `tfsdk:"name"`
	ExpiresAt types.String `tfsdk:"expires_at"`
	Created   types.String `tfsdk:"created"`
	Expired   types.Bool   `tfsdk:"expired"`
	UserID    types.Int64  `tfsdk:"user_id"`
}

func exUserAPITokenResourceSchema() schemaR.Schema {
	return schemaR.Schema{MarkdownDescription: "Manages an immutable Semaphore EX API token. The credential is returned once at creation and stored as sensitive Terraform state. Use `keepers` or Terraform `-replace` to rotate a token.", Attributes: map[string]schemaR.Attribute{
		"id":         schemaR.StringAttribute{MarkdownDescription: "Stable, non-secret API token identifier returned by Semaphore EX.", Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"name":       schemaR.StringAttribute{MarkdownDescription: "Human-readable name for the API token.", Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"expires_at": schemaR.StringAttribute{MarkdownDescription: "Optional RFC 3339 time at which the API token expires.", Optional: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"keepers":    schemaR.MapAttribute{MarkdownDescription: "Arbitrary values that force replacement when changed; use this to rotate the immutable token.", Optional: true, ElementType: types.StringType, PlanModifiers: []planmodifier.Map{mapplanmodifier.RequiresReplace()}},
		"credential": schemaR.StringAttribute{MarkdownDescription: "API credential returned once when the token is created. It is never read from Semaphore EX again.", Computed: true, Sensitive: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"created":    schemaR.StringAttribute{MarkdownDescription: "Creation time reported by Semaphore EX.", Computed: true},
		"expired":    schemaR.BoolAttribute{MarkdownDescription: "Whether Semaphore EX reports the token as expired.", Computed: true},
		"user_id":    schemaR.Int64Attribute{MarkdownDescription: "Owner user ID reported by Semaphore EX.", Computed: true},
	}}
}

func exUserAPITokenDataSourceSchema() schemaD.Schema {
	return schemaD.Schema{MarkdownDescription: "Reads non-secret metadata for an API token owned by the configured Semaphore EX user.", Attributes: map[string]schemaD.Attribute{
		"id":         schemaD.StringAttribute{MarkdownDescription: "Stable, non-secret API token identifier.", Computed: true},
		"token_id":   schemaD.StringAttribute{MarkdownDescription: "Stable, non-secret API token identifier to read.", Required: true},
		"name":       schemaD.StringAttribute{MarkdownDescription: "Human-readable API token name.", Computed: true},
		"expires_at": schemaD.StringAttribute{MarkdownDescription: "Token expiry time, when configured.", Computed: true},
		"created":    schemaD.StringAttribute{MarkdownDescription: "Creation time reported by Semaphore EX.", Computed: true},
		"expired":    schemaD.BoolAttribute{MarkdownDescription: "Whether Semaphore EX reports the token as expired.", Computed: true},
		"user_id":    schemaD.Int64Attribute{MarkdownDescription: "Owner user ID reported by Semaphore EX.", Computed: true},
	}}
}
