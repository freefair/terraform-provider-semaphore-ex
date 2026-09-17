package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rs "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// A deployment-window policy is one revision-fenced document per project. The
// API replaces its rules as a whole, so Terraform deliberately models them as
// an ordered collection without exposing the server-only rule IDs.
type exProjectDeploymentWindowModel struct {
	ID        types.Int64  `tfsdk:"id"`
	ProjectID types.Int64  `tfsdk:"project_id"`
	Revision  types.Int64  `tfsdk:"revision"`
	Timezone  types.String `tfsdk:"timezone"`
	Default   types.String `tfsdk:"default"`
	Rules     types.List   `tfsdk:"rules"`
}

type exProjectDeploymentWindowResource struct{ client *apiclient.SemaphoreUI }
type exProjectDeploymentWindowDataSource struct{ client *apiclient.SemaphoreUI }

var _ resource.ResourceWithConfigure = &exProjectDeploymentWindowResource{}
var _ resource.ResourceWithImportState = &exProjectDeploymentWindowResource{}
var _ datasource.DataSourceWithConfigure = &exProjectDeploymentWindowDataSource{}

func NewProjectDeploymentWindowResource() resource.Resource {
	return &exProjectDeploymentWindowResource{}
}
func NewProjectDeploymentWindowDataSource() datasource.DataSource {
	return &exProjectDeploymentWindowDataSource{}
}

func (r *exProjectDeploymentWindowResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_deployment_window"
}

func (d *exProjectDeploymentWindowDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_deployment_window"
}

func (r *exProjectDeploymentWindowResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = exDeploymentWindowClient(req.ProviderData, &resp.Diagnostics)
}

func (d *exProjectDeploymentWindowDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = exDeploymentWindowClient(req.ProviderData, &resp.Diagnostics)
}

func exDeploymentWindowClient(value any, diagnostics interface{ AddError(string, string) }) *apiclient.SemaphoreUI {
	if value == nil {
		return nil
	}
	client, ok := value.(*apiclient.SemaphoreUI)
	if !ok {
		diagnostics.AddError("Unexpected Deployment Window Configure Type", "Expected the configured Semaphore EX client.")
		return nil
	}
	return client
}

func (r *exProjectDeploymentWindowResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = exProjectDeploymentWindowResourceSchema()
}

func (d *exProjectDeploymentWindowDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = exProjectDeploymentWindowDataSourceSchema()
}

func exDeploymentWindowRuleTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name": types.StringType, "active": types.BoolType, "kind": types.StringType, "scope": types.StringType,
		"template_id": types.Int64Type, "workflow_id": types.Int64Type, "recurrence": types.StringType,
		"duration_minutes": types.Int64Type, "effective_from": types.StringType, "effective_until": types.StringType,
	}
}

func exDeploymentWindowRuleResourceAttributes() map[string]rs.Attribute {
	return map[string]rs.Attribute{
		"name": rs.StringAttribute{Required: true}, "active": rs.BoolAttribute{Required: true},
		"kind":        rs.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("allow", "freeze")}},
		"scope":       rs.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("project", "template", "workflow")}},
		"template_id": rs.Int64Attribute{Optional: true, Validators: []validator.Int64{int64validator.AtLeast(1)}},
		"workflow_id": rs.Int64Attribute{Optional: true, Validators: []validator.Int64{int64validator.AtLeast(1)}},
		"recurrence":  rs.StringAttribute{Required: true}, "duration_minutes": rs.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}},
		"effective_from": rs.StringAttribute{Optional: true}, "effective_until": rs.StringAttribute{Optional: true},
	}
}

func exDeploymentWindowRuleDataSourceAttributes() map[string]ds.Attribute {
	return map[string]ds.Attribute{
		"name": ds.StringAttribute{Computed: true}, "active": ds.BoolAttribute{Computed: true}, "kind": ds.StringAttribute{Computed: true}, "scope": ds.StringAttribute{Computed: true},
		"template_id": ds.Int64Attribute{Computed: true}, "workflow_id": ds.Int64Attribute{Computed: true}, "recurrence": ds.StringAttribute{Computed: true},
		"duration_minutes": ds.Int64Attribute{Computed: true}, "effective_from": ds.StringAttribute{Computed: true}, "effective_until": ds.StringAttribute{Computed: true},
	}
}

func exProjectDeploymentWindowResourceSchema() rs.Schema {
	return rs.Schema{MarkdownDescription: "Manages the revision-fenced deployment-window policy for one Semaphore EX project.", Attributes: map[string]rs.Attribute{
		"id":         rs.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
		"project_id": rs.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}},
		"revision":   rs.Int64Attribute{Computed: true}, "timezone": rs.StringAttribute{Required: true},
		"default": rs.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("allow", "deny")}},
		"rules":   rs.ListNestedAttribute{Required: true, NestedObject: rs.NestedAttributeObject{Attributes: exDeploymentWindowRuleResourceAttributes()}},
	}}
}

func exProjectDeploymentWindowDataSourceSchema() ds.Schema {
	return ds.Schema{MarkdownDescription: "Reads the deployment-window policy for one Semaphore EX project.", Attributes: map[string]ds.Attribute{
		"id": ds.Int64Attribute{Computed: true}, "project_id": ds.Int64Attribute{Required: true}, "revision": ds.Int64Attribute{Computed: true},
		"timezone": ds.StringAttribute{Computed: true}, "default": ds.StringAttribute{Computed: true},
		"rules": ds.ListNestedAttribute{Computed: true, NestedObject: ds.NestedAttributeObject{Attributes: exDeploymentWindowRuleDataSourceAttributes()}},
	}}
}

func exDeploymentWindowRoute() string { return "/project/{project_id}/deployment-windows" }

func exDeploymentWindowParams(projectID types.Int64) (map[string]string, error) {
	id, err := exPathID(projectID)
	if err != nil {
		return nil, err
	}
	return map[string]string{"project_id": id}, nil
}

func exDeploymentWindowBody(ctx context.Context, model exProjectDeploymentWindowModel, includeRevision bool) (map[string]any, error) {
	values := map[string]attr.Value{"timezone": model.Timezone, "default": model.Default, "rules": model.Rules}
	body := make(map[string]any, len(values)+1)
	for name, value := range values {
		wire, err := exWireValue(ctx, value)
		if err != nil {
			return nil, fmt.Errorf("%s is invalid: %w", name, err)
		}
		body[name] = wire
	}
	if includeRevision {
		if model.Revision.IsNull() || model.Revision.IsUnknown() || model.Revision.ValueInt64() < 1 {
			return nil, fmt.Errorf("refresh the deployment-window policy and retry; the provider will not overwrite a concurrent change")
		}
		body["revision"] = model.Revision.ValueInt64()
	}
	return body, nil
}

func exDeploymentWindowFromResponse(ctx context.Context, old exProjectDeploymentWindowModel, raw map[string]any) (exProjectDeploymentWindowModel, error) {
	value, err := exTypedValue(ctx, types.ObjectType{AttrTypes: map[string]attr.Type{
		"project_id": types.Int64Type, "revision": types.Int64Type, "timezone": types.StringType, "default": types.StringType,
		"rules": types.ListType{ElemType: types.ObjectType{AttrTypes: exDeploymentWindowRuleTypes()}},
	}}, raw)
	if err != nil {
		return old, err
	}
	objectValue, ok := value.(types.Object)
	if !ok {
		return old, fmt.Errorf("API response does not match the deployment-window schema")
	}
	object := objectValue.Attributes()
	projectID, ok := object["project_id"].(types.Int64)
	if !ok {
		return old, fmt.Errorf("API response has invalid deployment-window project identity")
	}
	if !old.ProjectID.IsNull() && !old.ProjectID.IsUnknown() && !old.ProjectID.Equal(projectID) {
		return old, fmt.Errorf("API returned a different project identity")
	}
	revision, revisionOK := object["revision"].(types.Int64)
	timezone, timezoneOK := object["timezone"].(types.String)
	decision, decisionOK := object["default"].(types.String)
	rules, rulesOK := object["rules"].(types.List)
	if !revisionOK || !timezoneOK || !decisionOK || !rulesOK {
		return old, fmt.Errorf("API response has invalid deployment-window fields")
	}
	return exProjectDeploymentWindowModel{ID: projectID, ProjectID: projectID, Revision: revision, Timezone: timezone, Default: decision, Rules: rules}, nil
}

func exReadProjectDeploymentWindow(ctx context.Context, client *apiclient.SemaphoreUI, old exProjectDeploymentWindowModel) (exProjectDeploymentWindowModel, error) {
	params, err := exDeploymentWindowParams(old.ProjectID)
	if err != nil {
		return old, err
	}
	var raw map[string]any
	if err = exRequest(ctx, client, http.MethodGet, exDeploymentWindowRoute(), params, nil, &raw); err != nil {
		return old, err
	}
	return exDeploymentWindowFromResponse(ctx, old, raw)
}

func (r *exProjectDeploymentWindowResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan exProjectDeploymentWindowModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	current, err := exReadProjectDeploymentWindow(ctx, r.client, plan)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Deployment Window Policy", err.Error())
		return
	}
	plan.Revision = current.Revision
	body, err := exDeploymentWindowBody(ctx, plan, true)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Deployment Window Policy", err.Error())
		return
	}
	params, _ := exDeploymentWindowParams(plan.ProjectID)
	var raw map[string]any
	if err = exRequest(ctx, r.client, http.MethodPut, exDeploymentWindowRoute(), params, body, &raw); err != nil {
		resp.Diagnostics.AddError("Error Creating Semaphore EX Deployment Window Policy", err.Error())
		return
	}
	next, err := exDeploymentWindowFromResponse(ctx, plan, raw)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Deployment Window Policy Response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *exProjectDeploymentWindowResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state exProjectDeploymentWindowModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	next, err := exReadProjectDeploymentWindow(ctx, r.client, state)
	if exNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Semaphore EX Deployment Window Policy", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *exProjectDeploymentWindowResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state exProjectDeploymentWindowModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID, plan.ProjectID, plan.Revision = state.ID, state.ProjectID, state.Revision
	body, err := exDeploymentWindowBody(ctx, plan, true)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Deployment Window Policy", err.Error())
		return
	}
	params, _ := exDeploymentWindowParams(plan.ProjectID)
	var raw map[string]any
	if err = exRequest(ctx, r.client, http.MethodPut, exDeploymentWindowRoute(), params, body, &raw); err != nil {
		resp.Diagnostics.AddError("Error Updating Semaphore EX Deployment Window Policy", err.Error())
		return
	}
	next, err := exDeploymentWindowFromResponse(ctx, plan, raw)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Deployment Window Policy Response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *exProjectDeploymentWindowResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state exProjectDeploymentWindowModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if state.Revision.IsNull() || state.Revision.IsUnknown() || state.Revision.ValueInt64() < 1 {
		resp.Diagnostics.AddError("Deployment Window Policy Revision Unavailable", "Refresh the deployment-window policy and retry; the provider will not overwrite a concurrent change.")
		return
	}
	params, err := exDeploymentWindowParams(state.ProjectID)
	if err == nil {
		err = exRequestWithOptions(ctx, r.client, http.MethodDelete, exDeploymentWindowRoute(), exRequestOptions{PathParams: params, Query: map[string]string{"expected_revision": strconv.FormatInt(state.Revision.ValueInt64(), 10)}}, nil, nil)
	}
	if err != nil && !exNotFound(err) {
		resp.Diagnostics.AddError("Error Resetting Semaphore EX Deployment Window Policy", err.Error())
	}
}

func (r *exProjectDeploymentWindowResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	projectID, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil || projectID < 1 {
		resp.Diagnostics.AddError("Invalid Deployment Window Policy Import ID", "Use the positive numeric project ID.")
		return
	}
	next, err := exReadProjectDeploymentWindow(ctx, r.client, exProjectDeploymentWindowModel{ProjectID: types.Int64Value(projectID)})
	if err != nil {
		resp.Diagnostics.AddError("Error Importing Semaphore EX Deployment Window Policy", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (d *exProjectDeploymentWindowDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config exProjectDeploymentWindowModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	next, err := exReadProjectDeploymentWindow(ctx, d.client, config)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Semaphore EX Deployment Window Policy", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}
