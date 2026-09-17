package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rs "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const exAuditWebhookID = "audit_webhook"

type exAuditWebhookModel struct {
	ID                      types.String `tfsdk:"id"`
	Endpoint                types.String `tfsdk:"endpoint"`
	CredentialWO            types.String `tfsdk:"credential_wo"`
	CredentialWOVersion     types.Int64  `tfsdk:"credential_wo_version"`
	CredentialConfigured    types.Bool   `tfsdk:"credential_configured"`
	Paused                  types.Bool   `tfsdk:"paused"`
	CurrentKeyID            types.String `tfsdk:"current_key_id"`
	NextKeyID               types.String `tfsdk:"next_key_id"`
	CurrentGeneration       types.Int64  `tfsdk:"current_generation"`
	NextGeneration          types.Int64  `tfsdk:"next_generation"`
	SigningRevision         types.Int64  `tfsdk:"signing_revision"`
	SigningBootstrapVersion types.Int64  `tfsdk:"signing_bootstrap_version"`
	SigningStageVersion     types.Int64  `tfsdk:"signing_stage_version"`
	CurrentSigningSecret    types.String `tfsdk:"current_signing_secret"`
	NextSigningSecret       types.String `tfsdk:"next_signing_secret"`
}
type exAuditWebhookDataSourceModel struct {
	ID                   types.String `tfsdk:"id"`
	Endpoint             types.String `tfsdk:"endpoint"`
	CredentialConfigured types.Bool   `tfsdk:"credential_configured"`
	Paused               types.Bool   `tfsdk:"paused"`
	CurrentKeyID         types.String `tfsdk:"current_key_id"`
	NextKeyID            types.String `tfsdk:"next_key_id"`
	CurrentGeneration    types.Int64  `tfsdk:"current_generation"`
	NextGeneration       types.Int64  `tfsdk:"next_generation"`
	SigningRevision      types.Int64  `tfsdk:"signing_revision"`
}
type exAuditWebhookResource struct{ client *apiclient.SemaphoreUI }
type exAuditWebhookDataSource struct{ client *apiclient.SemaphoreUI }

func NewAuditWebhookResource() resource.Resource       { return &exAuditWebhookResource{} }
func NewAuditWebhookDataSource() datasource.DataSource { return &exAuditWebhookDataSource{} }

type exHTTPSURLValidator struct{}

func (exHTTPSURLValidator) Description(context.Context) string {
	return "Must be an HTTPS URL without credentials or fragments."
}
func (v exHTTPSURLValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (v exHTTPSURLValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueString()
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || parsed.Opaque != "" || strings.Contains(value, "#") {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid audit webhook endpoint", v.Description(ctx))
	}
}

func exAuditWebhookResourceSchema() rs.Schema {
	return rs.Schema{MarkdownDescription: "Manages the singleton Semaphore EX audit webhook configuration. Destroy pauses deliveries but retains the server configuration and credentials. It never sends test deliveries.", Attributes: map[string]rs.Attribute{
		"id": rs.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}, "endpoint": rs.StringAttribute{Required: true, Validators: []validator.String{exHTTPSURLValidator{}}}, "credential_wo": rs.StringAttribute{Optional: true, Sensitive: true, WriteOnly: true}, "credential_wo_version": rs.Int64Attribute{Optional: true}, "credential_configured": rs.BoolAttribute{Computed: true}, "paused": rs.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)}, "current_key_id": rs.StringAttribute{Computed: true}, "next_key_id": rs.StringAttribute{Computed: true}, "current_generation": rs.Int64Attribute{Computed: true}, "next_generation": rs.Int64Attribute{Computed: true}, "signing_revision": rs.Int64Attribute{Computed: true}, "signing_bootstrap_version": rs.Int64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}}, "signing_stage_version": rs.Int64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}}, "current_signing_secret": rs.StringAttribute{Computed: true, Sensitive: true}, "next_signing_secret": rs.StringAttribute{Computed: true, Sensitive: true}}}
}
func exAuditWebhookDataSourceSchema() ds.Schema {
	return ds.Schema{MarkdownDescription: "Reads the non-secret Semaphore EX audit webhook configuration.", Attributes: map[string]ds.Attribute{"id": ds.StringAttribute{Computed: true}, "endpoint": ds.StringAttribute{Computed: true}, "credential_configured": ds.BoolAttribute{Computed: true}, "paused": ds.BoolAttribute{Computed: true}, "current_key_id": ds.StringAttribute{Computed: true}, "next_key_id": ds.StringAttribute{Computed: true}, "current_generation": ds.Int64Attribute{Computed: true}, "next_generation": ds.Int64Attribute{Computed: true}, "signing_revision": ds.Int64Attribute{Computed: true}}}
}
func (r *exAuditWebhookResource) Metadata(_ context.Context, q resource.MetadataRequest, p *resource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_audit_webhook"
}
func (d *exAuditWebhookDataSource) Metadata(_ context.Context, q datasource.MetadataRequest, p *datasource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_audit_webhook"
}
func (r *exAuditWebhookResource) Schema(_ context.Context, _ resource.SchemaRequest, p *resource.SchemaResponse) {
	p.Schema = exAuditWebhookResourceSchema()
}
func (d *exAuditWebhookDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, p *datasource.SchemaResponse) {
	p.Schema = exAuditWebhookDataSourceSchema()
}
func (r *exAuditWebhookResource) Configure(_ context.Context, q resource.ConfigureRequest, p *resource.ConfigureResponse) {
	if q.ProviderData == nil {
		return
	}
	if c, ok := exNotificationClient(q.ProviderData); ok {
		r.client = c
	} else {
		p.Diagnostics.AddError("Unexpected Resource Configure Type", "Expected *client.SemaphoreUI.")
	}
}
func (d *exAuditWebhookDataSource) Configure(_ context.Context, q datasource.ConfigureRequest, p *datasource.ConfigureResponse) {
	if q.ProviderData == nil {
		return
	}
	if c, ok := exNotificationClient(q.ProviderData); ok {
		d.client = c
	} else {
		p.Diagnostics.AddError("Unexpected Data Source Configure Type", "Expected *client.SemaphoreUI.")
	}
}

func exAuditWebhookFromResponse(old exAuditWebhookModel, raw map[string]any) (exAuditWebhookModel, error) {
	next := old
	next.ID = types.StringValue(exAuditWebhookID)
	for name, target := range map[string]any{"endpoint": &next.Endpoint, "credential_configured": &next.CredentialConfigured, "paused": &next.Paused, "current_key_id": &next.CurrentKeyID, "next_key_id": &next.NextKeyID, "current_generation": &next.CurrentGeneration, "next_generation": &next.NextGeneration, "signing_revision": &next.SigningRevision} {
		value, ok := raw[name]
		if !ok {
			switch out := target.(type) {
			case *types.String:
				*out = types.StringNull()
			case *types.Bool:
				*out = types.BoolNull()
			case *types.Int64:
				*out = types.Int64Null()
			}
			continue
		}
		switch out := target.(type) {
		case *types.String:
			text, ok := value.(string)
			if !ok {
				return old, fmt.Errorf("invalid audit webhook response %s", name)
			}
			*out = types.StringValue(text)
		case *types.Bool:
			b, ok := value.(bool)
			if !ok {
				return old, fmt.Errorf("invalid audit webhook response %s", name)
			}
			*out = types.BoolValue(b)
		case *types.Int64:
			n, err := exTypedValue(context.Background(), types.Int64Type, value)
			if err != nil {
				return old, fmt.Errorf("invalid audit webhook response %s", name)
			}
			converted, ok := n.(types.Int64)
			if !ok {
				return old, fmt.Errorf("invalid audit webhook response %s", name)
			}
			*out = converted
		}
	}
	return exAuditWebhookRetainSigningSecrets(old, next), nil
}

func exAuditWebhookRetainSigningSecrets(old, next exAuditWebhookModel) exAuditWebhookModel {
	if old.CurrentGeneration.Equal(next.CurrentGeneration) && old.CurrentKeyID.Equal(next.CurrentKeyID) {
		next.CurrentSigningSecret = old.CurrentSigningSecret
	} else if old.NextGeneration.Equal(next.CurrentGeneration) && old.NextKeyID.Equal(next.CurrentKeyID) {
		next.CurrentSigningSecret = old.NextSigningSecret
	} else {
		next.CurrentSigningSecret = types.StringNull()
	}
	if old.NextGeneration.Equal(next.NextGeneration) && old.NextKeyID.Equal(next.NextKeyID) {
		next.NextSigningSecret = old.NextSigningSecret
	} else if old.CurrentGeneration.Equal(next.NextGeneration) && old.CurrentKeyID.Equal(next.NextKeyID) {
		next.NextSigningSecret = old.CurrentSigningSecret
	} else {
		next.NextSigningSecret = types.StringNull()
	}
	return next
}

type exAuditWebhookSigningSecretResponse struct {
	Secret            string `json:"secret"`
	CurrentKeyID      string `json:"current_key_id"`
	NextKeyID         string `json:"next_key_id"`
	CurrentGeneration int64  `json:"current_generation"`
	NextGeneration    int64  `json:"next_generation"`
	SigningRevision   int64  `json:"signing_revision"`
}

func exAuditWebhookSigningMutation(ctx context.Context, client *apiclient.SemaphoreUI, state exAuditWebhookModel, route string, next bool) (exAuditWebhookModel, error) {
	if state.SigningRevision.IsNull() || state.SigningRevision.IsUnknown() || state.SigningRevision.ValueInt64() < 0 {
		return state, fmt.Errorf("audit webhook signing revision is unavailable")
	}
	var raw exAuditWebhookSigningSecretResponse
	if err := exRequestWithOptions(ctx, client, http.MethodPost, route, exRequestOptions{Query: map[string]string{"revision": fmt.Sprintf("%d", state.SigningRevision.ValueInt64())}}, nil, &raw); err != nil {
		return state, err
	}
	if raw.Secret == "" {
		return state, fmt.Errorf("audit webhook signing response omitted the one-time secret")
	}
	nextState := state
	nextState.CurrentKeyID = types.StringValue(raw.CurrentKeyID)
	nextState.NextKeyID = types.StringValue(raw.NextKeyID)
	nextState.CurrentGeneration = types.Int64Value(raw.CurrentGeneration)
	nextState.NextGeneration = types.Int64Value(raw.NextGeneration)
	nextState.SigningRevision = types.Int64Value(raw.SigningRevision)
	if next {
		nextState.NextSigningSecret = types.StringValue(raw.Secret)
	} else {
		nextState.CurrentSigningSecret = types.StringValue(raw.Secret)
		nextState.NextSigningSecret = types.StringNull()
	}
	return nextState, nil
}
func exAuditWebhookDataSourceFromResponse(old exAuditWebhookDataSourceModel, raw map[string]any) (exAuditWebhookDataSourceModel, error) {
	resourceState, err := exAuditWebhookFromResponse(exAuditWebhookModel{ID: old.ID, Endpoint: old.Endpoint, CredentialConfigured: old.CredentialConfigured, Paused: old.Paused, CurrentKeyID: old.CurrentKeyID, NextKeyID: old.NextKeyID, CurrentGeneration: old.CurrentGeneration, NextGeneration: old.NextGeneration, SigningRevision: old.SigningRevision}, raw)
	if err != nil {
		return old, err
	}
	return exAuditWebhookDataSourceModel{ID: resourceState.ID, Endpoint: resourceState.Endpoint, CredentialConfigured: resourceState.CredentialConfigured, Paused: resourceState.Paused, CurrentKeyID: resourceState.CurrentKeyID, NextKeyID: resourceState.NextKeyID, CurrentGeneration: resourceState.CurrentGeneration, NextGeneration: resourceState.NextGeneration, SigningRevision: resourceState.SigningRevision}, nil
}
func exAuditWebhookRead(ctx context.Context, c *apiclient.SemaphoreUI, old exAuditWebhookModel) (exAuditWebhookModel, error) {
	var raw map[string]any
	if err := exRequest(ctx, c, http.MethodGet, "/audit-webhook", nil, nil, &raw); err != nil {
		return old, err
	}
	return exAuditWebhookFromResponse(old, raw)
}
func exAuditWebhookConfigure(ctx context.Context, c *apiclient.SemaphoreUI, plan, config exAuditWebhookModel) (exAuditWebhookModel, error) {
	body := map[string]any{"endpoint": plan.Endpoint.ValueString()}
	if !config.CredentialWO.IsNull() && !config.CredentialWO.IsUnknown() {
		body["credential"] = config.CredentialWO.ValueString()
	}
	var raw map[string]any
	if err := exRequest(ctx, c, http.MethodPut, "/audit-webhook", nil, body, &raw); err != nil {
		return plan, err
	}
	state, err := exAuditWebhookFromResponse(plan, raw)
	if err != nil {
		return plan, err
	}
	if state.Paused.ValueBool() != plan.Paused.ValueBool() {
		route := "/audit-webhook/resume"
		if plan.Paused.ValueBool() {
			route = "/audit-webhook/pause"
		}
		if err := exRequest(ctx, c, http.MethodPost, route, nil, nil, &raw); err != nil {
			return plan, err
		}
		state, err = exAuditWebhookFromResponse(state, raw)
	}
	return state, err
}
func (r *exAuditWebhookResource) Create(ctx context.Context, q resource.CreateRequest, p *resource.CreateResponse) {
	var plan, config exAuditWebhookModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &plan)...)
	p.Diagnostics.Append(q.Config.Get(ctx, &config)...)
	if p.Diagnostics.HasError() {
		return
	}
	seed := plan
	seed.SigningBootstrapVersion = types.Int64Null()
	seed.SigningStageVersion = types.Int64Null()
	state, err := exAuditWebhookConfigure(ctx, r.client, seed, config)
	if err != nil {
		p.Diagnostics.AddError("Error Configuring Semaphore EX Audit Webhook", err.Error())
		return
	}
	if !plan.SigningBootstrapVersion.IsNull() && !plan.SigningBootstrapVersion.IsUnknown() {
		state, err = exAuditWebhookSigningMutation(ctx, r.client, state, "/audit-webhook/signing-secret", false)
		if err != nil {
			p.Diagnostics.AddError("Error Creating Audit Webhook Signing Secret", err.Error())
			return
		}
		state.SigningBootstrapVersion = plan.SigningBootstrapVersion
	}
	if !plan.SigningStageVersion.IsNull() && !plan.SigningStageVersion.IsUnknown() {
		staged, stageErr := exAuditWebhookSigningMutation(ctx, r.client, state, "/audit-webhook/signing-secret/stage", true)
		if stageErr != nil {
			p.Diagnostics.Append(p.State.Set(ctx, &state)...)
			p.Diagnostics.AddError("Audit Webhook Signing Secret Staging Failed", "The bootstrap signing secret was retained in Terraform state. Retry by changing signing_stage_version after resolving the server error.")
			return
		}
		state = staged
		state.SigningStageVersion = plan.SigningStageVersion
	}
	p.Diagnostics.Append(p.State.Set(ctx, &state)...)
}
func (r *exAuditWebhookResource) Read(ctx context.Context, q resource.ReadRequest, p *resource.ReadResponse) {
	var state exAuditWebhookModel
	p.Diagnostics.Append(q.State.Get(ctx, &state)...)
	if p.Diagnostics.HasError() {
		return
	}
	next, err := exAuditWebhookRead(ctx, r.client, state)
	if exNotFound(err) {
		p.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		p.Diagnostics.AddError("Error Reading Semaphore EX Audit Webhook", err.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (r *exAuditWebhookResource) Update(ctx context.Context, q resource.UpdateRequest, p *resource.UpdateResponse) {
	var plan, config, prior exAuditWebhookModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &plan)...)
	p.Diagnostics.Append(q.Config.Get(ctx, &config)...)
	p.Diagnostics.Append(q.State.Get(ctx, &prior)...)
	if p.Diagnostics.HasError() {
		return
	}
	seed := prior
	seed.Endpoint, seed.Paused = plan.Endpoint, plan.Paused
	configureInput := config
	credentialVersionChanged := !plan.CredentialWOVersion.Equal(prior.CredentialWOVersion)
	if !credentialVersionChanged {
		configureInput.CredentialWO = types.StringNull()
	} else if config.CredentialWO.IsNull() || config.CredentialWO.IsUnknown() {
		p.Diagnostics.AddError("Audit Webhook Credential Version Requires Material", "Set credential_wo when changing credential_wo_version.")
		return
	}
	state, err := exAuditWebhookConfigure(ctx, r.client, seed, configureInput)
	if err != nil {
		p.Diagnostics.AddError("Error Updating Semaphore EX Audit Webhook", err.Error())
		return
	}
	if credentialVersionChanged {
		state.CredentialWOVersion = plan.CredentialWOVersion
	}
	if !plan.SigningBootstrapVersion.IsNull() && !plan.SigningBootstrapVersion.IsUnknown() && !plan.SigningBootstrapVersion.Equal(prior.SigningBootstrapVersion) {
		state, err = exAuditWebhookSigningMutation(ctx, r.client, state, "/audit-webhook/signing-secret", false)
		if err != nil {
			p.Diagnostics.Append(p.State.Set(ctx, &state)...)
			p.Diagnostics.AddError("Error Creating Audit Webhook Signing Secret", err.Error())
			return
		}
		state.SigningBootstrapVersion = plan.SigningBootstrapVersion
	}
	if !plan.SigningStageVersion.IsNull() && !plan.SigningStageVersion.IsUnknown() && !plan.SigningStageVersion.Equal(prior.SigningStageVersion) {
		staged, stageErr := exAuditWebhookSigningMutation(ctx, r.client, state, "/audit-webhook/signing-secret/stage", true)
		if stageErr != nil {
			p.Diagnostics.Append(p.State.Set(ctx, &state)...)
			p.Diagnostics.AddError("Audit Webhook Signing Secret Staging Failed", "The achieved signing state was retained. Resolve the server error and retry.")
			return
		}
		state = staged
		state.SigningStageVersion = plan.SigningStageVersion
	}
	p.Diagnostics.Append(p.State.Set(ctx, &state)...)
}

// The singleton has no delete route. Destroy therefore uses the server's safe
// transition: pause delivery while retaining configuration and credentials.
func (r *exAuditWebhookResource) Delete(ctx context.Context, _ resource.DeleteRequest, p *resource.DeleteResponse) {
	if err := exRequest(ctx, r.client, http.MethodPost, "/audit-webhook/pause", nil, nil, nil); err != nil && !exNotFound(err) {
		p.Diagnostics.AddError("Error Pausing Semaphore EX Audit Webhook", err.Error())
	}
}
func (r *exAuditWebhookResource) ImportState(ctx context.Context, q resource.ImportStateRequest, p *resource.ImportStateResponse) {
	if q.ID != exAuditWebhookID {
		p.Diagnostics.AddError("Invalid Import ID", "Use audit_webhook.")
		return
	}
	state, err := exAuditWebhookRead(ctx, r.client, exAuditWebhookModel{ID: types.StringValue(exAuditWebhookID)})
	if err != nil {
		p.Diagnostics.AddError("Error Importing Semaphore EX Audit Webhook", err.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &state)...)
}
func (d *exAuditWebhookDataSource) Read(ctx context.Context, q datasource.ReadRequest, p *datasource.ReadResponse) {
	var config exAuditWebhookDataSourceModel
	p.Diagnostics.Append(q.Config.Get(ctx, &config)...)
	if p.Diagnostics.HasError() {
		return
	}
	var raw map[string]any
	err := exRequest(ctx, d.client, http.MethodGet, "/audit-webhook", nil, nil, &raw)
	state, responseErr := exAuditWebhookDataSourceFromResponse(config, raw)
	if err == nil {
		err = responseErr
	}
	if err != nil {
		p.Diagnostics.AddError("Error Reading Semaphore EX Audit Webhook", err.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &state)...)
}

var _ resource.ResourceWithImportState = &exAuditWebhookResource{}
