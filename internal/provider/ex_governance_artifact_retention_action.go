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

type exWorkflowArtifactRetentionPublishAction struct {
	client  *apiclient.SemaphoreUI
	project bool
}

var _ action.ActionWithConfigure = &exWorkflowArtifactRetentionPublishAction{}

func NewGlobalWorkflowArtifactRetentionPublishAction() action.Action {
	return &exWorkflowArtifactRetentionPublishAction{}
}
func NewProjectWorkflowArtifactRetentionPublishAction() action.Action {
	return &exWorkflowArtifactRetentionPublishAction{project: true}
}
func (a *exWorkflowArtifactRetentionPublishAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	name := "global_workflow_artifact_retention_publish"
	if a.project {
		name = "project_workflow_artifact_retention_publish"
	}
	resp.TypeName = req.ProviderTypeName + "_" + name
}
func (a *exWorkflowArtifactRetentionPublishAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Workflow Artifact Retention Configure Type", "Expected the configured Semaphore EX client.")
		return
	}
	a.client = client
}
func (a *exWorkflowArtifactRetentionPublishAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"expected_revision":  schema.Int64Attribute{Required: true, MarkdownDescription: "Exact current revision; use 0 only before the first publication.", Validators: []validator.Int64{int64validator.AtLeast(0)}},
		"retention_seconds":  schema.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(3600)}},
		"max_artifact_bytes": schema.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}},
		"max_run_bytes":      schema.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}},
	}
	if a.project {
		attrs["project_id"] = schema.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}}
	} else {
		attrs["project_id"] = schema.Int64Attribute{Optional: true}
	}
	resp.Schema = schema.Schema{MarkdownDescription: "Publishes an append-only, revision-fenced Semaphore EX workflow artifact retention policy. Invoke explicitly; the API deliberately has no delete or reset operation.", Attributes: attrs}
}
func (a *exWorkflowArtifactRetentionPublishAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config struct {
		ProjectID        types.Int64 `tfsdk:"project_id"`
		ExpectedRevision types.Int64 `tfsdk:"expected_revision"`
		RetentionSeconds types.Int64 `tfsdk:"retention_seconds"`
		MaxArtifactBytes types.Int64 `tfsdk:"max_artifact_bytes"`
		MaxRunBytes      types.Int64 `tfsdk:"max_run_bytes"`
	}
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	for name, value := range map[string]types.Int64{"expected_revision": config.ExpectedRevision, "retention_seconds": config.RetentionSeconds, "max_artifact_bytes": config.MaxArtifactBytes, "max_run_bytes": config.MaxRunBytes} {
		if value.IsNull() || value.IsUnknown() {
			resp.Diagnostics.AddError("Unknown Workflow Artifact Retention Input", name+" must be known before publication.")
			return
		}
	}
	if !a.project && !config.ProjectID.IsNull() {
		resp.Diagnostics.AddError("Invalid Global Retention Scope", "project_id must be omitted for the global policy; use the project publication action for a project policy.")
		return
	}
	params := map[string]string(nil)
	if a.project {
		if config.ProjectID.IsNull() || config.ProjectID.IsUnknown() || config.ProjectID.ValueInt64() < 1 {
			resp.Diagnostics.AddError("Invalid Workflow Artifact Retention Project", "project_id must be a known positive integer.")
			return
		}
		params = map[string]string{"project_id": strconv.FormatInt(config.ProjectID.ValueInt64(), 10)}
	}
	body := map[string]any{"expected_revision": config.ExpectedRevision.ValueInt64(), "retention_seconds": config.RetentionSeconds.ValueInt64(), "max_artifact_bytes": config.MaxArtifactBytes.ValueInt64(), "max_run_bytes": config.MaxRunBytes.ValueInt64()}
	var raw map[string]any
	if err := exRequest(ctx, a.client, http.MethodPut, exWorkflowArtifactRetentionRoute(a.project), params, body, &raw); err != nil {
		resp.Diagnostics.AddError("Error Publishing Semaphore EX Workflow Artifact Retention", err.Error())
		return
	}
	if resp.SendProgress != nil {
		scope := "global"
		if a.project {
			scope = "project " + strconv.FormatInt(config.ProjectID.ValueInt64(), 10)
		}
		resp.SendProgress(action.InvokeProgressEvent{Message: fmt.Sprintf("Workflow artifact retention policy published for %s.", scope)})
	}
}
