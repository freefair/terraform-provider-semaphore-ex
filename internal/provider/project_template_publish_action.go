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

type projectTemplatePublishAction struct{ client *apiclient.SemaphoreUI }

var _ action.ActionWithConfigure = &projectTemplatePublishAction{}

func NewProjectTemplatePublishAction() action.Action { return &projectTemplatePublishAction{} }
func (a *projectTemplatePublishAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_template_publish"
}
func (a *projectTemplatePublishAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Publishes the current template configuration as an immutable version. An identical published snapshot is reused by the server. Invoked explicitly or through an action trigger; planning and refreshing do not publish.", Attributes: map[string]schema.Attribute{
		"project_id":  schema.Int64Attribute{Required: true, MarkdownDescription: "Project owning the template.", Validators: []validator.Int64{int64validator.AtLeast(1)}},
		"template_id": schema.Int64Attribute{Required: true, MarkdownDescription: "Template to publish.", Validators: []validator.Int64{int64validator.AtLeast(1)}},
	}}
}
func (a *projectTemplatePublishAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
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
func (a *projectTemplatePublishAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config struct {
		ProjectID  types.Int64 `tfsdk:"project_id"`
		TemplateID types.Int64 `tfsdk:"template_id"`
	}
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if config.ProjectID.IsUnknown() || config.TemplateID.IsUnknown() || config.ProjectID.ValueInt64() <= 0 || config.TemplateID.ValueInt64() <= 0 {
		resp.Diagnostics.AddError("Invalid Template Identity", "Both identifiers must be known positive integers.")
		return
	}
	params := map[string]string{"project_id": strconv.FormatInt(config.ProjectID.ValueInt64(), 10), "template_id": strconv.FormatInt(config.TemplateID.ValueInt64(), 10)}
	var version templateVersionResponse
	if err := exRequest(ctx, a.client, http.MethodPost, "/project/{project_id}/templates/{template_id}/versions", params, nil, &version); err != nil {
		resp.Diagnostics.AddError("Error Publishing Template Version", err.Error())
		return
	}
	if version.ID <= 0 || version.VersionNumber <= 0 {
		resp.Diagnostics.AddError("Invalid Publication Response", "Server did not return the published version identity.")
		return
	}
	if resp.SendProgress != nil {
		resp.SendProgress(action.InvokeProgressEvent{Message: fmt.Sprintf("Template version %d is published (record %d).", version.VersionNumber, version.ID)})
	}
}
