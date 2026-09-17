package provider

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rs "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"regexp"
)

type identityTargetModel struct {
	Scope     types.String `tfsdk:"scope"`
	ProjectID types.Int64  `tfsdk:"project_id"`
	RoleID    types.String `tfsdk:"role_id"`
}
type identityMappingModel struct {
	ID              types.String `tfsdk:"id"`
	ProviderID      types.String `tfsdk:"provider_id"`
	GroupExternalID types.String `tfsdk:"group_external_id"`
	ClaimValue      types.String `tfsdk:"claim_value"`
	Target          types.Object `tfsdk:"target"`
	Enabled         types.Bool   `tfsdk:"enabled"`
	Revision        types.Int64  `tfsdk:"revision"`
}
type totpPolicyModel struct {
	ID              types.String `tfsdk:"id"`
	State           types.String `tfsdk:"state"`
	SelectedUserIDs types.List   `tfsdk:"selected_user_ids"`
}

var identityTargetTypes = map[string]attr.Type{"scope": types.StringType, "project_id": types.Int64Type, "role_id": types.StringType}

func identityTargetResourceAttribute() rs.SingleNestedAttribute {
	return rs.SingleNestedAttribute{Required: true, Attributes: map[string]rs.Attribute{
		"scope": rs.StringAttribute{Required: true}, "project_id": rs.Int64Attribute{Optional: true}, "role_id": rs.StringAttribute{Required: true},
	}}
}
func identityTargetDataSourceAttribute() ds.SingleNestedAttribute {
	return ds.SingleNestedAttribute{Computed: true, Attributes: map[string]ds.Attribute{
		"scope": ds.StringAttribute{Computed: true}, "project_id": ds.Int64Attribute{Computed: true}, "role_id": ds.StringAttribute{Computed: true},
	}}
}

func identityMappingResourceSchema(description string, oidc bool) rs.Schema {
	attributes := map[string]rs.Attribute{
		"id":          rs.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[^A-Z]+$`), "must be lowercase because the server canonicalizes mapping IDs")}},
		"provider_id": rs.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[^A-Z]+$`), "must be lowercase because the server canonicalizes provider IDs")}},
		"target":      identityTargetResourceAttribute(),
		"enabled":     rs.BoolAttribute{Required: true},
		"revision":    rs.Int64Attribute{Computed: true},
	}
	if oidc {
		attributes["group_external_id"] = rs.StringAttribute{MarkdownDescription: "Not applicable to OIDC mappings.", Computed: true}
		attributes["claim_value"] = rs.StringAttribute{MarkdownDescription: "OIDC group claim value to map without leading or trailing whitespace.", Required: true, Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^(\S.*\S|\S)?$`), "must not have leading or trailing whitespace")}}
	} else {
		attributes["group_external_id"] = rs.StringAttribute{MarkdownDescription: "Lowercase LDAP group external identifier to map.", Required: true, Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[^A-Z]+$`), "must be lowercase because the server canonicalizes LDAP identifiers")}}
		attributes["claim_value"] = rs.StringAttribute{MarkdownDescription: "Not applicable to LDAP mappings.", Computed: true}
	}
	return rs.Schema{MarkdownDescription: description, Attributes: attributes}
}
func identityMappingDataSourceSchema(description string) ds.Schema {
	return ds.Schema{MarkdownDescription: description, Attributes: map[string]ds.Attribute{
		"id": ds.StringAttribute{Required: true}, "provider_id": ds.StringAttribute{Required: true}, "group_external_id": ds.StringAttribute{Computed: true}, "claim_value": ds.StringAttribute{Computed: true}, "target": identityTargetDataSourceAttribute(), "enabled": ds.BoolAttribute{Computed: true}, "revision": ds.Int64Attribute{Computed: true},
	}}
}
func totpPolicyResourceSchema() rs.Schema {
	return rs.Schema{MarkdownDescription: "Configures the global TOTP enrollment policy.", Attributes: map[string]rs.Attribute{
		"id": rs.StringAttribute{Computed: true}, "state": rs.StringAttribute{Required: true}, "selected_user_ids": rs.ListAttribute{Optional: true, Computed: true, ElementType: types.Int64Type, PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}},
	}}
}
func totpPolicyDataSourceSchema() ds.Schema {
	return ds.Schema{MarkdownDescription: "Reads the global TOTP enrollment policy.", Attributes: map[string]ds.Attribute{
		"id": ds.StringAttribute{Computed: true}, "state": ds.StringAttribute{Computed: true}, "selected_user_ids": ds.ListAttribute{Computed: true, ElementType: types.Int64Type},
	}}
}

var _ resource.Resource = (*totpPolicyResource)(nil)
var _ datasource.DataSource = (*totpPolicyDataSource)(nil)
