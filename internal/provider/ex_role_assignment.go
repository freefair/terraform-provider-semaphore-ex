package provider

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type globalRoleAssignmentModel struct {
	ID       types.Int64  `tfsdk:"id"`
	UserID   types.Int64  `tfsdk:"user_id"`
	RoleID   types.String `tfsdk:"role_id"`
	Revision types.Int64  `tfsdk:"revision"`
}
type globalRoleAssignmentResponse struct {
	ID       int64  `json:"id"`
	UserID   int64  `json:"user_id"`
	RoleID   string `json:"role_id"`
	Revision int64  `json:"revision"`
}
type globalRoleAssignmentResource struct{ client *apiclient.SemaphoreUI }
type globalRoleAssignmentDataSource struct{ client *apiclient.SemaphoreUI }

func NewGlobalRoleAssignmentResource() resource.Resource { return &globalRoleAssignmentResource{} }
func NewGlobalRoleAssignmentDataSource() datasource.DataSource {
	return &globalRoleAssignmentDataSource{}
}
func (r *globalRoleAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_global_role_assignment"
}
func (r *globalRoleAssignmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *globalRoleAssignmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = globalRoleAssignmentResourceSchema()
}
func (d *globalRoleAssignmentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_global_role_assignment"
}
func (d *globalRoleAssignmentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *globalRoleAssignmentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = globalRoleAssignmentDataSourceSchema()
}
func globalRoleAssignmentResourceSchema() resourceschema.Schema {
	return resourceschema.Schema{MarkdownDescription: "Assigns a global custom role to a user.", Attributes: map[string]resourceschema.Attribute{"id": resourceschema.Int64Attribute{MarkdownDescription: "Server-generated assignment ID.", Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}}, "user_id": resourceschema.Int64Attribute{MarkdownDescription: "Assigned user ID.", Required: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}}, "role_id": resourceschema.StringAttribute{MarkdownDescription: "Opaque global role ID.", Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}}, "revision": resourceschema.Int64Attribute{MarkdownDescription: "Server revision required for deletion.", Computed: true}}}
}
func globalRoleAssignmentDataSourceSchema() datasourceschema.Schema {
	return datasourceschema.Schema{MarkdownDescription: "Reads a global role assignment.", Attributes: map[string]datasourceschema.Attribute{"id": datasourceschema.Int64Attribute{Required: true}, "user_id": datasourceschema.Int64Attribute{Required: true}, "role_id": datasourceschema.StringAttribute{Computed: true}, "revision": datasourceschema.Int64Attribute{Computed: true}}}
}
func assignmentModel(v globalRoleAssignmentResponse) globalRoleAssignmentModel {
	return globalRoleAssignmentModel{ID: types.Int64Value(v.ID), UserID: types.Int64Value(v.UserID), RoleID: types.StringValue(v.RoleID), Revision: types.Int64Value(v.Revision)}
}
func assignmentCollectionRoute(userID int64) string { return "/users/{user_id}/global-roles" }
func assignmentOptions(userID int64) exRequestOptions {
	return exRequestOptions{PathParams: map[string]string{"user_id": strconv.FormatInt(userID, 10)}}
}
func readAssignment(ctx context.Context, c *apiclient.SemaphoreUI, userID, id int64) (globalRoleAssignmentModel, error) {
	var values []globalRoleAssignmentResponse
	if err := exRequestWithOptions(ctx, c, http.MethodGet, assignmentCollectionRoute(userID), assignmentOptions(userID), nil, &values); err != nil {
		return globalRoleAssignmentModel{}, err
	}
	for _, v := range values {
		if v.ID == id {
			return assignmentModel(v), nil
		}
	}
	return globalRoleAssignmentModel{}, &exAPIError{StatusCode: http.StatusNotFound, method: http.MethodGet, route: assignmentCollectionRoute(userID)}
}
func (r *globalRoleAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan globalRoleAssignmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var created globalRoleAssignmentResponse
	err := exRequestWithOptions(ctx, r.client, http.MethodPost, assignmentCollectionRoute(plan.UserID.ValueInt64()), assignmentOptions(plan.UserID.ValueInt64()), map[string]string{"role_id": plan.RoleID.ValueString()}, &created)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Global Role Assignment", err.Error())
		return
	}
	state := assignmentModel(created)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
func (r *globalRoleAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state globalRoleAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	next, err := readAssignment(ctx, r.client, state.UserID.ValueInt64(), state.ID.ValueInt64())
	if exNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Global Role Assignment", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}
func (r *globalRoleAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state globalRoleAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if state.Revision.IsNull() || state.Revision.IsUnknown() || state.Revision.ValueInt64() <= 0 {
		resp.Diagnostics.AddError("Assignment Revision Unavailable", "Refresh the assignment and retry; the provider will not delete without its prior revision.")
		return
	}
	o := assignmentOptions(state.UserID.ValueInt64())
	o.PathParams["assignment_id"] = strconv.FormatInt(state.ID.ValueInt64(), 10)
	o.Query = map[string]string{"revision": strconv.FormatInt(state.Revision.ValueInt64(), 10)}
	err := exRequestWithOptions(ctx, r.client, http.MethodDelete, "/users/{user_id}/global-roles/{assignment_id}", o, nil, nil)
	if err != nil && !exNotFound(err) {
		resp.Diagnostics.AddError("Error Removing Global Role Assignment", err.Error())
	}
}
func (r *globalRoleAssignmentResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Global Role Assignment Cannot Be Updated", "Changing its user or role requires replacement.")
}
func (r *globalRoleAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 4 || parts[0] != "user" || parts[2] != "assignment" {
		resp.Diagnostics.AddError("Invalid Global Role Assignment Import ID", "Use user/<user_id>/assignment/<assignment_id>.")
		return
	}
	userID, e1 := strconv.ParseInt(parts[1], 10, 64)
	id, e2 := strconv.ParseInt(parts[3], 10, 64)
	if e1 != nil || e2 != nil || userID <= 0 || id <= 0 {
		resp.Diagnostics.AddError("Invalid Global Role Assignment Import ID", "Use positive numeric user and assignment IDs.")
		return
	}
	state, err := readAssignment(ctx, r.client, userID, id)
	if err != nil {
		resp.Diagnostics.AddError("Error Importing Global Role Assignment", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
func (d *globalRoleAssignmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config globalRoleAssignmentModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, err := readAssignment(ctx, d.client, config.UserID.ValueInt64(), config.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Global Role Assignment", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
