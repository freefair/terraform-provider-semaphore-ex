package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type projectWorkflowRestoreAction struct{ client *apiclient.SemaphoreUI }

func NewProjectWorkflowRestoreAction() action.Action { return &projectWorkflowRestoreAction{} }
func (a *projectWorkflowRestoreAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_workflow_restore"
}
func (a *projectWorkflowRestoreAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Invalid Provider Client", "Expected the configured Semaphore EX client.")
		return
	}
	a.client = client
}
func (a *projectWorkflowRestoreAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	positive := []validator.Int64{int64validator.AtLeast(1)}
	resp.Schema = schema.Schema{MarkdownDescription: "Restores a workflow snapshot as a new server-owned revision. Invoke explicitly: this replaces the current graph and may cause drift in a managed workflow resource. No workflow run is started. The restore endpoint does not accept an expected revision.", Attributes: map[string]schema.Attribute{
		"project_id":     schema.Int64Attribute{Required: true, Validators: positive},
		"workflow_id":    schema.Int64Attribute{Required: true, Validators: positive},
		"version_number": schema.Int64Attribute{Required: true, Validators: positive},
		"message":        schema.StringAttribute{Optional: true},
	}}
}
func (a *projectWorkflowRestoreAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var model struct {
		ProjectID     types.Int64  `tfsdk:"project_id"`
		WorkflowID    types.Int64  `tfsdk:"workflow_id"`
		VersionNumber types.Int64  `tfsdk:"version_number"`
		Message       types.String `tfsdk:"message"`
	}
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	params := map[string]string{}
	for name, value := range map[string]types.Int64{"project_id": model.ProjectID, "workflow_id": model.WorkflowID, "version_number": model.VersionNumber} {
		id, err := exPathID(value)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Workflow Version Identity", err.Error())
			return
		}
		params[name] = id
	}
	if model.Message.IsUnknown() {
		resp.Diagnostics.AddError("Unknown Restore Message", "message must be known before invoking restore.")
		return
	}
	var response struct {
		ID       int64 `json:"id"`
		Revision int64 `json:"revision"`
	}
	if err := exRequest(ctx, a.client, http.MethodPost, "/project/{project_id}/workflows/{workflow_id}/versions/{version_number}/restore", params, map[string]any{"message": model.Message.ValueString()}, &response); err != nil {
		resp.Diagnostics.AddError("Error Restoring Workflow Version", err.Error())
		return
	}
	if response.ID != model.WorkflowID.ValueInt64() || response.Revision <= 0 {
		resp.Diagnostics.AddError("Invalid Restore Response", "Server did not return the restored workflow identity and revision.")
		return
	}
	if resp.SendProgress != nil {
		resp.SendProgress(action.InvokeProgressEvent{Message: fmt.Sprintf("Workflow %s restored as revision %s.", params["workflow_id"], strconv.FormatInt(response.Revision, 10))})
	}
}
