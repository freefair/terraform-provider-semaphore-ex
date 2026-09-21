package provider

import (
	"context"
	"fmt"
	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rs "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"net/http"
	"time"
)

type runtimeSecretsModel struct {
	ID        types.String `tfsdk:"id"`
	State     types.String `tfsdk:"state"`
	ExpiresAt types.String `tfsdk:"expires_at"`
}
type runtimeSecretsResource struct{ client *apiclient.SemaphoreUI }
type runtimeSecretsDataSource struct{ client *apiclient.SemaphoreUI }

func NewRuntimeSecretsResource() resource.Resource       { return &runtimeSecretsResource{} }
func NewRuntimeSecretsDataSource() datasource.DataSource { return &runtimeSecretsDataSource{} }
func (r *runtimeSecretsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runtime_secrets"
}
func (d *runtimeSecretsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runtime_secrets"
}
func (r *runtimeSecretsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = rs.Schema{MarkdownDescription: "Manages configured runtime-secrets state and expiry. Requires the admin-only configured-state GET endpoint. Destroy forgets ownership and preserves the server setting.", Attributes: map[string]rs.Attribute{
		"id":         rs.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Singleton identity runtime_secrets."},
		"state":      rs.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("active", "disabled", "read_only")}, MarkdownDescription: "Explicit configured lifecycle state. Effective expiry does not rewrite this setting."},
		"expires_at": rs.StringAttribute{Optional: true, Validators: []validator.String{runtimeExpiryValidator{}}, MarkdownDescription: "Optional RFC3339 expiry. Omit to remove an expiry. Import retains the configured timestamp even after expiry."},
	}}
}
func (d *runtimeSecretsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = ds.Schema{MarkdownDescription: "Reads configured runtime-secrets state and expiry without changing them. Requires administrator access and the configured-state GET endpoint.", Attributes: map[string]ds.Attribute{"id": ds.StringAttribute{Computed: true}, "state": ds.StringAttribute{Computed: true}, "expires_at": ds.StringAttribute{Computed: true}}}
}
func (r *runtimeSecretsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	var ok bool
	r.client, ok = req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Invalid Provider Client", "Expected the configured Semaphore EX client.")
	}
}
func (d *runtimeSecretsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	var ok bool
	d.client, ok = req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Invalid Provider Client", "Expected the configured Semaphore EX client.")
	}
}
func readRuntimeSecrets(ctx context.Context, client *apiclient.SemaphoreUI, previous runtimeSecretsModel) (runtimeSecretsModel, error) {
	var raw struct {
		ID        string  `json:"id"`
		State     string  `json:"state"`
		ExpiresAt *string `json:"expires_at"`
	}
	if err := exRequest(ctx, client, http.MethodGet, "/capabilities/runtime-secrets", nil, nil, &raw); err != nil {
		if exNotFound(err) {
			return runtimeSecretsModel{}, fmt.Errorf("server does not expose configured runtime-secrets state; upgrade to a server providing GET /api/capabilities/runtime-secrets before managing or importing this setting")
		}
		return runtimeSecretsModel{}, err
	}
	if raw.ID != "runtime_secrets" || (raw.State != "active" && raw.State != "disabled" && raw.State != "read_only") {
		return runtimeSecretsModel{}, fmt.Errorf("server returned invalid configured runtime-secrets state")
	}
	result := runtimeSecretsModel{ID: types.StringValue(raw.ID), State: types.StringValue(raw.State), ExpiresAt: types.StringPointerValue(raw.ExpiresAt)}
	if raw.ExpiresAt != nil {
		actual, err := time.Parse(time.RFC3339Nano, *raw.ExpiresAt)
		if err != nil {
			return runtimeSecretsModel{}, fmt.Errorf("server returned an invalid runtime-secrets expiry")
		}
		prior, err := time.Parse(time.RFC3339Nano, previous.ExpiresAt.ValueString())
		if err == nil && actual.Equal(prior) {
			result.ExpiresAt = previous.ExpiresAt
		}
	}
	return result, nil
}
func writeRuntimeSecrets(ctx context.Context, client *apiclient.SemaphoreUI, plan runtimeSecretsModel) error {
	// Check configured-state read support before any explicitly requested write.
	if _, err := readRuntimeSecrets(ctx, client, runtimeSecretsModel{}); err != nil {
		return err
	}
	if plan.State.IsUnknown() || plan.State.IsNull() || plan.ExpiresAt.IsUnknown() {
		return fmt.Errorf("runtime-secrets settings must be known before applying")
	}
	body := map[string]any{"state": plan.State.ValueString(), "expires_at": plan.ExpiresAt.ValueStringPointer()}
	return exRequest(ctx, client, http.MethodPut, "/capabilities/runtime-secrets", nil, body, nil)
}
func (r *runtimeSecretsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan runtimeSecretsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := writeRuntimeSecrets(ctx, r.client, plan); err != nil {
		resp.Diagnostics.AddError("Error Configuring Runtime Secrets", err.Error())
		return
	}
	plan.ID = types.StringValue("runtime_secrets")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
	state, err := readRuntimeSecrets(ctx, r.client, plan)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Configured Runtime Secrets", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
func (r *runtimeSecretsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var previous runtimeSecretsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &previous)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, err := readRuntimeSecrets(ctx, r.client, previous)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Configured Runtime Secrets", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
func (r *runtimeSecretsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan runtimeSecretsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := writeRuntimeSecrets(ctx, r.client, plan); err != nil {
		resp.Diagnostics.AddError("Error Configuring Runtime Secrets", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
	state, err := readRuntimeSecrets(ctx, r.client, plan)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Configured Runtime Secrets", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
func (r *runtimeSecretsResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}
func (r *runtimeSecretsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID != "runtime_secrets" {
		resp.Diagnostics.AddError("Invalid Import Identity", "Use runtime_secrets for this singleton.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
func (d *runtimeSecretsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	state, err := readRuntimeSecrets(ctx, d.client, runtimeSecretsModel{})
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Configured Runtime Secrets", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

type runtimeExpiryValidator struct{}

func (runtimeExpiryValidator) Description(context.Context) string {
	return "Expiry must be an RFC3339 timestamp including a timezone."
}
func (v runtimeExpiryValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (v runtimeExpiryValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if _, err := time.Parse(time.RFC3339Nano, req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Expiry", v.Description(ctx))
	}
}
