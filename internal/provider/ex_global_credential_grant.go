package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type exGlobalCredentialGrantModel struct {
	ID           types.Int64  `tfsdk:"id"`
	CredentialID types.Int64  `tfsdk:"credential_id"`
	ProjectID    types.Int64  `tfsdk:"project_id"`
	Operations   types.Set    `tfsdk:"operations"`
	ExpiresAt    types.String `tfsdk:"expires_at"`
	Status       types.String `tfsdk:"status"`
	Revision     types.Int64  `tfsdk:"revision"`
}
type exGlobalCredentialGrantResponse struct {
	ID           int64   `json:"id"`
	CredentialID int64   `json:"credential_id"`
	ProjectID    int64   `json:"project_id"`
	Operations   int64   `json:"operations"`
	ExpiresAt    *string `json:"expires_at"`
	Status       string  `json:"status"`
	Revision     int64   `json:"revision"`
}
type exGlobalCredentialGrantResource struct{ client *apiclient.SemaphoreUI }
type exGlobalCredentialGrantDataSource struct{ client *apiclient.SemaphoreUI }

var _ resource.ResourceWithImportState = &exGlobalCredentialGrantResource{}

func NewGlobalCredentialGrantResource() resource.Resource { return &exGlobalCredentialGrantResource{} }
func NewGlobalCredentialGrantDataSource() datasource.DataSource {
	return &exGlobalCredentialGrantDataSource{}
}
func (r *exGlobalCredentialGrantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_global_credential_grant"
}
func (d *exGlobalCredentialGrantDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_global_credential_grant"
}
func (r *exGlobalCredentialGrantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (d *exGlobalCredentialGrantDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func exGlobalCredentialGrantResourceSchema() resourceschema.Schema {
	return resourceschema.Schema{MarkdownDescription: "Manages a project grant for a global Semaphore EX credential.", Attributes: map[string]resourceschema.Attribute{
		"id":            resourceschema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
		"credential_id": resourceschema.Int64Attribute{Required: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}},
		"project_id":    resourceschema.Int64Attribute{Required: true},
		"operations":    resourceschema.SetAttribute{Required: true, ElementType: types.StringType},
		"expires_at":    resourceschema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"status":        resourceschema.StringAttribute{Optional: true, Computed: true, Validators: []validator.String{stringvalidator.OneOf("active", "revoked")}}, "revision": resourceschema.Int64Attribute{Computed: true},
	}}
}
func exGlobalCredentialGrantDataSourceSchema() datasourceschema.Schema {
	return datasourceschema.Schema{MarkdownDescription: "Reads a global Semaphore EX credential project grant.", Attributes: map[string]datasourceschema.Attribute{
		"id": datasourceschema.Int64Attribute{Required: true}, "credential_id": datasourceschema.Int64Attribute{Required: true}, "project_id": datasourceschema.Int64Attribute{Computed: true}, "operations": datasourceschema.SetAttribute{Computed: true, ElementType: types.StringType}, "expires_at": datasourceschema.StringAttribute{Computed: true}, "status": datasourceschema.StringAttribute{Computed: true}, "revision": datasourceschema.Int64Attribute{Computed: true},
	}}
}
func (r *exGlobalCredentialGrantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = exGlobalCredentialGrantResourceSchema()
}
func (d *exGlobalCredentialGrantDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = exGlobalCredentialGrantDataSourceSchema()
}

func exGlobalCredentialGrantOperations(ctx context.Context, operations types.Set) (int64, error) {
	var values []string
	if d := operations.ElementsAs(ctx, &values, false); d.HasError() {
		return 0, fmt.Errorf("could not read configured grant operations")
	}
	var mask int64
	for _, value := range values {
		switch value {
		case "reference":
			mask |= 1
		case "consume":
			mask |= 2
		default:
			return 0, fmt.Errorf("grant operation %q is not supported; use reference or consume", value)
		}
	}
	if mask == 0 {
		return 0, fmt.Errorf("at least one grant operation is required")
	}
	return mask, nil
}
func exGlobalCredentialGrantOperationSet(ctx context.Context, mask int64) (types.Set, error) {
	values := []string{}
	if mask&1 != 0 {
		values = append(values, "reference")
	}
	if mask&2 != 0 {
		values = append(values, "consume")
	}
	if mask&^int64(3) != 0 || len(values) == 0 {
		return types.SetNull(types.StringType), fmt.Errorf("server grant contains unsupported operations")
	}
	result, d := types.SetValueFrom(ctx, types.StringType, values)
	if d.HasError() {
		return types.SetNull(types.StringType), fmt.Errorf("could not store grant operations")
	}
	return result, nil
}
func exGlobalCredentialGrantModelFromResponse(ctx context.Context, old exGlobalCredentialGrantModel, raw exGlobalCredentialGrantResponse) (exGlobalCredentialGrantModel, error) {
	operations, err := exGlobalCredentialGrantOperationSet(ctx, raw.Operations)
	if err != nil {
		return old, err
	}
	next := old
	next.ID = types.Int64Value(raw.ID)
	next.CredentialID = types.Int64Value(raw.CredentialID)
	next.ProjectID = types.Int64Value(raw.ProjectID)
	next.Operations = operations
	next.Status = types.StringValue(raw.Status)
	next.Revision = types.Int64Value(raw.Revision)
	if raw.ExpiresAt == nil {
		next.ExpiresAt = types.StringNull()
	} else {
		next.ExpiresAt = types.StringValue(*raw.ExpiresAt)
	}
	return next, nil
}
func exGlobalCredentialGrantParams(m exGlobalCredentialGrantModel) map[string]string {
	return map[string]string{"credential_id": strconv.FormatInt(m.CredentialID.ValueInt64(), 10), "grant_id": strconv.FormatInt(m.ID.ValueInt64(), 10)}
}
func exGlobalCredentialGrantRead(ctx context.Context, c *apiclient.SemaphoreUI, old exGlobalCredentialGrantModel) (exGlobalCredentialGrantModel, error) {
	var grants []exGlobalCredentialGrantResponse
	if err := exRequest(ctx, c, http.MethodGet, "/global-credentials/{credential_id}/grants", map[string]string{"credential_id": strconv.FormatInt(old.CredentialID.ValueInt64(), 10)}, nil, &grants); err != nil {
		return old, err
	}
	for _, grant := range grants {
		if grant.ID == old.ID.ValueInt64() {
			return exGlobalCredentialGrantModelFromResponse(ctx, old, grant)
		}
	}
	return old, &exAPIError{StatusCode: http.StatusNotFound, method: http.MethodGet, route: "/global-credentials/{credential_id}/grants"}
}
func exGlobalCredentialGrantRequireRevision(m exGlobalCredentialGrantModel) error {
	if m.Revision.IsNull() || m.Revision.IsUnknown() || m.Revision.ValueInt64() <= 0 {
		return fmt.Errorf("refresh the global credential grant and retry; the provider will not overwrite a concurrent grant change")
	}
	return nil
}
func exGlobalCredentialGrantPayload(ctx context.Context, m exGlobalCredentialGrantModel, revision int64) (map[string]any, error) {
	operations, err := exGlobalCredentialGrantOperations(ctx, m.Operations)
	if err != nil {
		return nil, err
	}
	result := map[string]any{"project_id": m.ProjectID.ValueInt64(), "operations": operations}
	if revision > 0 {
		result["revision"] = revision
	}
	if !m.ExpiresAt.IsNull() && !m.ExpiresAt.IsUnknown() {
		result["expires_at"] = m.ExpiresAt.ValueString()
	}
	return result, nil
}

func (r *exGlobalCredentialGrantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan exGlobalCredentialGrantModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload, err := exGlobalCredentialGrantPayload(ctx, plan, 0)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Global Credential Grant", err.Error())
		return
	}
	var raw exGlobalCredentialGrantResponse
	if err = exRequest(ctx, r.client, http.MethodPost, "/global-credentials/{credential_id}/grants", map[string]string{"credential_id": strconv.FormatInt(plan.CredentialID.ValueInt64(), 10)}, payload, &raw); err != nil {
		resp.Diagnostics.AddError("Error Creating Semaphore EX Global Credential Grant", err.Error())
		return
	}
	state, err := exGlobalCredentialGrantModelFromResponse(ctx, plan, raw)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Global Credential Grant Response", err.Error())
		return
	}
	if plan.Status.ValueString() == "revoked" {
		if err = r.setStatus(ctx, state, "revoke", &state); err != nil {
			resp.Diagnostics.AddError("Error Revoking Semaphore EX Global Credential Grant", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
func (r *exGlobalCredentialGrantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state exGlobalCredentialGrantModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	next, err := exGlobalCredentialGrantRead(ctx, r.client, state)
	if exNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Semaphore EX Global Credential Grant", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}
func (r *exGlobalCredentialGrantResource) setStatus(ctx context.Context, state exGlobalCredentialGrantModel, action string, target *exGlobalCredentialGrantModel) error {
	var raw exGlobalCredentialGrantResponse
	if err := exRequest(ctx, r.client, http.MethodPost, "/global-credentials/{credential_id}/grants/{grant_id}/"+action, exGlobalCredentialGrantParams(state), map[string]any{"revision": state.Revision.ValueInt64()}, &raw); err != nil {
		return err
	}
	next, err := exGlobalCredentialGrantModelFromResponse(ctx, state, raw)
	if err == nil {
		*target = next
	}
	return err
}
func (r *exGlobalCredentialGrantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state exGlobalCredentialGrantModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := exGlobalCredentialGrantRequireRevision(state); err != nil {
		resp.Diagnostics.AddError("Global Credential Grant Revision Unavailable", err.Error())
		return
	}
	plan.ID = state.ID
	plan.CredentialID = state.CredentialID
	current := state
	changed := !plan.ProjectID.Equal(state.ProjectID) || !plan.Operations.Equal(state.Operations) || !plan.ExpiresAt.Equal(state.ExpiresAt)
	if changed {
		payload, err := exGlobalCredentialGrantPayload(ctx, plan, state.Revision.ValueInt64())
		if err != nil {
			resp.Diagnostics.AddError("Invalid Global Credential Grant", err.Error())
			return
		}
		var raw exGlobalCredentialGrantResponse
		if err = exRequest(ctx, r.client, http.MethodPut, "/global-credentials/{credential_id}/grants/{grant_id}", exGlobalCredentialGrantParams(state), payload, &raw); err != nil {
			resp.Diagnostics.AddError("Error Updating Semaphore EX Global Credential Grant", err.Error())
			return
		}
		current, err = exGlobalCredentialGrantModelFromResponse(ctx, plan, raw)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Global Credential Grant Response", err.Error())
			return
		}
	}
	if !plan.Status.Equal(state.Status) {
		action := "revoke"
		if plan.Status.ValueString() == "active" {
			action = "restore"
		} else if plan.Status.ValueString() != "revoked" {
			resp.Diagnostics.AddError("Invalid Global Credential Grant Status", "status must be active or revoked")
			return
		}
		if err := r.setStatus(ctx, current, action, &current); err != nil {
			resp.Diagnostics.AddError("Error Updating Semaphore EX Global Credential Grant Status", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &current)...)
}
func (r *exGlobalCredentialGrantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state exGlobalCredentialGrantModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := exGlobalCredentialGrantRequireRevision(state); err != nil {
		resp.Diagnostics.AddError("Global Credential Grant Revision Unavailable", err.Error())
		return
	}
	current := state
	if current.Status.ValueString() == "active" {
		if err := r.setStatus(ctx, current, "revoke", &current); err != nil {
			resp.Diagnostics.AddError("Error Revoking Semaphore EX Global Credential Grant Before Deletion", err.Error())
			return
		}
	}
	err := exRequestWithOptions(ctx, r.client, http.MethodDelete, "/global-credentials/{credential_id}/grants/{grant_id}", exRequestOptions{PathParams: exGlobalCredentialGrantParams(current), Query: map[string]string{"expected_revision": strconv.FormatInt(current.Revision.ValueInt64(), 10)}}, nil, nil)
	if err != nil && !exNotFound(err) {
		resp.Diagnostics.AddError("Error Removing Semaphore EX Global Credential Grant", err.Error())
	}
}
func (r *exGlobalCredentialGrantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 4 || parts[0] != "credential" || parts[2] != "grant" {
		resp.Diagnostics.AddError("Invalid Global Credential Grant Import ID", "Use credential/<credential_id>/grant/<grant_id>.")
		return
	}
	credentialID, credentialErr := strconv.ParseInt(parts[1], 10, 64)
	grantID, grantErr := strconv.ParseInt(parts[3], 10, 64)
	if credentialErr != nil || grantErr != nil || credentialID <= 0 || grantID <= 0 {
		resp.Diagnostics.AddError("Invalid Global Credential Grant Import ID", "Use positive numeric credential and grant IDs.")
		return
	}
	next, err := exGlobalCredentialGrantRead(ctx, r.client, exGlobalCredentialGrantModel{CredentialID: types.Int64Value(credentialID), ID: types.Int64Value(grantID)})
	if err != nil {
		resp.Diagnostics.AddError("Error Importing Semaphore EX Global Credential Grant", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}
func (d *exGlobalCredentialGrantDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config exGlobalCredentialGrantModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, err := exGlobalCredentialGrantRead(ctx, d.client, config)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Semaphore EX Global Credential Grant", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
