package provider

import (
	"context"
	"net/http"
	"regexp"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

type optionSetAction struct{ client *apiclient.SemaphoreUI }

func NewOptionSetAction() action.Action { return &optionSetAction{} }
func (a *optionSetAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_option_set"
}
func (a *optionSetAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
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
func (a *optionSetAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Explicitly writes a persisted global system option. This is an action because the API has no option-delete contract; there is no implied destroy or reset. Changes may require a server restart. Prefer a dedicated resource when one models the same setting.", Attributes: map[string]schema.Attribute{
		"key":   schema.StringAttribute{Required: true, MarkdownDescription: "Exact option key.", Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[\w.]+$`), "Use letters, digits, underscores and dots.")}},
		"value": schema.StringAttribute{Required: true, WriteOnly: true, MarkdownDescription: "Persisted option value. Pass a sensitive, ephemeral input variable for confidential values; the provider does not emit the value in progress."},
	}}
}
func (a *optionSetAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config optionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if config.Key.IsUnknown() || config.Value.IsUnknown() || config.Key.IsNull() || config.Value.IsNull() {
		resp.Diagnostics.AddError("Unknown System Option", "The key and value must be known before invocation.")
		return
	}
	if err := exRequest(ctx, a.client, http.MethodPost, "/options", nil, map[string]string{"key": config.Key.ValueString(), "value": config.Value.ValueString()}, nil); err != nil {
		resp.Diagnostics.AddError("Error Writing System Option", err.Error())
	}
}
