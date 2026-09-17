package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type projectTemplateInventoryResource struct{ client *apiclient.SemaphoreUI }

func NewProjectTemplateInventoryResource() resource.Resource {
	return &projectTemplateInventoryResource{}
}
func (r *projectTemplateInventoryResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_template_inventory"
}
func (r *projectTemplateInventoryResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = ProjectTemplateInventorySchema().GetResource(ctx)
}
func (r *projectTemplateInventoryResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Invalid Provider Client", "Expected the configured Semaphore EX client.")
		return
	}
	r.client = client
}
func templateInventoryParams(m projectTemplateInventoryModel) map[string]string {
	return map[string]string{"project_id": strconv.FormatInt(m.ProjectID.ValueInt64(), 10), "template_id": strconv.FormatInt(m.TemplateID.ValueInt64(), 10), "inventory_id": strconv.FormatInt(m.InventoryID.ValueInt64(), 10)}
}
func templateInventoryIdentity(m projectTemplateInventoryModel) string {
	return fmt.Sprintf("project/%d/template/%d/inventory/%d", m.ProjectID.ValueInt64(), m.TemplateID.ValueInt64(), m.InventoryID.ValueInt64())
}
func templateInventoryAttached(ctx context.Context, client *apiclient.SemaphoreUI, m projectTemplateInventoryModel) (bool, error) {
	var inventory struct {
		ID         int64  `json:"id"`
		ProjectID  int64  `json:"project_id"`
		TemplateID *int64 `json:"template_id"`
	}
	err := exRequest(ctx, client, http.MethodGet, "/project/{project_id}/inventory/{inventory_id}", templateInventoryParams(m), nil, &inventory)
	if exNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if inventory.ID != m.InventoryID.ValueInt64() || inventory.ProjectID != m.ProjectID.ValueInt64() {
		return false, fmt.Errorf("API returned a different inventory identity")
	}
	return inventory.TemplateID != nil && *inventory.TemplateID == m.TemplateID.ValueInt64(), nil
}
func (r *projectTemplateInventoryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectTemplateInventoryModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := exRequest(ctx, r.client, http.MethodPost, "/project/{project_id}/templates/{template_id}/inventory/{inventory_id}/attach", templateInventoryParams(plan), nil, nil); err != nil {
		resp.Diagnostics.AddError("Error Attaching Inventory", err.Error())
		return
	}
	plan.ID = types.StringValue(templateInventoryIdentity(plan))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *projectTemplateInventoryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectTemplateInventoryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	attached, err := templateInventoryAttached(ctx, r.client, state)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Inventory Attachment", err.Error())
		return
	}
	if !attached {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
func (r *projectTemplateInventoryResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unexpected Attachment Update", "Changing any association identity requires replacement.")
}
func (r *projectTemplateInventoryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectTemplateInventoryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	attached, err := templateInventoryAttached(ctx, r.client, state)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Inventory Attachment", err.Error())
		return
	}
	if !attached {
		return
	}
	if err := exRequest(ctx, r.client, http.MethodPost, "/project/{project_id}/templates/{template_id}/inventory/{inventory_id}/detach", templateInventoryParams(state), nil, nil); err != nil && !exNotFound(err) {
		resp.Diagnostics.AddError("Error Detaching Inventory", err.Error())
	}
}
func (r *projectTemplateInventoryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 6 || parts[0] != "project" || parts[2] != "template" || parts[4] != "inventory" {
		resp.Diagnostics.AddError("Invalid Import ID", "Expected project/<id>/template/<id>/inventory/<id>.")
		return
	}
	ids := make([]int64, 3)
	for i := range ids {
		value, err := strconv.ParseInt(parts[2*i+1], 10, 64)
		if err != nil || value <= 0 {
			resp.Diagnostics.AddError("Invalid Import ID", "All identifiers must be positive integers.")
			return
		}
		ids[i] = value
	}
	state := projectTemplateInventoryModel{ProjectID: types.Int64Value(ids[0]), TemplateID: types.Int64Value(ids[1]), InventoryID: types.Int64Value(ids[2])}
	attached, err := templateInventoryAttached(ctx, r.client, state)
	if err != nil {
		resp.Diagnostics.AddError("Error Importing Inventory Attachment", err.Error())
		return
	}
	if !attached {
		resp.Diagnostics.AddError("Inventory Is Not Attached", "The requested inventory is not attached to the specified template.")
		return
	}
	state.ID = types.StringValue(templateInventoryIdentity(state))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
