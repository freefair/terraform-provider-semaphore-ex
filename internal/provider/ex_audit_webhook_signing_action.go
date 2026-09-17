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

type exAuditWebhookSigningAction struct {
	client *apiclient.SemaphoreUI
	revoke bool
}

func NewAuditWebhookSigningPromoteAction() action.Action { return &exAuditWebhookSigningAction{} }
func NewAuditWebhookSigningRevokeNextAction() action.Action {
	return &exAuditWebhookSigningAction{revoke: true}
}

func (a *exAuditWebhookSigningAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	name := "audit_webhook_signing_promote"
	if a.revoke {
		name = "audit_webhook_signing_revoke_next"
	}
	resp.TypeName = req.ProviderTypeName + "_" + name
}
func (a *exAuditWebhookSigningAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Action Configure Type", "Expected *client.SemaphoreUI.")
		return
	}
	a.client = client
}
func (a *exAuditWebhookSigningAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Explicitly advances audit webhook signing state using the exact server signing revision.", Attributes: map[string]schema.Attribute{
		"expected_signing_revision": schema.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(0)}},
	}}
}
func (a *exAuditWebhookSigningAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config struct {
		ExpectedSigningRevision types.Int64 `tfsdk:"expected_signing_revision"`
	}
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	route := "/audit-webhook/signing-secret/promote"
	method := http.MethodPost
	if a.revoke {
		route = "/audit-webhook/signing-secret/next"
		method = http.MethodDelete
	}
	if err := exRequestWithOptions(ctx, a.client, method, route, exRequestOptions{Query: map[string]string{"revision": strconv.FormatInt(config.ExpectedSigningRevision.ValueInt64(), 10)}}, nil, nil); err != nil {
		resp.Diagnostics.AddError("Audit Webhook Signing Lifecycle Operation Failed", fmt.Sprintf("The server rejected the revision-fenced operation: %v", err))
	}
}
