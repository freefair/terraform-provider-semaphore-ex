package provider

import (
	"context"
	"net/http"
	"strconv"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type exWorkflowTriggerSigningAction struct {
	client    *apiclient.SemaphoreUI
	operation string
}

func NewWorkflowTriggerSigningPromoteAction() action.Action {
	return &exWorkflowTriggerSigningAction{operation: "promote"}
}
func NewWorkflowTriggerSigningRevokeAction() action.Action {
	return &exWorkflowTriggerSigningAction{operation: "revoke"}
}
func (a *exWorkflowTriggerSigningAction) Metadata(_ context.Context, q action.MetadataRequest, p *action.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_workflow_trigger_signing_" + a.operation
}
func (a *exWorkflowTriggerSigningAction) Configure(_ context.Context, q action.ConfigureRequest, p *action.ConfigureResponse) {
	a.client = exWorkflowTriggerClient(q.ProviderData, &p.Diagnostics)
}
func (a *exWorkflowTriggerSigningAction) Schema(_ context.Context, _ action.SchemaRequest, p *action.SchemaResponse) {
	p.Schema = schema.Schema{MarkdownDescription: "Explicitly " + a.operation + "s a staged webhook signing key using the supplied revision. It never returns secret material.", Attributes: map[string]schema.Attribute{"project_id": schema.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}}, "workflow_id": schema.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}}, "trigger_id": schema.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}}, "expected_revision": schema.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}}}}
}
func (a *exWorkflowTriggerSigningAction) Invoke(ctx context.Context, q action.InvokeRequest, p *action.InvokeResponse) {
	var c struct {
		ProjectID        types.Int64 `tfsdk:"project_id"`
		WorkflowID       types.Int64 `tfsdk:"workflow_id"`
		TriggerID        types.Int64 `tfsdk:"trigger_id"`
		ExpectedRevision types.Int64 `tfsdk:"expected_revision"`
	}
	p.Diagnostics.Append(q.Config.Get(ctx, &c)...)
	if p.Diagnostics.HasError() {
		return
	}
	for name, value := range map[string]types.Int64{"project_id": c.ProjectID, "workflow_id": c.WorkflowID, "trigger_id": c.TriggerID, "expected_revision": c.ExpectedRevision} {
		if value.IsNull() || value.IsUnknown() || value.ValueInt64() < 1 {
			p.Diagnostics.AddError("Invalid Workflow Trigger Signing Input", name+" must be a known positive integer.")
			return
		}
	}
	params := map[string]string{"project_id": strconv.FormatInt(c.ProjectID.ValueInt64(), 10), "workflow_id": strconv.FormatInt(c.WorkflowID.ValueInt64(), 10), "trigger_id": strconv.FormatInt(c.TriggerID.ValueInt64(), 10)}
	if err := exRequest(ctx, a.client, http.MethodPost, exWorkflowTriggerRoute(true)+"/webhook-signing/"+a.operation, params, map[string]any{"revision": c.ExpectedRevision.ValueInt64()}, nil); err != nil {
		p.Diagnostics.AddError("Error "+a.operation+"ing Semaphore EX Workflow Trigger Signing Key", err.Error())
	}
}
