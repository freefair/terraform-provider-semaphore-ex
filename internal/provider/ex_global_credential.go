package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type exGlobalCredentialModel struct {
	ID                types.Int64  `tfsdk:"id"`
	Type              types.String `tfsdk:"type"`
	DisplayName       types.String `tfsdk:"display_name"`
	Enabled           types.Bool   `tfsdk:"enabled"`
	Revision          types.Int64  `tfsdk:"revision"`
	CurrentVersion    types.Int64  `tfsdk:"current_version"`
	Fingerprint       types.String `tfsdk:"fingerprint"`
	MaterialKind      types.String `tfsdk:"material_kind"`
	Value             types.String `tfsdk:"value"`
	ValueWO           types.String `tfsdk:"value_wo"`
	ValueWOVersion    types.Int64  `tfsdk:"value_wo_version"`
	ExternalReference types.Object `tfsdk:"external_reference"`
}

type exGlobalCredentialDataSourceModel struct {
	ID                types.Int64  `tfsdk:"id"`
	Type              types.String `tfsdk:"type"`
	DisplayName       types.String `tfsdk:"display_name"`
	Enabled           types.Bool   `tfsdk:"enabled"`
	Revision          types.Int64  `tfsdk:"revision"`
	CurrentVersion    types.Int64  `tfsdk:"current_version"`
	Fingerprint       types.String `tfsdk:"fingerprint"`
	MaterialKind      types.String `tfsdk:"material_kind"`
	ExternalReference types.Object `tfsdk:"external_reference"`
}

type exGlobalCredentialResponse struct {
	ID                int64          `json:"id"`
	Type              string         `json:"type"`
	DisplayName       string         `json:"display_name"`
	Enabled           bool           `json:"enabled"`
	Revision          int64          `json:"revision"`
	CurrentVersion    int64          `json:"current_version"`
	Fingerprint       string         `json:"fingerprint"`
	MaterialKind      string         `json:"material_kind"`
	ExternalReference map[string]any `json:"external_reference"`
}

type exGlobalCredentialResource struct{ client *apiclient.SemaphoreUI }
type exGlobalCredentialDataSource struct{ client *apiclient.SemaphoreUI }

var _ resource.ResourceWithImportState = &exGlobalCredentialResource{}

func NewGlobalCredentialResource() resource.Resource { return &exGlobalCredentialResource{} }
func NewGlobalCredentialDataSource() datasource.DataSource {
	return withNamedLookup(&exGlobalCredentialDataSource{}, "global_credential")
}

func (r *exGlobalCredentialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_global_credential"
}
func (d *exGlobalCredentialDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_global_credential"
}
func (r *exGlobalCredentialResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", "Expected *client.SemaphoreUI.")
		return
	}
	r.client = c
}
func (d *exGlobalCredentialDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", "Expected *client.SemaphoreUI.")
		return
	}
	d.client = c
}

func exGlobalCredentialExternalReferenceAttributes() map[string]resourceschema.Attribute {
	return map[string]resourceschema.Attribute{
		"provider": resourceschema.StringAttribute{Required: true}, "provider_id": resourceschema.StringAttribute{Required: true},
		"mount": resourceschema.StringAttribute{Required: true}, "path": resourceschema.StringAttribute{Required: true},
		"version": resourceschema.Int64Attribute{Required: true}, "field": resourceschema.StringAttribute{Required: true},
	}
}
func exGlobalCredentialExternalReferenceType() map[string]attr.Type {
	return map[string]attr.Type{"provider": types.StringType, "provider_id": types.StringType, "mount": types.StringType, "path": types.StringType, "version": types.Int64Type, "field": types.StringType}
}
func exGlobalCredentialResourceSchema() resourceschema.Schema {
	return resourceschema.Schema{MarkdownDescription: "Manages a global Semaphore EX credential without reading credential material from the API.", Attributes: map[string]resourceschema.Attribute{
		"id":           resourceschema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
		"type":         resourceschema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("string")}},
		"display_name": resourceschema.StringAttribute{Required: true},
		"enabled":      resourceschema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"revision":     resourceschema.Int64Attribute{Computed: true}, "current_version": resourceschema.Int64Attribute{Computed: true},
		"fingerprint": resourceschema.StringAttribute{Computed: true}, "material_kind": resourceschema.StringAttribute{Computed: true},
		"value":              resourceschema.StringAttribute{Optional: true, Sensitive: true, Validators: []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("value_wo")), stringvalidator.ConflictsWith(path.MatchRoot("external_reference"))}},
		"value_wo":           resourceschema.StringAttribute{Optional: true, Sensitive: true, WriteOnly: true, Validators: []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("value")), stringvalidator.ConflictsWith(path.MatchRoot("external_reference"))}},
		"value_wo_version":   resourceschema.Int64Attribute{Optional: true},
		"external_reference": resourceschema.SingleNestedAttribute{Optional: true, Attributes: exGlobalCredentialExternalReferenceAttributes()},
	}}
}
func exGlobalCredentialDataSourceSchema() datasourceschema.Schema {
	ref := map[string]datasourceschema.Attribute{"provider": datasourceschema.StringAttribute{Computed: true}, "provider_id": datasourceschema.StringAttribute{Computed: true}, "mount": datasourceschema.StringAttribute{Computed: true}, "path": datasourceschema.StringAttribute{Computed: true}, "version": datasourceschema.Int64Attribute{Computed: true}, "field": datasourceschema.StringAttribute{Computed: true}}
	return datasourceschema.Schema{MarkdownDescription: "Reads value-free global Semaphore EX credential metadata.", Attributes: map[string]datasourceschema.Attribute{
		"id": datasourceschema.Int64Attribute{Required: true}, "type": datasourceschema.StringAttribute{Computed: true}, "display_name": datasourceschema.StringAttribute{Computed: true}, "enabled": datasourceschema.BoolAttribute{Computed: true}, "revision": datasourceschema.Int64Attribute{Computed: true}, "current_version": datasourceschema.Int64Attribute{Computed: true}, "fingerprint": datasourceschema.StringAttribute{Computed: true}, "material_kind": datasourceschema.StringAttribute{Computed: true}, "external_reference": datasourceschema.SingleNestedAttribute{Computed: true, Attributes: ref},
	}}
}
func (r *exGlobalCredentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = exGlobalCredentialResourceSchema()
}
func (d *exGlobalCredentialDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = exGlobalCredentialDataSourceSchema()
}

func exGlobalCredentialExternalReference(ctx context.Context, value types.Object) (map[string]any, error) {
	if value.IsNull() {
		return nil, nil
	}
	if value.IsUnknown() {
		return nil, fmt.Errorf("external_reference is unknown")
	}
	v, err := exWireValue(ctx, value)
	if err != nil {
		return nil, err
	}
	result, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("external_reference is invalid")
	}
	return result, nil
}
func exGlobalCredentialMaterial(ctx context.Context, m exGlobalCredentialModel, useWriteOnly bool) (map[string]any, error) {
	if useWriteOnly && !m.ValueWO.IsNull() && !m.ValueWO.IsUnknown() {
		return map[string]any{"string_value": m.ValueWO.ValueString()}, nil
	}
	if !useWriteOnly && !m.Value.IsNull() && !m.Value.IsUnknown() {
		return map[string]any{"string_value": m.Value.ValueString()}, nil
	}
	ref, err := exGlobalCredentialExternalReference(ctx, m.ExternalReference)
	if err != nil {
		return nil, err
	}
	if ref != nil {
		return map[string]any{"external_reference": ref}, nil
	}
	return nil, fmt.Errorf("set exactly one of value, value_wo, or external_reference")
}
func exGlobalCredentialModelFromResponse(ctx context.Context, old exGlobalCredentialModel, raw exGlobalCredentialResponse) (exGlobalCredentialModel, error) {
	next := old
	next.ID = types.Int64Value(raw.ID)
	next.Type = types.StringValue(raw.Type)
	next.DisplayName = types.StringValue(raw.DisplayName)
	next.Enabled = types.BoolValue(raw.Enabled)
	next.Revision = types.Int64Value(raw.Revision)
	next.CurrentVersion = types.Int64Value(raw.CurrentVersion)
	next.Fingerprint = types.StringValue(raw.Fingerprint)
	next.MaterialKind = types.StringValue(raw.MaterialKind)
	if raw.ExternalReference != nil {
		value, err := exTypedValue(ctx, types.ObjectType{AttrTypes: exGlobalCredentialExternalReferenceType()}, raw.ExternalReference)
		if err != nil {
			return next, err
		}
		externalReference, ok := value.(types.Object)
		if !ok {
			return next, fmt.Errorf("API external reference does not match the Terraform schema")
		}
		next.ExternalReference = externalReference
	} else if next.ExternalReference.IsNull() || next.ExternalReference.IsUnknown() || len(next.ExternalReference.AttributeTypes(ctx)) == 0 {
		next.ExternalReference = types.ObjectNull(exGlobalCredentialExternalReferenceType())
	}
	return next, nil
}

func exGlobalCredentialRead(ctx context.Context, client *apiclient.SemaphoreUI, old exGlobalCredentialModel) (exGlobalCredentialModel, error) {
	var raw exGlobalCredentialResponse
	if err := exRequest(ctx, client, http.MethodGet, "/global-credentials/{credential_id}", map[string]string{"credential_id": strconv.FormatInt(old.ID.ValueInt64(), 10)}, nil, &raw); err != nil {
		return old, err
	}
	return exGlobalCredentialModelFromResponse(ctx, old, raw)
}
func exGlobalCredentialMutationState(ctx context.Context, old exGlobalCredentialModel, raw exGlobalCredentialResponse) (exGlobalCredentialModel, error) {
	return exGlobalCredentialModelFromResponse(ctx, old, raw)
}
func exGlobalCredentialRequireRevision(m exGlobalCredentialModel) error {
	if m.Revision.IsNull() || m.Revision.IsUnknown() || m.Revision.ValueInt64() <= 0 {
		return fmt.Errorf("refresh the global credential and retry; the provider will not overwrite a concurrent credential change")
	}
	return nil
}
func (r *exGlobalCredentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan exGlobalCredentialModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	var config exGlobalCredentialModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	materialSource := plan
	materialSource.ValueWO = config.ValueWO
	material, err := exGlobalCredentialMaterial(ctx, materialSource, true)
	if err != nil {
		material, err = exGlobalCredentialMaterial(ctx, plan, false)
	}
	if err != nil {
		resp.Diagnostics.AddError("Invalid Global Credential Material", err.Error())
		return
	}
	body := map[string]any{"type": plan.Type.ValueString(), "display_name": plan.DisplayName.ValueString(), "material": material}
	var raw exGlobalCredentialResponse
	if err = exRequest(ctx, r.client, http.MethodPost, "/global-credentials", nil, body, &raw); err != nil {
		resp.Diagnostics.AddError("Error Creating Semaphore EX Global Credential", err.Error())
		return
	}
	state, err := exGlobalCredentialMutationState(ctx, plan, raw)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Global Credential Response", err.Error())
		return
	}
	if !plan.Enabled.ValueBool() {
		if err := exRequest(ctx, r.client, http.MethodPost, "/global-credentials/{credential_id}/enabled", map[string]string{"credential_id": strconv.FormatInt(state.ID.ValueInt64(), 10)}, map[string]any{"enabled": false, "revision": state.Revision.ValueInt64()}, &raw); err != nil {
			resp.Diagnostics.AddError("Error Setting Semaphore EX Global Credential Enabled State", err.Error())
			return
		}
		state, err = exGlobalCredentialMutationState(ctx, state, raw)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Global Credential Response", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
func (r *exGlobalCredentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state exGlobalCredentialModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	next, err := exGlobalCredentialRead(ctx, r.client, state)
	if exNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Semaphore EX Global Credential", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}
func (r *exGlobalCredentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state exGlobalCredentialModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	var config exGlobalCredentialModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := exGlobalCredentialRequireRevision(state); err != nil {
		resp.Diagnostics.AddError("Global Credential Revision Unavailable", err.Error())
		return
	}
	plan.ID = state.ID
	plan.Revision = state.Revision
	plan.CurrentVersion = state.CurrentVersion
	plan.Fingerprint = state.Fingerprint
	plan.MaterialKind = state.MaterialKind
	current := state
	if !plan.DisplayName.Equal(state.DisplayName) {
		var raw exGlobalCredentialResponse
		if err := exRequest(ctx, r.client, http.MethodPut, "/global-credentials/{credential_id}", map[string]string{"credential_id": strconv.FormatInt(state.ID.ValueInt64(), 10)}, map[string]any{"display_name": plan.DisplayName.ValueString(), "revision": state.Revision.ValueInt64()}, &raw); err != nil {
			resp.Diagnostics.AddError("Error Updating Semaphore EX Global Credential", err.Error())
			return
		}
		var err error
		current, err = exGlobalCredentialMutationState(ctx, plan, raw)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Global Credential Response", err.Error())
			return
		}
	}
	materialChanged := (!plan.Value.Equal(state.Value)) || (!plan.ExternalReference.Equal(state.ExternalReference)) || (!config.ValueWO.IsNull() && !config.ValueWO.IsUnknown() && !plan.ValueWOVersion.Equal(state.ValueWOVersion))
	if materialChanged {
		materialSource := plan
		materialSource.ValueWO = config.ValueWO
		material, err := exGlobalCredentialMaterial(ctx, materialSource, true)
		if err != nil {
			material, err = exGlobalCredentialMaterial(ctx, plan, false)
		}
		if err != nil {
			resp.Diagnostics.AddError("Invalid Global Credential Material", err.Error())
			return
		}
		var raw exGlobalCredentialResponse
		if err = exRequest(ctx, r.client, http.MethodPost, "/global-credentials/{credential_id}/rotate", map[string]string{"credential_id": strconv.FormatInt(state.ID.ValueInt64(), 10)}, map[string]any{"material": material, "revision": current.Revision.ValueInt64()}, &raw); err != nil {
			resp.Diagnostics.AddError("Error Rotating Semaphore EX Global Credential", err.Error())
			return
		}
		current, err = exGlobalCredentialMutationState(ctx, plan, raw)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Global Credential Response", err.Error())
			return
		}
	}
	if !plan.Enabled.Equal(state.Enabled) {
		var raw exGlobalCredentialResponse
		if err := exRequest(ctx, r.client, http.MethodPost, "/global-credentials/{credential_id}/enabled", map[string]string{"credential_id": strconv.FormatInt(state.ID.ValueInt64(), 10)}, map[string]any{"enabled": plan.Enabled.ValueBool(), "revision": current.Revision.ValueInt64()}, &raw); err != nil {
			resp.Diagnostics.AddError("Error Updating Semaphore EX Global Credential Enabled State", err.Error())
			return
		}
		var err error
		current, err = exGlobalCredentialMutationState(ctx, plan, raw)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Global Credential Response", err.Error())
			return
		}
	}
	current.Value = plan.Value
	current.ValueWOVersion = plan.ValueWOVersion
	resp.Diagnostics.Append(resp.State.Set(ctx, &current)...)
}
func (r *exGlobalCredentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state exGlobalCredentialModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := exGlobalCredentialRequireRevision(state); err != nil {
		resp.Diagnostics.AddError("Global Credential Revision Unavailable", err.Error())
		return
	}
	current := state
	if current.Enabled.ValueBool() {
		var raw exGlobalCredentialResponse
		if err := exRequest(ctx, r.client, http.MethodPost, "/global-credentials/{credential_id}/enabled", map[string]string{"credential_id": strconv.FormatInt(current.ID.ValueInt64(), 10)}, map[string]any{"enabled": false, "revision": current.Revision.ValueInt64()}, &raw); err != nil {
			resp.Diagnostics.AddError("Error Disabling Semaphore EX Global Credential Before Deletion", err.Error())
			return
		}
		var err error
		current, err = exGlobalCredentialMutationState(ctx, current, raw)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Global Credential Response", err.Error())
			return
		}
	}
	err := exRequestWithOptions(ctx, r.client, http.MethodDelete, "/global-credentials/{credential_id}", exRequestOptions{PathParams: map[string]string{"credential_id": strconv.FormatInt(current.ID.ValueInt64(), 10)}, Query: map[string]string{"expected_revision": strconv.FormatInt(current.Revision.ValueInt64(), 10)}}, nil, nil)
	if err != nil && !exNotFound(err) {
		resp.Diagnostics.AddError("Error Removing Semaphore EX Global Credential", err.Error())
	}
}
func (r *exGlobalCredentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil || id <= 0 {
		resp.Diagnostics.AddError("Invalid Global Credential Import ID", "Use the positive numeric global credential ID.")
		return
	}
	next, err := exGlobalCredentialRead(ctx, r.client, exGlobalCredentialModel{ID: types.Int64Value(id)})
	if err != nil {
		resp.Diagnostics.AddError("Error Importing Semaphore EX Global Credential", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}
func (d *exGlobalCredentialDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config exGlobalCredentialDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	next, err := exGlobalCredentialRead(ctx, d.client, exGlobalCredentialModel{ID: config.ID})
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Semaphore EX Global Credential", err.Error())
		return
	}
	state := exGlobalCredentialDataSourceModel{ID: next.ID, Type: next.Type, DisplayName: next.DisplayName, Enabled: next.Enabled, Revision: next.Revision, CurrentVersion: next.CurrentVersion, Fingerprint: next.Fingerprint, MaterialKind: next.MaterialKind, ExternalReference: next.ExternalReference}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
