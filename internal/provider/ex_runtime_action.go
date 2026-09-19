package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	runtimePreflightFingerprintHeader = "X-Semaphore-Preflight-Fingerprint"
	runtimePreflightTokenHeader       = "X-Semaphore-Preflight-Token"
)

// exRuntimeAction invokes one bounded Semaphore EX runtime endpoint. Runtime
// actions deliberately do not model task or workflow lifecycle state.
type exRuntimeAction struct {
	client *apiclient.SemaphoreUI
	kind   exRuntimeActionKind
}

type exRuntimeActionKind string

const (
	exRuntimeTaskStart        exRuntimeActionKind = "project_task_start"
	exRuntimeTaskStop         exRuntimeActionKind = "project_task_stop"
	exRuntimeWorkflowStart    exRuntimeActionKind = "project_workflow_start"
	exRuntimeWorkflowStop     exRuntimeActionKind = "project_workflow_stop"
	exRuntimeWorkflowApproval exRuntimeActionKind = "project_workflow_approval"
)

func NewProjectTaskStartAction() action.Action {
	return &exRuntimeAction{kind: exRuntimeTaskStart}
}

func NewProjectTaskStopAction() action.Action {
	return &exRuntimeAction{kind: exRuntimeTaskStop}
}

func NewProjectWorkflowStartAction() action.Action {
	return &exRuntimeAction{kind: exRuntimeWorkflowStart}
}

func NewProjectWorkflowStopAction() action.Action {
	return &exRuntimeAction{kind: exRuntimeWorkflowStop}
}

func NewProjectWorkflowApprovalAction() action.Action {
	return &exRuntimeAction{kind: exRuntimeWorkflowApproval}
}

func (a *exRuntimeAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + string(a.kind)
}

func (a *exRuntimeAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Runtime Action Configure Type", "Expected the configured Semaphore EX client.")
		return
	}
	a.client = client
}

func (a *exRuntimeAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: a.description(), Attributes: a.attributes()}
}

func (a *exRuntimeAction) description() string {
	switch a.kind {
	case exRuntimeTaskStart:
		return "Starts one task explicitly. The action returns after the server queues the task; it does not wait for task completion. Planning, refresh, and schema discovery never start work."
	case exRuntimeTaskStop:
		return "Requests that one task stop explicitly. The action returns after the stop request is accepted; it does not wait for task termination."
	case exRuntimeWorkflowStart:
		return "Starts one workflow run explicitly. The action returns after the server creates the run; it does not wait for workflow completion. Planning, refresh, and schema discovery never start work."
	case exRuntimeWorkflowStop:
		return "Requests that one workflow run stop explicitly. The action returns after the stop request is accepted; it does not wait for workflow termination."
	case exRuntimeWorkflowApproval:
		return "Submits one bounded workflow approval decision explicitly. The server evaluates the immutable approval snapshot and current eligibility; this action does not read or bypass approval policy."
	default:
		return "Invokes a bounded Semaphore EX runtime operation explicitly."
	}
}

func runtimeProjectAttribute() schema.Int64Attribute {
	return schema.Int64Attribute{Required: true, MarkdownDescription: "Project owning the target.", Validators: []validator.Int64{int64validator.AtLeast(1)}}
}

func runtimeIDAttribute(description string) schema.Int64Attribute {
	return schema.Int64Attribute{Required: true, MarkdownDescription: description, Validators: []validator.Int64{int64validator.AtLeast(1)}}
}

func (a *exRuntimeAction) attributes() map[string]schema.Attribute {
	switch a.kind {
	case exRuntimeTaskStart:
		return map[string]schema.Attribute{
			"project_id":            runtimeProjectAttribute(),
			"template_id":           schema.Int64Attribute{Optional: true, MarkdownDescription: "Template to run. Set template_id or template_name."},
			"template_name":         schema.StringAttribute{Optional: true, MarkdownDescription: "Template name to run when template_id is omitted."},
			"playbook":              schema.StringAttribute{Optional: true, MarkdownDescription: "Optional playbook override."},
			"environment":           schema.DynamicAttribute{Optional: true, MarkdownDescription: "Optional native HCL environment object. It is JSON-encoded because the Semaphore task API stores environment overrides as a JSON string."},
			"arguments":             schema.DynamicAttribute{Optional: true, MarkdownDescription: "Optional native HCL list or object of command-line arguments. It is JSON-encoded because the Semaphore task API expects an arguments JSON string."},
			"git_branch":            schema.StringAttribute{Optional: true, MarkdownDescription: "Optional repository branch override."},
			"inventory_id":          schema.Int64Attribute{Optional: true, MarkdownDescription: "Optional inventory override.", Validators: []validator.Int64{int64validator.AtLeast(1)}},
			"ssh_keys":              runtimeSSHKeyBindingsAttribute(),
			"message":               schema.StringAttribute{Optional: true, MarkdownDescription: "Optional task message."},
			"preflight_fingerprint": schema.StringAttribute{Optional: true, MarkdownDescription: "Exact fingerprint from a separately performed task preflight. The provider never obtains one automatically."},
			"preflight_token":       schema.StringAttribute{Optional: true, WriteOnly: true, MarkdownDescription: "Write-only review token paired with preflight_fingerprint. Supply it from an ephemeral input; the provider never reads or returns it."},
		}
	case exRuntimeTaskStop:
		return map[string]schema.Attribute{
			"project_id": runtimeProjectAttribute(), "task_id": runtimeIDAttribute("Task to stop."),
			"force": schema.BoolAttribute{Optional: true, MarkdownDescription: "Request immediate force-stop."},
		}
	case exRuntimeWorkflowStart:
		return map[string]schema.Attribute{
			"project_id": runtimeProjectAttribute(), "workflow_id": runtimeIDAttribute("Workflow to run."),
			"parameters":            schema.DynamicAttribute{Optional: true, MarkdownDescription: "Optional native HCL object of declared workflow parameter values. Numbers and booleans retain their JSON types."},
			"node_overrides":        runtimeNodeOverridesAttribute(),
			"preflight_fingerprint": schema.StringAttribute{Optional: true, MarkdownDescription: "Exact fingerprint from a separately performed workflow preflight. The provider never obtains one automatically."},
			"preflight_token":       schema.StringAttribute{Optional: true, WriteOnly: true, MarkdownDescription: "Write-only review token paired with preflight_fingerprint. Supply it from an ephemeral input; the provider never reads or returns it."},
		}
	case exRuntimeWorkflowStop:
		return map[string]schema.Attribute{
			"project_id": runtimeProjectAttribute(), "workflow_id": runtimeIDAttribute("Workflow owning the run."), "run_id": runtimeIDAttribute("Workflow run to stop."),
		}
	case exRuntimeWorkflowApproval:
		return map[string]schema.Attribute{
			"project_id": runtimeProjectAttribute(), "workflow_id": runtimeIDAttribute("Workflow owning the approval."), "run_id": runtimeIDAttribute("Workflow run owning the approval."), "node_id": runtimeIDAttribute("Approval node to decide."),
			"status":  schema.StringAttribute{Required: true, MarkdownDescription: "Decision status.", Validators: []validator.String{stringvalidator.OneOf("approved", "rejected")}},
			"comment": schema.StringAttribute{Optional: true, MarkdownDescription: "Optional approval comment."},
		}
	default:
		return nil
	}
}

func runtimeSSHKeyBindingsAttribute() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{Optional: true, MarkdownDescription: "Optional explicit SSH key selection. Omit to inherit; use an empty list to select no non-always keys. The server validates key ownership and any required host routing.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
		"access_key_id": schema.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}},
		"hosts":         schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Optional exact lowercase DNS or IP host mapping."},
	}}}
}

func runtimeNodeOverridesAttribute() schema.MapNestedAttribute {
	return schema.MapNestedAttribute{Optional: true, MarkdownDescription: "Optional overrides keyed by workflow node ID. Each object follows the server's bounded workflow-node override contract.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
		"inventory_id":    schema.Int64Attribute{Optional: true, Validators: []validator.Int64{int64validator.AtLeast(1)}},
		"environment_ids": schema.ListAttribute{Optional: true, ElementType: types.Int64Type},
		"arguments":       schema.StringAttribute{Optional: true, MarkdownDescription: "JSON-encoded task argument array or object."},
		"git_branch":      schema.StringAttribute{Optional: true},
	}}}
}

func (a *exRuntimeAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	switch a.kind {
	case exRuntimeTaskStart:
		a.invokeTaskStart(ctx, req, resp)
	case exRuntimeTaskStop:
		a.invokeTaskStop(ctx, req, resp)
	case exRuntimeWorkflowStart:
		a.invokeWorkflowStart(ctx, req, resp)
	case exRuntimeWorkflowStop:
		a.invokeWorkflowStop(ctx, req, resp)
	case exRuntimeWorkflowApproval:
		a.invokeWorkflowApproval(ctx, req, resp)
	default:
		resp.Diagnostics.AddError("Unsupported Runtime Action", "The action has no configured runtime operation.")
	}
}

type exRuntimeTaskStartModel struct {
	ProjectID            types.Int64   `tfsdk:"project_id"`
	TemplateID           types.Int64   `tfsdk:"template_id"`
	TemplateName         types.String  `tfsdk:"template_name"`
	Playbook             types.String  `tfsdk:"playbook"`
	Environment          types.Dynamic `tfsdk:"environment"`
	Arguments            types.Dynamic `tfsdk:"arguments"`
	GitBranch            types.String  `tfsdk:"git_branch"`
	InventoryID          types.Int64   `tfsdk:"inventory_id"`
	SSHKeys              types.List    `tfsdk:"ssh_keys"`
	Message              types.String  `tfsdk:"message"`
	PreflightFingerprint types.String  `tfsdk:"preflight_fingerprint"`
	PreflightToken       types.String  `tfsdk:"preflight_token"`
}

func (a *exRuntimeAction) invokeTaskStart(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config exRuntimeTaskStartModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || !runtimePositive(resp, "project_id", config.ProjectID) {
		return
	}
	if config.TemplateID.IsUnknown() || config.TemplateName.IsUnknown() || (config.TemplateID.IsNull() && config.TemplateName.IsNull()) || (!config.TemplateID.IsNull() && config.TemplateID.ValueInt64() < 1) {
		resp.Diagnostics.AddError("Invalid Task Template", "Set template_id to a known positive integer or set template_name.")
		return
	}
	body := map[string]any{}
	if !config.TemplateID.IsNull() {
		body["template_id"] = config.TemplateID.ValueInt64()
	}
	if !config.TemplateName.IsNull() {
		body["template_name"] = config.TemplateName.ValueString()
	}
	for key, value := range map[string]types.String{"playbook": config.Playbook, "git_branch": config.GitBranch, "message": config.Message} {
		if value.IsUnknown() {
			resp.Diagnostics.AddError("Unknown Task Input", key+" must be known before invocation.")
			return
		}
		if !value.IsNull() {
			body[key] = value.ValueString()
		}
	}
	if !config.InventoryID.IsNull() {
		if !runtimePositive(resp, "inventory_id", config.InventoryID) {
			return
		}
		body["inventory_id"] = config.InventoryID.ValueInt64()
	}
	if !config.SSHKeys.IsNull() {
		if config.SSHKeys.IsUnknown() {
			resp.Diagnostics.AddError("Unknown Task SSH Key Selection", "ssh_keys must be known before invocation.")
			return
		}
		sshKeys, err := exWireValue(ctx, config.SSHKeys)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Task SSH Key Selection", err.Error())
			return
		}
		body["ssh_keys"] = sshKeys
	}
	for key, value := range map[string]types.Dynamic{"environment": config.Environment, "arguments": config.Arguments} {
		encoded, ok := runtimeJSONString(ctx, resp, key, value)
		if !ok {
			return
		}
		if encoded != nil {
			body[key] = *encoded
		}
	}
	headers, ok := runtimePreflightHeaders(resp, config.PreflightFingerprint, config.PreflightToken)
	if !ok {
		return
	}
	var created struct {
		ID json.Number `json:"id"`
	}
	if err := exRequestWithOptions(ctx, a.client, http.MethodPost, "/project/{project_id}/tasks", exRequestOptions{PathParams: map[string]string{"project_id": strconv.FormatInt(config.ProjectID.ValueInt64(), 10)}, Headers: headers}, body, &created); err != nil {
		resp.Diagnostics.AddError("Error Starting Semaphore EX Task", err.Error())
		return
	}
	runtimeProgress(resp, "Task start request accepted.")
}

type exRuntimeTaskStopModel struct {
	ProjectID types.Int64 `tfsdk:"project_id"`
	TaskID    types.Int64 `tfsdk:"task_id"`
	Force     types.Bool  `tfsdk:"force"`
}

func (a *exRuntimeAction) invokeTaskStop(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config exRuntimeTaskStopModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || !runtimePositive(resp, "project_id", config.ProjectID) || !runtimePositive(resp, "task_id", config.TaskID) {
		return
	}
	if config.Force.IsUnknown() {
		resp.Diagnostics.AddError("Unknown Task Stop Input", "force must be known before invocation.")
		return
	}
	body := map[string]any{}
	if !config.Force.IsNull() {
		body["force"] = config.Force.ValueBool()
	}
	if err := exRequest(ctx, a.client, http.MethodPost, "/project/{project_id}/tasks/{task_id}/stop", map[string]string{"project_id": strconv.FormatInt(config.ProjectID.ValueInt64(), 10), "task_id": strconv.FormatInt(config.TaskID.ValueInt64(), 10)}, body, nil); err != nil {
		resp.Diagnostics.AddError("Error Stopping Semaphore EX Task", err.Error())
		return
	}
	runtimeProgress(resp, "Task stop request accepted.")
}

type exRuntimeWorkflowStartModel struct {
	ProjectID            types.Int64   `tfsdk:"project_id"`
	WorkflowID           types.Int64   `tfsdk:"workflow_id"`
	Parameters           types.Dynamic `tfsdk:"parameters"`
	NodeOverrides        types.Map     `tfsdk:"node_overrides"`
	PreflightFingerprint types.String  `tfsdk:"preflight_fingerprint"`
	PreflightToken       types.String  `tfsdk:"preflight_token"`
}

func (a *exRuntimeAction) invokeWorkflowStart(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config exRuntimeWorkflowStartModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || !runtimePositive(resp, "project_id", config.ProjectID) || !runtimePositive(resp, "workflow_id", config.WorkflowID) {
		return
	}
	body := map[string]any{}
	if !config.Parameters.IsNull() {
		if config.Parameters.IsUnknown() {
			resp.Diagnostics.AddError("Unknown Workflow Parameters", "parameters must be known before invocation.")
			return
		}
		parameters, err := exWireValue(ctx, config.Parameters)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Workflow Parameters", err.Error())
			return
		}
		if _, ok := parameters.(map[string]any); !ok {
			resp.Diagnostics.AddError("Invalid Workflow Parameters", "parameters must be a native HCL object.")
			return
		}
		body["parameters"] = parameters
	}
	if !config.NodeOverrides.IsNull() {
		if config.NodeOverrides.IsUnknown() {
			resp.Diagnostics.AddError("Unknown Workflow Node Overrides", "node_overrides must be known before invocation.")
			return
		}
		overrides, err := exWireValue(ctx, config.NodeOverrides)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Workflow Node Overrides", err.Error())
			return
		}
		body["node_overrides"] = overrides
	}
	headers, ok := runtimePreflightHeaders(resp, config.PreflightFingerprint, config.PreflightToken)
	if !ok {
		return
	}
	var created struct {
		ID json.Number `json:"id"`
	}
	if err := exRequestWithOptions(ctx, a.client, http.MethodPost, "/project/{project_id}/workflows/{workflow_id}/run", exRequestOptions{PathParams: map[string]string{"project_id": strconv.FormatInt(config.ProjectID.ValueInt64(), 10), "workflow_id": strconv.FormatInt(config.WorkflowID.ValueInt64(), 10)}, Headers: headers}, body, &created); err != nil {
		resp.Diagnostics.AddError("Error Starting Semaphore EX Workflow", err.Error())
		return
	}
	runtimeProgress(resp, "Workflow start request accepted.")
}

type exRuntimeWorkflowStopModel struct {
	ProjectID  types.Int64 `tfsdk:"project_id"`
	WorkflowID types.Int64 `tfsdk:"workflow_id"`
	RunID      types.Int64 `tfsdk:"run_id"`
}

func (a *exRuntimeAction) invokeWorkflowStop(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config exRuntimeWorkflowStopModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || !runtimePositive(resp, "project_id", config.ProjectID) || !runtimePositive(resp, "workflow_id", config.WorkflowID) || !runtimePositive(resp, "run_id", config.RunID) {
		return
	}
	if err := exRequest(ctx, a.client, http.MethodPost, "/project/{project_id}/workflows/{workflow_id}/runs/{run_id}/stop", map[string]string{"project_id": strconv.FormatInt(config.ProjectID.ValueInt64(), 10), "workflow_id": strconv.FormatInt(config.WorkflowID.ValueInt64(), 10), "run_id": strconv.FormatInt(config.RunID.ValueInt64(), 10)}, nil, nil); err != nil {
		resp.Diagnostics.AddError("Error Stopping Semaphore EX Workflow", err.Error())
		return
	}
	runtimeProgress(resp, "Workflow stop request accepted.")
}

type exRuntimeWorkflowApprovalModel struct {
	ProjectID  types.Int64  `tfsdk:"project_id"`
	WorkflowID types.Int64  `tfsdk:"workflow_id"`
	RunID      types.Int64  `tfsdk:"run_id"`
	NodeID     types.Int64  `tfsdk:"node_id"`
	Status     types.String `tfsdk:"status"`
	Comment    types.String `tfsdk:"comment"`
}

func (a *exRuntimeAction) invokeWorkflowApproval(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config exRuntimeWorkflowApprovalModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || !runtimePositive(resp, "project_id", config.ProjectID) || !runtimePositive(resp, "workflow_id", config.WorkflowID) || !runtimePositive(resp, "run_id", config.RunID) || !runtimePositive(resp, "node_id", config.NodeID) {
		return
	}
	if config.Status.IsNull() || config.Status.IsUnknown() || (config.Status.ValueString() != "approved" && config.Status.ValueString() != "rejected") {
		resp.Diagnostics.AddError("Invalid Workflow Approval Status", "status must be approved or rejected.")
		return
	}
	if config.Comment.IsUnknown() {
		resp.Diagnostics.AddError("Unknown Workflow Approval Comment", "comment must be known before invocation.")
		return
	}
	body := map[string]any{"status": config.Status.ValueString(), "source": "user"}
	if !config.Comment.IsNull() {
		body["comment"] = config.Comment.ValueString()
	}
	if err := exRequest(ctx, a.client, http.MethodPost, "/project/{project_id}/workflows/{workflow_id}/runs/{run_id}/approvals/{node_id}", map[string]string{"project_id": strconv.FormatInt(config.ProjectID.ValueInt64(), 10), "workflow_id": strconv.FormatInt(config.WorkflowID.ValueInt64(), 10), "run_id": strconv.FormatInt(config.RunID.ValueInt64(), 10), "node_id": strconv.FormatInt(config.NodeID.ValueInt64(), 10)}, body, nil); err != nil {
		resp.Diagnostics.AddError("Error Resolving Semaphore EX Workflow Approval", err.Error())
		return
	}
	runtimeProgress(resp, "Workflow approval decision accepted.")
}

func runtimePositive(resp *action.InvokeResponse, name string, value types.Int64) bool {
	if value.IsNull() || value.IsUnknown() || value.ValueInt64() < 1 {
		resp.Diagnostics.AddError("Invalid Runtime Identifier", name+" must be a known positive integer.")
		return false
	}
	return true
}

func runtimeJSONString(ctx context.Context, resp *action.InvokeResponse, name string, value types.Dynamic) (*string, bool) {
	if value.IsNull() {
		return nil, true
	}
	if value.IsUnknown() {
		resp.Diagnostics.AddError("Unknown Task Input", name+" must be known before invocation.")
		return nil, false
	}
	wire, err := exWireValue(ctx, value)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Task Input", err.Error())
		return nil, false
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Task Input", fmt.Sprintf("could not JSON-encode %s", name))
		return nil, false
	}
	result := string(encoded)
	return &result, true
}

func runtimePreflightHeaders(resp *action.InvokeResponse, fingerprint, token types.String) (map[string]string, bool) {
	headers := map[string]string{}
	for name, value := range map[string]types.String{runtimePreflightFingerprintHeader: fingerprint, runtimePreflightTokenHeader: token} {
		if value.IsUnknown() {
			resp.Diagnostics.AddError("Unknown Execution Preflight Input", name+" must be known before invocation.")
			return nil, false
		}
		if !value.IsNull() {
			headers[name] = value.ValueString()
		}
	}
	return headers, true
}

func runtimeProgress(resp *action.InvokeResponse, message string) {
	if resp.SendProgress != nil {
		resp.SendProgress(action.InvokeProgressEvent{Message: message})
	}
}
