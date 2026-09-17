package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

type identityMappingResource struct {
	client *apiclient.SemaphoreUI
	oidc   bool
}
type identityMappingDataSource struct {
	client *apiclient.SemaphoreUI
	oidc   bool
}
type totpPolicyResource struct{ client *apiclient.SemaphoreUI }
type totpPolicyDataSource struct{ client *apiclient.SemaphoreUI }

func NewLDAPGroupMappingResource() resource.Resource       { return &identityMappingResource{} }
func NewLDAPGroupMappingDataSource() datasource.DataSource { return &identityMappingDataSource{} }
func NewOIDCGroupMappingResource() resource.Resource       { return &identityMappingResource{oidc: true} }
func NewOIDCGroupMappingDataSource() datasource.DataSource {
	return &identityMappingDataSource{oidc: true}
}
func NewTOTPPolicyResource() resource.Resource       { return &totpPolicyResource{} }
func NewTOTPPolicyDataSource() datasource.DataSource { return &totpPolicyDataSource{} }

func identityKind(oidc bool) string {
	if oidc {
		return "OIDC"
	}
	return "LDAP"
}
func identityValueName(oidc bool) string {
	if oidc {
		return "claim_value"
	}
	return "group_external_id"
}
func identityMappingValue(oidc bool, m identityMappingModel) types.String {
	if oidc {
		return m.ClaimValue
	}
	return m.GroupExternalID
}
func identityRoute(oidc bool) string {
	if oidc {
		return "/capabilities/oidc/group-mappings"
	}
	return "/capabilities/ldap/group-mappings"
}
func (r *identityMappingResource) Metadata(_ context.Context, q resource.MetadataRequest, p *resource.MetadataResponse) {
	n := "ldap_group_mapping"
	if r.oidc {
		n = "oidc_group_mapping"
	}
	p.TypeName = q.ProviderTypeName + "_" + n
}
func (d *identityMappingDataSource) Metadata(_ context.Context, q datasource.MetadataRequest, p *datasource.MetadataResponse) {
	n := "ldap_group_mapping"
	if d.oidc {
		n = "oidc_group_mapping"
	}
	p.TypeName = q.ProviderTypeName + "_" + n
}
func (r *identityMappingResource) Configure(_ context.Context, q resource.ConfigureRequest, p *resource.ConfigureResponse) {
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
func (d *identityMappingDataSource) Configure(_ context.Context, q datasource.ConfigureRequest, p *datasource.ConfigureResponse) {
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
func (r *identityMappingResource) Schema(_ context.Context, _ resource.SchemaRequest, p *resource.SchemaResponse) {
	p.Schema = identityMappingResourceSchema("Maps an external identity-provider group to a custom role.", r.oidc)
}
func (d *identityMappingDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, p *datasource.SchemaResponse) {
	p.Schema = identityMappingDataSourceSchema("Reads an external identity-provider group mapping.")
}

func identityTarget(ctx context.Context, value types.Object) (map[string]any, error) {
	var target identityTargetModel
	diags := value.As(ctx, &target, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, fmt.Errorf("identity role target is invalid")
	}
	out := map[string]any{"scope": target.Scope.ValueString(), "role_id": target.RoleID.ValueString()}
	if !target.ProjectID.IsNull() {
		out["project_id"] = target.ProjectID.ValueInt64()
	}
	return out, nil
}
func identityModel(ctx context.Context, oidc bool, raw map[string]any) (identityMappingModel, error) {
	target, ok := raw["target"].(map[string]any)
	if !ok {
		return identityMappingModel{}, fmt.Errorf("API response has no role target")
	}
	obj, err := exTypedValue(ctx, types.ObjectType{AttrTypes: identityTargetTypes}, target)
	if err != nil {
		return identityMappingModel{}, err
	}
	object, ok := obj.(types.Object)
	if !ok {
		return identityMappingModel{}, fmt.Errorf("API response has an invalid role target")
	}
	value, ok := raw[identityValueName(oidc)].(string)
	if !ok {
		return identityMappingModel{}, fmt.Errorf("API response has no mapping value")
	}
	id, _ := raw["id"].(string)
	providerID, _ := raw["provider_id"].(string)
	enabled, _ := raw["enabled"].(bool)
	revision, err := identityNumber(raw["revision"])
	if err != nil {
		return identityMappingModel{}, err
	}
	mapping := identityMappingModel{ID: types.StringValue(id), ProviderID: types.StringValue(providerID), GroupExternalID: types.StringNull(), ClaimValue: types.StringNull(), Target: object, Enabled: types.BoolValue(enabled), Revision: types.Int64Value(revision)}
	if oidc {
		mapping.ClaimValue = types.StringValue(value)
	} else {
		mapping.GroupExternalID = types.StringValue(value)
	}
	return mapping, nil
}
func identityNumber(v any) (int64, error) {
	switch n := v.(type) {
	case json.Number:
		return n.Int64()
	case float64:
		return int64(n), nil
	case int64:
		return n, nil
	case int:
		return int64(n), nil
	default:
		return 0, fmt.Errorf("API response has no mapping revision")
	}
}
func readIdentityMapping(ctx context.Context, c *apiclient.SemaphoreUI, oidc bool, state identityMappingModel) (identityMappingModel, error) {
	var values []map[string]any
	o := exRequestOptions{Query: map[string]string{"provider_id": state.ProviderID.ValueString()}}
	if err := exRequestWithOptions(ctx, c, http.MethodGet, identityRoute(oidc), o, nil, &values); err != nil {
		return identityMappingModel{}, err
	}
	for _, v := range values {
		if id, _ := v["id"].(string); id == state.ID.ValueString() {
			next, modelErr := identityModel(ctx, oidc, v)
			if modelErr != nil {
				return identityMappingModel{}, modelErr
			}
			if oidc && strings.EqualFold(next.ClaimValue.ValueString(), state.ClaimValue.ValueString()) {
				var providers []struct {
					ID                 string `json:"id"`
					ClaimConfiguration struct {
						CaseInsensitive bool `json:"case_insensitive"`
					} `json:"claim_configuration"`
				}
				if err := exRequest(ctx, c, http.MethodGet, "/capabilities/oidc/group-mapping/providers", nil, nil, &providers); err != nil {
					return identityMappingModel{}, err
				}
				for _, provider := range providers {
					if provider.ID == state.ProviderID.ValueString() && provider.ClaimConfiguration.CaseInsensitive {
						next.ClaimValue = state.ClaimValue
						break
					}
				}
			}
			return next, nil
		}
	}
	return identityMappingModel{}, &exAPIError{StatusCode: 404, method: http.MethodGet, route: identityRoute(oidc)}
}
func oidcProviderCaseInsensitive(ctx context.Context, c *apiclient.SemaphoreUI, providerID string) (bool, error) {
	var providers []struct {
		ID                 string `json:"id"`
		ClaimConfiguration struct {
			CaseInsensitive bool `json:"case_insensitive"`
		} `json:"claim_configuration"`
	}
	if err := exRequest(ctx, c, http.MethodGet, "/capabilities/oidc/group-mapping/providers", nil, nil, &providers); err != nil {
		return false, err
	}
	for _, provider := range providers {
		if provider.ID == providerID {
			return provider.ClaimConfiguration.CaseInsensitive, nil
		}
	}
	return false, fmt.Errorf("OIDC mapping provider is unavailable")
}
func writeIdentityMapping(ctx context.Context, c *apiclient.SemaphoreUI, oidc bool, m identityMappingModel) (identityMappingModel, error) {
	caseInsensitive := false
	if oidc {
		var configErr error
		caseInsensitive, configErr = oidcProviderCaseInsensitive(ctx, c, m.ProviderID.ValueString())
		if configErr != nil {
			return identityMappingModel{}, configErr
		}
	}
	target, err := identityTarget(ctx, m.Target)
	if err != nil {
		return identityMappingModel{}, err
	}
	body := map[string]any{"provider_id": m.ProviderID.ValueString(), identityValueName(oidc): identityMappingValue(oidc, m).ValueString(), "target": target, "enabled": m.Enabled.ValueBool(), "expected_revision": 0}
	if !m.Revision.IsNull() && !m.Revision.IsUnknown() {
		body["expected_revision"] = m.Revision.ValueInt64()
	}
	var raw map[string]any
	o := exRequestOptions{PathParams: map[string]string{"mapping_id": m.ID.ValueString()}}
	err = exRequestWithOptions(ctx, c, http.MethodPut, identityRoute(oidc)+"/{mapping_id}", o, body, &raw)
	if err != nil {
		return identityMappingModel{}, err
	}
	next, modelErr := identityModel(ctx, oidc, raw)
	if modelErr != nil {
		return identityMappingModel{}, modelErr
	}
	if oidc && caseInsensitive && strings.EqualFold(next.ClaimValue.ValueString(), m.ClaimValue.ValueString()) {
		next.ClaimValue = m.ClaimValue
	}
	return next, nil
}
func (r *identityMappingResource) Create(ctx context.Context, q resource.CreateRequest, p *resource.CreateResponse) {
	var m identityMappingModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &m)...)
	if p.Diagnostics.HasError() {
		return
	}
	next, e := writeIdentityMapping(ctx, r.client, r.oidc, m)
	if e != nil {
		p.Diagnostics.AddError("Error Creating "+identityKind(r.oidc)+" Group Mapping", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (r *identityMappingResource) Read(ctx context.Context, q resource.ReadRequest, p *resource.ReadResponse) {
	var m identityMappingModel
	p.Diagnostics.Append(q.State.Get(ctx, &m)...)
	if p.Diagnostics.HasError() {
		return
	}
	next, e := readIdentityMapping(ctx, r.client, r.oidc, m)
	if exNotFound(e) {
		p.State.RemoveResource(ctx)
		return
	}
	if e != nil {
		p.Diagnostics.AddError("Error Reading "+identityKind(r.oidc)+" Group Mapping", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (r *identityMappingResource) Update(ctx context.Context, q resource.UpdateRequest, p *resource.UpdateResponse) {
	var m identityMappingModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &m)...)
	if p.Diagnostics.HasError() {
		return
	}
	var prior identityMappingModel
	p.Diagnostics.Append(q.State.Get(ctx, &prior)...)
	if p.Diagnostics.HasError() {
		return
	}
	m.Revision = prior.Revision
	next, e := writeIdentityMapping(ctx, r.client, r.oidc, m)
	if e != nil {
		p.Diagnostics.AddError("Error Updating "+identityKind(r.oidc)+" Group Mapping", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (r *identityMappingResource) Delete(ctx context.Context, q resource.DeleteRequest, p *resource.DeleteResponse) {
	var m identityMappingModel
	p.Diagnostics.Append(q.State.Get(ctx, &m)...)
	if p.Diagnostics.HasError() {
		return
	}
	if m.Revision.IsNull() || m.Revision.IsUnknown() || m.Revision.ValueInt64() <= 0 {
		p.Diagnostics.AddError("Mapping Revision Unavailable", "Refresh the mapping and retry; deletion requires the prior server revision.")
		return
	}
	o := exRequestOptions{PathParams: map[string]string{"mapping_id": m.ID.ValueString()}, Query: map[string]string{"provider_id": m.ProviderID.ValueString(), "expected_revision": strconv.FormatInt(m.Revision.ValueInt64(), 10)}}
	e := exRequestWithOptions(ctx, r.client, http.MethodDelete, identityRoute(r.oidc)+"/{mapping_id}", o, nil, nil)
	if e != nil && !exNotFound(e) {
		p.Diagnostics.AddError("Error Removing "+identityKind(r.oidc)+" Group Mapping", e.Error())
	}
}
func (r *identityMappingResource) ImportState(ctx context.Context, q resource.ImportStateRequest, p *resource.ImportStateResponse) {
	parts := strings.Split(q.ID, "/")
	if len(parts) != 3 || parts[1] != "mapping" || parts[0] == "" || parts[2] == "" {
		p.Diagnostics.AddError("Invalid "+identityKind(r.oidc)+" Group Mapping Import ID", "Use <provider_id>/mapping/<mapping_id>.")
		return
	}
	next, e := readIdentityMapping(ctx, r.client, r.oidc, identityMappingModel{ProviderID: types.StringValue(parts[0]), ID: types.StringValue(parts[2])})
	if e != nil {
		p.Diagnostics.AddError("Error Importing "+identityKind(r.oidc)+" Group Mapping", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (d *identityMappingDataSource) Read(ctx context.Context, q datasource.ReadRequest, p *datasource.ReadResponse) {
	var m identityMappingModel
	p.Diagnostics.Append(q.Config.Get(ctx, &m)...)
	if p.Diagnostics.HasError() {
		return
	}
	next, e := readIdentityMapping(ctx, d.client, d.oidc, m)
	if e != nil {
		p.Diagnostics.AddError("Error Reading "+identityKind(d.oidc)+" Group Mapping", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}

func (r *totpPolicyResource) Metadata(_ context.Context, q resource.MetadataRequest, p *resource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_totp_policy"
}
func (d *totpPolicyDataSource) Metadata(_ context.Context, q datasource.MetadataRequest, p *datasource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_totp_policy"
}
func (r *totpPolicyResource) Configure(_ context.Context, q resource.ConfigureRequest, p *resource.ConfigureResponse) {
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
func (d *totpPolicyDataSource) Configure(_ context.Context, q datasource.ConfigureRequest, p *datasource.ConfigureResponse) {
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
func (r *totpPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, p *resource.SchemaResponse) {
	p.Schema = totpPolicyResourceSchema()
}
func (d *totpPolicyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, p *datasource.SchemaResponse) {
	p.Schema = totpPolicyDataSourceSchema()
}
func readTOTP(ctx context.Context, c *apiclient.SemaphoreUI) (totpPolicyModel, error) {
	var raw struct {
		State           string  `json:"state"`
		SelectedUserIDs []int64 `json:"selected_user_ids"`
	}
	if e := exRequest(ctx, c, http.MethodGet, "/capabilities/totp", nil, nil, &raw); e != nil {
		return totpPolicyModel{}, e
	}
	v, diags := types.ListValueFrom(ctx, types.Int64Type, raw.SelectedUserIDs)
	if diags.HasError() {
		return totpPolicyModel{}, fmt.Errorf("API response has invalid selected user IDs")
	}
	return totpPolicyModel{ID: types.StringValue("totp"), State: types.StringValue(raw.State), SelectedUserIDs: v}, nil
}
func writeTOTP(ctx context.Context, c *apiclient.SemaphoreUI, m totpPolicyModel) (totpPolicyModel, error) {
	var ids []int64
	if !m.SelectedUserIDs.IsNull() && !m.SelectedUserIDs.IsUnknown() {
		diags := m.SelectedUserIDs.ElementsAs(ctx, &ids, false)
		if diags.HasError() {
			return totpPolicyModel{}, fmt.Errorf("selected user IDs are invalid")
		}
	}
	if m.State.ValueString() != "required_selected" && len(ids) != 0 {
		return totpPolicyModel{}, fmt.Errorf("selected_user_ids may be set only when state is required_selected")
	}
	seen := map[int64]struct{}{}
	for _, id := range ids {
		if id < 1 {
			return totpPolicyModel{}, fmt.Errorf("selected user IDs must be positive")
		}
		if _, duplicate := seen[id]; duplicate {
			return totpPolicyModel{}, fmt.Errorf("selected user IDs must be unique")
		}
		seen[id] = struct{}{}
	}
	body := map[string]any{"state": m.State.ValueString(), "selected_user_ids": ids}
	if e := exRequest(ctx, c, http.MethodPut, "/capabilities/totp", nil, body, nil); e != nil {
		return totpPolicyModel{}, e
	}
	return readTOTP(ctx, c)
}
func (r *totpPolicyResource) Create(ctx context.Context, q resource.CreateRequest, p *resource.CreateResponse) {
	var m totpPolicyModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &m)...)
	if p.Diagnostics.HasError() {
		return
	}
	next, e := writeTOTP(ctx, r.client, m)
	if e != nil {
		p.Diagnostics.AddError("Error Configuring TOTP Policy", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (r *totpPolicyResource) Read(ctx context.Context, q resource.ReadRequest, p *resource.ReadResponse) {
	next, e := readTOTP(ctx, r.client)
	if e != nil {
		p.Diagnostics.AddError("Error Reading TOTP Policy", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (r *totpPolicyResource) Update(ctx context.Context, q resource.UpdateRequest, p *resource.UpdateResponse) {
	var m totpPolicyModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &m)...)
	if p.Diagnostics.HasError() {
		return
	}
	next, e := writeTOTP(ctx, r.client, m)
	if e != nil {
		p.Diagnostics.AddError("Error Configuring TOTP Policy", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (r *totpPolicyResource) Delete(ctx context.Context, q resource.DeleteRequest, p *resource.DeleteResponse) {
	_, e := writeTOTP(ctx, r.client, totpPolicyModel{State: types.StringValue("disabled")})
	if e != nil {
		p.Diagnostics.AddError("Error Disabling TOTP Policy", e.Error())
	}
}
func (r *totpPolicyResource) ImportState(ctx context.Context, _ resource.ImportStateRequest, p *resource.ImportStateResponse) {
	next, e := readTOTP(ctx, r.client)
	if e != nil {
		p.Diagnostics.AddError("Error Importing TOTP Policy", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (d *totpPolicyDataSource) Read(ctx context.Context, _ datasource.ReadRequest, p *datasource.ReadResponse) {
	next, e := readTOTP(ctx, d.client)
	if e != nil {
		p.Diagnostics.AddError("Error Reading TOTP Policy", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
