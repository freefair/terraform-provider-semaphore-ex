package provider

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rs "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type ldapConfigurationModel struct {
	ID                     types.String `tfsdk:"id"`
	DisplayName            types.String `tfsdk:"display_name"`
	State                  types.String `tfsdk:"state"`
	ServerURL              types.String `tfsdk:"server_url"`
	TLSMode                types.String `tfsdk:"tls_mode"`
	TrustMode              types.String `tfsdk:"trust_mode"`
	CAPEM                  types.String `tfsdk:"ca_pem"`
	BindDN                 types.String `tfsdk:"bind_dn"`
	BindPassword           types.String `tfsdk:"bind_password"`
	BindPasswordWOVersion  types.Int64  `tfsdk:"bind_password_wo_version"`
	BindPasswordConfigured types.Bool   `tfsdk:"bind_password_configured"`
	SearchBaseDN           types.String `tfsdk:"search_base_dn"`
	UserFilter             types.String `tfsdk:"user_filter"`
	IdentityAttribute      types.String `tfsdk:"identity_attribute"`
	UsernameAttribute      types.String `tfsdk:"username_attribute"`
	NameAttribute          types.String `tfsdk:"name_attribute"`
	EmailAttribute         types.String `tfsdk:"email_attribute"`
	GroupSearchBaseDN      types.String `tfsdk:"group_search_base_dn"`
	GroupUserFilter        types.String `tfsdk:"group_user_filter"`
	GroupFilter            types.String `tfsdk:"group_filter"`
	GroupIdentityAttribute types.String `tfsdk:"group_identity_attribute"`
	GroupMemberAttribute   types.String `tfsdk:"group_member_attribute"`
	GroupMaxDepth          types.Int64  `tfsdk:"group_max_depth"`
	SelectedUserIDs        types.List   `tfsdk:"selected_user_ids"`
}
type ldapConfigurationResource struct{ client *apiclient.SemaphoreUI }
type ldapConfigurationDataSource struct{ client *apiclient.SemaphoreUI }

var ldapProviderIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

// canonicalLDAPProviderIDValidator prevents the server's normalization from
// creating a perpetual configuration difference after an apply.
type canonicalLDAPProviderIDValidator struct{}

func (canonicalLDAPProviderIDValidator) Description(context.Context) string {
	return "LDAP provider IDs must be lowercase, trimmed, start with a letter, and contain only letters, digits, underscores, or hyphens."
}

func (v canonicalLDAPProviderIDValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v canonicalLDAPProviderIDValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueString()
	if value != strings.ToLower(strings.TrimSpace(value)) || !ldapProviderIDPattern.MatchString(value) {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid LDAP provider ID", v.Description(ctx))
	}
}

func NewLDAPConfigurationResource() resource.Resource       { return &ldapConfigurationResource{} }
func NewLDAPConfigurationDataSource() datasource.DataSource { return &ldapConfigurationDataSource{} }
func (r *ldapConfigurationResource) Metadata(_ context.Context, q resource.MetadataRequest, p *resource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_ldap_configuration"
}
func (d *ldapConfigurationDataSource) Metadata(_ context.Context, q datasource.MetadataRequest, p *datasource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_ldap_configuration"
}
func (r *ldapConfigurationResource) Configure(_ context.Context, q resource.ConfigureRequest, p *resource.ConfigureResponse) {
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
func (d *ldapConfigurationDataSource) Configure(_ context.Context, q datasource.ConfigureRequest, p *datasource.ConfigureResponse) {
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
func ldapResourceSchema() rs.Schema {
	a := map[string]rs.Attribute{}
	required := []string{"id", "display_name", "state", "server_url", "tls_mode", "trust_mode", "bind_dn", "search_base_dn", "user_filter", "identity_attribute", "username_attribute", "name_attribute", "email_attribute"}
	for _, n := range required {
		a[n] = rs.StringAttribute{Required: true}
	}
	a["id"] = rs.StringAttribute{Required: true, Validators: []validator.String{canonicalLDAPProviderIDValidator{}}}
	a["state"] = rs.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("disabled", "shadow", "selected_users", "active")}}
	for _, n := range []string{"ca_pem", "group_search_base_dn", "group_user_filter", "group_filter", "group_identity_attribute", "group_member_attribute"} {
		a[n] = rs.StringAttribute{Optional: true, Computed: true}
	}
	a["bind_password"] = rs.StringAttribute{Optional: true, Sensitive: true, WriteOnly: true}
	a["bind_password_wo_version"] = rs.Int64Attribute{Optional: true, Validators: []validator.Int64{int64validator.AtLeast(1)}}
	a["bind_password_configured"] = rs.BoolAttribute{Computed: true}
	a["group_max_depth"] = rs.Int64Attribute{Optional: true, Computed: true}
	a["selected_user_ids"] = rs.ListAttribute{Optional: true, Computed: true, ElementType: types.Int64Type}
	return rs.Schema{MarkdownDescription: "Configures an LDAP identity provider and its administrator-controlled exposure state.", Attributes: a}
}
func ldapDataSourceSchema() ds.Schema {
	a := map[string]ds.Attribute{"id": ds.StringAttribute{Required: true}}
	for _, n := range []string{"display_name", "state", "server_url", "tls_mode", "trust_mode", "ca_pem", "bind_dn", "search_base_dn", "user_filter", "identity_attribute", "username_attribute", "name_attribute", "email_attribute", "group_search_base_dn", "group_user_filter", "group_filter", "group_identity_attribute", "group_member_attribute"} {
		a[n] = ds.StringAttribute{Computed: true}
	}
	a["bind_password_configured"] = ds.BoolAttribute{Computed: true}
	a["group_max_depth"] = ds.Int64Attribute{Computed: true}
	a["selected_user_ids"] = ds.ListAttribute{Computed: true, ElementType: types.Int64Type}
	return ds.Schema{MarkdownDescription: "Reads an LDAP identity provider configuration without its bind password.", Attributes: a}
}
func (r *ldapConfigurationResource) Schema(_ context.Context, _ resource.SchemaRequest, p *resource.SchemaResponse) {
	p.Schema = ldapResourceSchema()
}
func (d *ldapConfigurationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, p *datasource.SchemaResponse) {
	p.Schema = ldapDataSourceSchema()
}
func ldapPayload(m ldapConfigurationModel) map[string]any {
	return ldapPayloadWithBindPassword(m, true)
}

func ldapPayloadWithBindPassword(m ldapConfigurationModel, includeBindPassword bool) map[string]any {
	out := map[string]any{"id": strings.ToLower(strings.TrimSpace(m.ID.ValueString())), "display_name": m.DisplayName.ValueString(), "server_url": m.ServerURL.ValueString(), "tls_mode": m.TLSMode.ValueString(), "trust_mode": m.TrustMode.ValueString(), "bind_dn": m.BindDN.ValueString(), "search_base_dn": m.SearchBaseDN.ValueString(), "user_filter": m.UserFilter.ValueString(), "identity_attribute": m.IdentityAttribute.ValueString(), "username_attribute": m.UsernameAttribute.ValueString(), "name_attribute": m.NameAttribute.ValueString(), "email_attribute": m.EmailAttribute.ValueString()}
	for n, v := range map[string]types.String{"ca_pem": m.CAPEM, "group_search_base_dn": m.GroupSearchBaseDN, "group_user_filter": m.GroupUserFilter, "group_filter": m.GroupFilter, "group_identity_attribute": m.GroupIdentityAttribute, "group_member_attribute": m.GroupMemberAttribute} {
		if !v.IsNull() {
			if n == "ca_pem" {
				out[n] = strings.TrimSpace(v.ValueString())
			} else {
				out[n] = v.ValueString()
			}
		}
	}
	if !m.GroupMaxDepth.IsNull() {
		out["group_max_depth"] = m.GroupMaxDepth.ValueInt64()
	}
	if includeBindPassword && !m.BindPassword.IsNull() && !m.BindPassword.IsUnknown() {
		out["bind_password"] = m.BindPassword.ValueString()
	}
	return out
}

func ldapMergeUnknownOptionalComputed(plan *ldapConfigurationModel, prior ldapConfigurationModel) {
	for _, values := range []struct{ plan, prior *types.String }{
		{&plan.CAPEM, &prior.CAPEM}, {&plan.GroupSearchBaseDN, &prior.GroupSearchBaseDN},
		{&plan.GroupUserFilter, &prior.GroupUserFilter}, {&plan.GroupFilter, &prior.GroupFilter},
		{&plan.GroupIdentityAttribute, &prior.GroupIdentityAttribute}, {&plan.GroupMemberAttribute, &prior.GroupMemberAttribute},
	} {
		if values.plan.IsUnknown() {
			*values.plan = *values.prior
		}
	}
	if plan.GroupMaxDepth.IsUnknown() {
		plan.GroupMaxDepth = prior.GroupMaxDepth
	}
	if plan.SelectedUserIDs.IsUnknown() {
		plan.SelectedUserIDs = prior.SelectedUserIDs
	}
}

func ldapPreserveConfiguredStrings(next *ldapConfigurationModel, configured ldapConfigurationModel) {
	for _, values := range []struct{ next, configured *types.String }{
		{&next.DisplayName, &configured.DisplayName}, {&next.ServerURL, &configured.ServerURL}, {&next.TLSMode, &configured.TLSMode}, {&next.TrustMode, &configured.TrustMode},
		{&next.CAPEM, &configured.CAPEM}, {&next.BindDN, &configured.BindDN}, {&next.SearchBaseDN, &configured.SearchBaseDN}, {&next.UserFilter, &configured.UserFilter},
		{&next.IdentityAttribute, &configured.IdentityAttribute}, {&next.UsernameAttribute, &configured.UsernameAttribute}, {&next.NameAttribute, &configured.NameAttribute}, {&next.EmailAttribute, &configured.EmailAttribute},
		{&next.GroupSearchBaseDN, &configured.GroupSearchBaseDN}, {&next.GroupUserFilter, &configured.GroupUserFilter}, {&next.GroupFilter, &configured.GroupFilter},
		{&next.GroupIdentityAttribute, &configured.GroupIdentityAttribute}, {&next.GroupMemberAttribute, &configured.GroupMemberAttribute},
	} {
		if !values.configured.IsNull() && !values.configured.IsUnknown() && strings.TrimSpace(values.configured.ValueString()) == strings.TrimSpace(values.next.ValueString()) {
			*values.next = *values.configured
		}
	}
}

func ldapHasConfiguredSelectedUsers(ctx context.Context, config tfsdk.Config) (bool, error) {
	var selected types.List
	if diagnostics := config.GetAttribute(ctx, path.Root("selected_user_ids"), &selected); diagnostics.HasError() {
		return false, fmt.Errorf("could not read selected LDAP users from configuration")
	}
	if selected.IsNull() || selected.IsUnknown() {
		return false, nil
	}
	var ids []int64
	if diagnostics := selected.ElementsAs(ctx, &ids, false); diagnostics.HasError() {
		return false, fmt.Errorf("selected LDAP users are invalid")
	}
	return len(ids) != 0, nil
}

func ldapValidateConfiguredSelection(ctx context.Context, config tfsdk.Config, state types.String) error {
	hasSelectedUsers, err := ldapHasConfiguredSelectedUsers(ctx, config)
	if err != nil || !hasSelectedUsers || state.ValueString() == "selected_users" {
		return err
	}
	return fmt.Errorf("selected_user_ids may be configured only when state is selected_users")
}
func readLDAP(ctx context.Context, c *apiclient.SemaphoreUI, id string) (ldapConfigurationModel, error) {
	var values []map[string]any
	if e := exRequest(ctx, c, http.MethodGet, "/capabilities/ldap", nil, nil, &values); e != nil {
		return ldapConfigurationModel{}, e
	}
	for _, raw := range values {
		if got, _ := raw["id"].(string); strings.EqualFold(got, id) {
			return ldapModel(ctx, raw)
		}
	}
	return ldapConfigurationModel{}, &exAPIError{StatusCode: 404, method: http.MethodGet, route: "/capabilities/ldap"}
}
func ldapModel(ctx context.Context, v map[string]any) (ldapConfigurationModel, error) {
	get := func(k string) types.String { x, _ := v[k].(string); return types.StringValue(x) }
	ids := []int64{}
	if raw, ok := v["selected_user_ids"].([]any); ok {
		for _, item := range raw {
			n, e := identityNumber(item)
			if e != nil {
				return ldapConfigurationModel{}, e
			}
			ids = append(ids, n)
		}
	}
	selected, diags := types.ListValueFrom(ctx, types.Int64Type, ids)
	if diags.HasError() {
		return ldapConfigurationModel{}, fmt.Errorf("invalid LDAP selected user IDs")
	}
	depth := int64(0)
	if v["group_max_depth"] != nil {
		var e error
		depth, e = identityNumber(v["group_max_depth"])
		if e != nil {
			return ldapConfigurationModel{}, e
		}
	}
	configured, _ := v["bind_password_configured"].(bool)
	model := ldapConfigurationModel{ID: get("id"), DisplayName: get("display_name"), State: get("state"), ServerURL: get("server_url"), TLSMode: get("tls_mode"), TrustMode: get("trust_mode"), CAPEM: get("ca_pem"), BindDN: get("bind_dn"), BindPasswordConfigured: types.BoolValue(configured), SearchBaseDN: get("search_base_dn"), UserFilter: get("user_filter"), IdentityAttribute: get("identity_attribute"), UsernameAttribute: get("username_attribute"), NameAttribute: get("name_attribute"), EmailAttribute: get("email_attribute"), GroupSearchBaseDN: get("group_search_base_dn"), GroupUserFilter: get("group_user_filter"), GroupFilter: get("group_filter"), GroupIdentityAttribute: get("group_identity_attribute"), GroupMemberAttribute: get("group_member_attribute"), GroupMaxDepth: types.Int64Value(depth), SelectedUserIDs: selected}
	model.ID = types.StringValue(strings.ToLower(model.ID.ValueString()))
	model.CAPEM = types.StringValue(strings.TrimSpace(model.CAPEM.ValueString()))
	return model, nil
}
func writeLDAP(ctx context.Context, c *apiclient.SemaphoreUI, m ldapConfigurationModel) (ldapConfigurationModel, error) {
	var raw map[string]any
	if e := exRequest(ctx, c, http.MethodPut, "/capabilities/ldap", nil, ldapPayload(m), &raw); e != nil {
		return ldapConfigurationModel{}, e
	}
	configured, e := ldapModel(ctx, raw)
	if e != nil {
		return ldapConfigurationModel{}, e
	}
	var ids []int64
	if !m.SelectedUserIDs.IsNull() && !m.SelectedUserIDs.IsUnknown() {
		if d := m.SelectedUserIDs.ElementsAs(ctx, &ids, false); d.HasError() {
			return ldapConfigurationModel{}, fmt.Errorf("invalid LDAP selected user IDs")
		}
	}
	if e := exRequest(ctx, c, http.MethodPut, "/capabilities/ldap/state", nil, map[string]any{"provider_id": m.ID.ValueString(), "state": m.State.ValueString(), "selected_user_ids": ids}, nil); e != nil {
		return configured, e
	}
	next, e := readLDAP(ctx, c, m.ID.ValueString())
	if e != nil {
		return configured, e
	}
	return next, nil
}

func ldapRetainPartialState(ctx context.Context, state *tfsdk.State, next ldapConfigurationModel, configured ldapConfigurationModel) {
	if next.ID.IsNull() || next.ID.IsUnknown() {
		return
	}
	next.BindPasswordWOVersion = configured.BindPasswordWOVersion
	ldapPreserveConfiguredStrings(&next, configured)
	_ = state.Set(ctx, &next)
}
func ldapPlanWithBindPassword(ctx context.Context, config tfsdk.Config, plan *ldapConfigurationModel) error {
	var bindPassword types.String
	diagnostics := config.GetAttribute(ctx, path.Root("bind_password"), &bindPassword)
	if diagnostics.HasError() {
		return fmt.Errorf("could not read LDAP bind password from configuration")
	}
	plan.BindPassword = bindPassword
	return nil
}

func (r *ldapConfigurationResource) Create(ctx context.Context, q resource.CreateRequest, p *resource.CreateResponse) {
	var m ldapConfigurationModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &m)...)
	if p.Diagnostics.HasError() {
		return
	}
	if err := ldapPlanWithBindPassword(ctx, q.Config, &m); err != nil {
		p.Diagnostics.AddError("Invalid LDAP Bind Password", err.Error())
		return
	}
	switch m.State.ValueString() {
	case "disabled", "shadow":
	case "active", "selected_users":
		p.Diagnostics.AddAttributeError(path.Root("state"), "LDAP Initial State Requires Readiness", "Create the provider with state = \"disabled\" or \"shadow\", run the explicit LDAP test action, then update state to active or selected_users.")
		return
	default:
		p.Diagnostics.AddAttributeError(path.Root("state"), "Invalid LDAP Initial State", "state must be disabled, shadow, selected_users, or active.")
		return
	}
	if err := ldapValidateConfiguredSelection(ctx, q.Config, m.State); err != nil {
		p.Diagnostics.AddAttributeError(path.Root("selected_user_ids"), "Invalid LDAP Selected Users", err.Error())
		return
	}
	next, e := writeLDAP(ctx, r.client, m)
	if e != nil {
		ldapRetainPartialState(ctx, &p.State, next, m)
		p.Diagnostics.AddError("Error Configuring LDAP Provider", e.Error())
		return
	}
	next.BindPasswordWOVersion = m.BindPasswordWOVersion
	ldapPreserveConfiguredStrings(&next, m)
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (r *ldapConfigurationResource) Update(ctx context.Context, q resource.UpdateRequest, p *resource.UpdateResponse) {
	var m ldapConfigurationModel
	var prior ldapConfigurationModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &m)...)
	p.Diagnostics.Append(q.State.Get(ctx, &prior)...)
	if p.Diagnostics.HasError() {
		return
	}
	if err := ldapPlanWithBindPassword(ctx, q.Config, &m); err != nil {
		p.Diagnostics.AddError("Invalid LDAP Bind Password", err.Error())
		return
	}
	if err := ldapValidateConfiguredSelection(ctx, q.Config, m.State); err != nil {
		p.Diagnostics.AddAttributeError(path.Root("selected_user_ids"), "Invalid LDAP Selected Users", err.Error())
		return
	}
	ldapMergeUnknownOptionalComputed(&m, prior)
	bindPasswordVersionChanged := !m.BindPasswordWOVersion.Equal(prior.BindPasswordWOVersion)
	if !bindPasswordVersionChanged {
		m.BindPassword = types.StringNull()
	} else if m.BindPassword.IsNull() || m.BindPassword.IsUnknown() {
		p.Diagnostics.AddAttributeError(path.Root("bind_password"), "LDAP bind password version requires material", "Set bind_password when changing bind_password_wo_version.")
		return
	}
	if !bindPasswordVersionChanged && reflect.DeepEqual(ldapPayloadWithBindPassword(m, false), ldapPayloadWithBindPassword(prior, false)) {
		var ids []int64
		if d := m.SelectedUserIDs.ElementsAs(ctx, &ids, false); d.HasError() {
			p.Diagnostics.AddError("Invalid LDAP Selected Users", "selected_user_ids are invalid")
			return
		}
		if e := exRequest(ctx, r.client, http.MethodPut, "/capabilities/ldap/state", nil, map[string]any{"provider_id": m.ID.ValueString(), "state": m.State.ValueString(), "selected_user_ids": ids}, nil); e != nil {
			p.Diagnostics.AddError("Error Updating LDAP Provider State", e.Error())
			return
		}
		next, e := readLDAP(ctx, r.client, m.ID.ValueString())
		if e != nil {
			p.Diagnostics.AddError("Error Reading LDAP Provider", e.Error())
			return
		}
		next.BindPasswordWOVersion = m.BindPasswordWOVersion
		ldapPreserveConfiguredStrings(&next, m)
		p.Diagnostics.Append(p.State.Set(ctx, &next)...)
		return
	}
	next, e := writeLDAP(ctx, r.client, m)
	if e != nil {
		ldapRetainPartialState(ctx, &p.State, next, m)
		p.Diagnostics.AddError("Error Configuring LDAP Provider", e.Error())
		return
	}
	if bindPasswordVersionChanged {
		next.BindPasswordWOVersion = m.BindPasswordWOVersion
	}
	ldapPreserveConfiguredStrings(&next, m)
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (r *ldapConfigurationResource) Read(ctx context.Context, q resource.ReadRequest, p *resource.ReadResponse) {
	var m ldapConfigurationModel
	p.Diagnostics.Append(q.State.Get(ctx, &m)...)
	if p.Diagnostics.HasError() {
		return
	}
	next, e := readLDAP(ctx, r.client, m.ID.ValueString())
	if exNotFound(e) {
		p.State.RemoveResource(ctx)
		return
	}
	if e != nil {
		p.Diagnostics.AddError("Error Reading LDAP Provider", e.Error())
		return
	}
	next.BindPasswordWOVersion = m.BindPasswordWOVersion
	ldapPreserveConfiguredStrings(&next, m)
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (r *ldapConfigurationResource) Delete(ctx context.Context, q resource.DeleteRequest, p *resource.DeleteResponse) {
	var m ldapConfigurationModel
	p.Diagnostics.Append(q.State.Get(ctx, &m)...)
	if p.Diagnostics.HasError() {
		return
	}
	e := exRequest(ctx, r.client, http.MethodPut, "/capabilities/ldap/state", nil, map[string]any{"provider_id": m.ID.ValueString(), "state": "disabled", "selected_user_ids": []int64{}}, nil)
	if e != nil && !exNotFound(e) {
		p.Diagnostics.AddError("Error Disabling LDAP Provider", e.Error())
	}
}
func (r *ldapConfigurationResource) ImportState(ctx context.Context, q resource.ImportStateRequest, p *resource.ImportStateResponse) {
	next, e := readLDAP(ctx, r.client, q.ID)
	if e != nil {
		p.Diagnostics.AddError("Error Importing LDAP Provider", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (d *ldapConfigurationDataSource) Read(ctx context.Context, q datasource.ReadRequest, p *datasource.ReadResponse) {
	var m ldapConfigurationModel
	p.Diagnostics.Append(q.Config.Get(ctx, &m)...)
	if p.Diagnostics.HasError() {
		return
	}
	next, e := readLDAP(ctx, d.client, m.ID.ValueString())
	if e != nil {
		p.Diagnostics.AddError("Error Reading LDAP Provider", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
