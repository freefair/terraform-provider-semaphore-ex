package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// generatedSSHKeyModel contains only public server-generated SSH key metadata.
// The server never returns the private key or the SSH login after creation.
type generatedSSHKeyModel struct {
	ID             types.Int64  `tfsdk:"id"`
	ProjectID      types.Int64  `tfsdk:"project_id"`
	Name           types.String `tfsdk:"name"`
	Login          types.String `tfsdk:"login"`
	LoginWOVersion types.Int64  `tfsdk:"login_wo_version"`
	Algorithm      types.String `tfsdk:"algorithm"`
	PublicKey      types.String `tfsdk:"public_key"`
	Fingerprint    types.String `tfsdk:"fingerprint"`
}

// generatedSSHKeyDataModel deliberately has no login field: the read API does
// not reveal it and the data source must not imply that it can be recovered.
type generatedSSHKeyDataModel struct {
	ID          types.Int64  `tfsdk:"id"`
	ProjectID   types.Int64  `tfsdk:"project_id"`
	Name        types.String `tfsdk:"name"`
	Algorithm   types.String `tfsdk:"algorithm"`
	PublicKey   types.String `tfsdk:"public_key"`
	Fingerprint types.String `tfsdk:"fingerprint"`
}

type generatedSSHKeyResource struct{ client *apiclient.SemaphoreUI }
type generatedSSHKeyDataSource struct{ client *apiclient.SemaphoreUI }

var _ resource.ResourceWithConfigure = &generatedSSHKeyResource{}
var _ resource.ResourceWithImportState = &generatedSSHKeyResource{}
var _ datasource.DataSourceWithConfigure = &generatedSSHKeyDataSource{}

func NewProjectGeneratedSSHKeyResource() resource.Resource { return &generatedSSHKeyResource{} }
func NewProjectGeneratedSSHKeyDataSource() datasource.DataSource {
	return withNamedLookup(&generatedSSHKeyDataSource{}, "project_generated_ssh_key")
}

func (r *generatedSSHKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_generated_ssh_key"
}

func (d *generatedSSHKeyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_generated_ssh_key"
}

func generatedSSHKeyClient(value any, diagnostics interface{ AddError(string, string) }) *apiclient.SemaphoreUI {
	if value == nil {
		return nil
	}
	client, ok := value.(*apiclient.SemaphoreUI)
	if !ok {
		diagnostics.AddError("Unexpected Generated SSH Key Configure Type", "Expected the configured Semaphore EX client.")
		return nil
	}
	return client
}

func (r *generatedSSHKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = generatedSSHKeyClient(req.ProviderData, &resp.Diagnostics)
}

func (d *generatedSSHKeyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = generatedSSHKeyClient(req.ProviderData, &resp.Diagnostics)
}

func (r *generatedSSHKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{MarkdownDescription: "Creates a server-generated SSH access key. The private key never enters Terraform state or provider output. `algorithm` is immutable and replaces the key when changed. `login` is write-only, so change it only with an incremented `login_wo_version`, which explicitly replaces the key. Import reads public metadata only; leave both login inputs omitted for imported keys.", Attributes: generatedSSHKeyResourceAttributes()}
}

func (d *generatedSSHKeyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceschema.Schema{MarkdownDescription: "Reads public metadata for a server-generated SSH access key. It never exposes private key material or SSH login.", Attributes: generatedSSHKeyDataSourceAttributes()}
}

func generatedSSHKeyResourceAttributes() map[string]resourceschema.Attribute {
	return map[string]resourceschema.Attribute{
		"id":               resourceschema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
		"project_id":       resourceschema.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}},
		"name":             resourceschema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}},
		"login":            resourceschema.StringAttribute{Optional: true, WriteOnly: true, MarkdownDescription: "SSH login used only to create a generated key. The server does not return it. To change login, increment login_wo_version to explicitly replace the key. Omit both after importing an existing generated key."},
		"login_wo_version": resourceschema.Int64Attribute{Optional: true, MarkdownDescription: "Persisted replacement trigger for the write-only login. Set a known positive value when creating; increment it when changing login to replace the generated key.", Validators: []validator.Int64{int64validator.AtLeast(1)}, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}},
		"algorithm":        resourceschema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("ed25519", "rsa-3072")}, MarkdownDescription: "Generation algorithm. Changing it replaces the key; use the explicit rotation action to rotate an existing key in place.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"public_key":       resourceschema.StringAttribute{Computed: true},
		"fingerprint":      resourceschema.StringAttribute{Computed: true},
	}
}

func generatedSSHKeyDataSourceAttributes() map[string]datasourceschema.Attribute {
	return map[string]datasourceschema.Attribute{
		"id":          datasourceschema.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}},
		"project_id":  datasourceschema.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}},
		"name":        datasourceschema.StringAttribute{Computed: true},
		"algorithm":   datasourceschema.StringAttribute{Computed: true},
		"public_key":  datasourceschema.StringAttribute{Computed: true},
		"fingerprint": datasourceschema.StringAttribute{Computed: true},
	}
}

func generatedSSHKeyParams(model generatedSSHKeyModel) (map[string]string, error) {
	projectID, err := exPathID(model.ProjectID)
	if err != nil {
		return nil, err
	}
	keyID, err := exPathID(model.ID)
	if err != nil {
		return nil, err
	}
	return map[string]string{"project_id": projectID, "key_id": keyID}, nil
}

func generatedSSHKeyFromAPI(raw map[string]any, prior generatedSSHKeyModel) (generatedSSHKeyModel, error) {
	model := prior
	if id, err := identityNumber(raw["id"]); err == nil && id > 0 {
		model.ID = types.Int64Value(id)
	} else {
		return generatedSSHKeyModel{}, fmt.Errorf("generated SSH key response has no key ID")
	}
	if projectID, err := identityNumber(raw["project_id"]); err == nil && projectID > 0 {
		model.ProjectID = types.Int64Value(projectID)
	} else {
		return generatedSSHKeyModel{}, fmt.Errorf("generated SSH key response has no project ID")
	}
	if name, ok := raw["name"].(string); ok && name != "" {
		model.Name = types.StringValue(name)
	} else {
		return generatedSSHKeyModel{}, fmt.Errorf("generated SSH key response has no name")
	}
	if keyType, ok := raw["type"].(string); !ok || keyType != "ssh" {
		return generatedSSHKeyModel{}, fmt.Errorf("key is not an SSH key")
	}
	metadata, ok := raw["generated_ssh_key"].(map[string]any)
	if !ok {
		return generatedSSHKeyModel{}, fmt.Errorf("key is not server-generated")
	}
	publicKey, publicOK := metadata["public_key"].(string)
	fingerprint, fingerprintOK := metadata["fingerprint"].(string)
	algorithm, algorithmOK := metadata["algorithm"].(string)
	if !publicOK || publicKey == "" || !fingerprintOK || fingerprint == "" || !algorithmOK || (algorithm != "ed25519" && algorithm != "rsa-3072") {
		return generatedSSHKeyModel{}, fmt.Errorf("generated SSH key metadata is incomplete")
	}
	model.PublicKey, model.Fingerprint, model.Algorithm = types.StringValue(publicKey), types.StringValue(fingerprint), types.StringValue(algorithm)
	if model.Login.IsUnknown() {
		model.Login = types.StringNull()
	}
	return model, nil
}

func generatedSSHKeyRead(ctx context.Context, client *apiclient.SemaphoreUI, model generatedSSHKeyModel) (generatedSSHKeyModel, error) {
	params, err := generatedSSHKeyParams(model)
	if err != nil {
		return generatedSSHKeyModel{}, err
	}
	var raw map[string]any
	if err = exRequest(ctx, client, http.MethodGet, "/project/{project_id}/keys/{key_id}", params, nil, &raw); err != nil {
		return generatedSSHKeyModel{}, err
	}
	return generatedSSHKeyFromAPI(raw, model)
}

func (r *generatedSSHKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config generatedSSHKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := exPathID(plan.ProjectID); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("project_id"), "Invalid Generated SSH Key Project", err.Error())
		return
	}
	if plan.Name.IsNull() || plan.Name.IsUnknown() || strings.TrimSpace(plan.Name.ValueString()) == "" {
		resp.Diagnostics.AddAttributeError(path.Root("name"), "Invalid Generated SSH Key Name", "name must be known and non-empty.")
		return
	}
	if config.Login.IsNull() || config.Login.IsUnknown() || strings.TrimSpace(config.Login.ValueString()) == "" {
		resp.Diagnostics.AddAttributeError(path.Root("login"), "Missing Generated SSH Key Login", "login is a required write-only input when creating a generated SSH key. It cannot be recovered after import.")
		return
	}
	if plan.LoginWOVersion.IsNull() || plan.LoginWOVersion.IsUnknown() || plan.LoginWOVersion.ValueInt64() < 1 {
		resp.Diagnostics.AddAttributeError(path.Root("login_wo_version"), "Missing Generated SSH Key Login Version", "login_wo_version must be a known positive value when creating a generated SSH key. Increment it when changing the write-only login.")
		return
	}
	if plan.Algorithm.IsNull() || plan.Algorithm.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("algorithm"), "Invalid Generated SSH Key Algorithm", "algorithm must be known before creation.")
		return
	}
	var response struct {
		Key         map[string]any `json:"key"`
		PublicKey   string         `json:"public_key"`
		Fingerprint string         `json:"fingerprint"`
		Algorithm   string         `json:"algorithm"`
	}
	params := map[string]string{"project_id": strconv.FormatInt(plan.ProjectID.ValueInt64(), 10)}
	if err := exRequest(ctx, r.client, http.MethodPost, "/project/{project_id}/keys/generate", params, map[string]any{"name": plan.Name.ValueString(), "login": config.Login.ValueString(), "algorithm": plan.Algorithm.ValueString()}, &response); err != nil {
		resp.Diagnostics.AddError("Error Creating Semaphore EX Generated SSH Key", err.Error())
		return
	}
	state, err := generatedSSHKeyFromAPI(response.Key, plan)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Generated SSH Key Response", err.Error())
		return
	}
	if response.PublicKey != state.PublicKey.ValueString() || response.Fingerprint != state.Fingerprint.ValueString() || response.Algorithm != state.Algorithm.ValueString() {
		resp.Diagnostics.AddError("Invalid Generated SSH Key Response", "Create response metadata does not match the public key record.")
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *generatedSSHKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state generatedSSHKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	next, err := generatedSSHKeyRead(ctx, r.client, state)
	if exNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Semaphore EX Generated SSH Key", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *generatedSSHKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state generatedSSHKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	params, err := generatedSSHKeyParams(state)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Generated SSH Key Identity", err.Error())
		return
	}
	if err = exRequest(ctx, r.client, http.MethodPut, "/project/{project_id}/keys/{key_id}", params, map[string]any{"id": state.ID.ValueInt64(), "project_id": state.ProjectID.ValueInt64(), "name": plan.Name.ValueString(), "type": "ssh"}, nil); err != nil {
		resp.Diagnostics.AddError("Error Renaming Semaphore EX Generated SSH Key", err.Error())
		return
	}
	next, err := generatedSSHKeyRead(ctx, r.client, generatedSSHKeyModel{ID: state.ID, ProjectID: state.ProjectID, Login: state.Login, LoginWOVersion: plan.LoginWOVersion})
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Renamed Semaphore EX Generated SSH Key", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

func (r *generatedSSHKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state generatedSSHKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	params, err := generatedSSHKeyParams(state)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Generated SSH Key Identity", err.Error())
		return
	}
	if err = exRequest(ctx, r.client, http.MethodDelete, "/project/{project_id}/keys/{key_id}", params, nil, nil); err != nil && !exNotFound(err) {
		resp.Diagnostics.AddError("Error Deleting Semaphore EX Generated SSH Key", err.Error())
	}
}

func (r *generatedSSHKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 4 || parts[0] != "project" || parts[2] != "generated-ssh-key" {
		resp.Diagnostics.AddError("Invalid Generated SSH Key Import ID", "Use project/<project_id>/generated-ssh-key/<key_id>.")
		return
	}
	projectID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || projectID < 1 {
		resp.Diagnostics.AddError("Invalid Generated SSH Key Import ID", "project_id must be positive.")
		return
	}
	keyID, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil || keyID < 1 {
		resp.Diagnostics.AddError("Invalid Generated SSH Key Import ID", "key_id must be positive.")
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &generatedSSHKeyModel{ID: types.Int64Value(keyID), ProjectID: types.Int64Value(projectID), Login: types.StringNull()})...)
}

func (d *generatedSSHKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config generatedSSHKeyDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	next, err := generatedSSHKeyRead(ctx, d.client, generatedSSHKeyModel{ID: config.ID, ProjectID: config.ProjectID, Login: types.StringNull()})
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Semaphore EX Generated SSH Key", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &generatedSSHKeyDataModel{ID: next.ID, ProjectID: next.ProjectID, Name: next.Name, Algorithm: next.Algorithm, PublicKey: next.PublicKey, Fingerprint: next.Fingerprint})...)
}
