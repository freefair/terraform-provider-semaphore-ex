package provider

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type exSecretOperationActionKind string

const (
	exSecretStorageSync           exSecretOperationActionKind = "project_secret_storage_sync"
	exSecretStorageConnectionTest exSecretOperationActionKind = "project_secret_storage_connection_test"
	exProjectEnvironmentSync      exSecretOperationActionKind = "project_environment_sync"
	exGeneratedSSHKeyRotate       exSecretOperationActionKind = "project_generated_ssh_key_rotate"
)

// exSecretOperationAction invokes a fixed runtime-secret command. It exposes
// no response attributes, so provider operations never retain secret material.
type exSecretOperationAction struct {
	client *apiclient.SemaphoreUI
	kind   exSecretOperationActionKind
}

func NewProjectSecretStorageSyncAction() action.Action {
	return &exSecretOperationAction{kind: exSecretStorageSync}
}

func NewProjectSecretStorageConnectionTestAction() action.Action {
	return &exSecretOperationAction{kind: exSecretStorageConnectionTest}
}

func NewProjectEnvironmentSyncAction() action.Action {
	return &exSecretOperationAction{kind: exProjectEnvironmentSync}
}

func NewProjectGeneratedSSHKeyRotateAction() action.Action {
	return &exSecretOperationAction{kind: exGeneratedSSHKeyRotate}
}

func (a *exSecretOperationAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + string(a.kind)
}

func (a *exSecretOperationAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Secret Operation Action Configure Type", "Expected the configured Semaphore EX client.")
		return
	}
	a.client = client
}

func (a *exSecretOperationAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: a.description(), Attributes: a.attributes()}
}

func (a *exSecretOperationAction) description() string {
	switch a.kind {
	case exSecretStorageSync:
		return "Requests one project secret-storage synchronization using a caller-chosen idempotency key. It returns after the server accepts or completes the operation and does not retry, poll, or expose secret values."
	case exSecretStorageConnectionTest:
		return "Tests one project secret-storage connection explicitly. The action does not retain or expose provider credentials, tokens, or connection response details."
	case exProjectEnvironmentSync:
		return "Synchronizes configured project-environment secret references explicitly. The action returns after the server completes the request and does not expose secret values."
	case exGeneratedSSHKeyRotate:
		return "Rotates one server-generated project SSH key only after explicit confirmation. The server returns public metadata only; this action discards all response material and never exposes private key data."
	default:
		return "Invokes a fixed Semaphore EX runtime-secret operation explicitly."
	}
}

func (a *exSecretOperationAction) attributes() map[string]schema.Attribute {
	switch a.kind {
	case exSecretStorageSync:
		return map[string]schema.Attribute{
			"project_id":           runtimeProjectAttribute(),
			"storage_id":           runtimeIDAttribute("Secret storage to synchronize."),
			"request_id":           schema.StringAttribute{Required: true, MarkdownDescription: "Caller-chosen durable idempotency key for this synchronization request.", Validators: []validator.String{stringvalidator.LengthAtLeast(1)}},
			"resolve_operation_id": schema.Int64Attribute{Optional: true, MarkdownDescription: "Optional conflicting operation ID to resolve as part of this request.", Validators: []validator.Int64{int64validator.AtLeast(1)}},
		}
	case exSecretStorageConnectionTest:
		return map[string]schema.Attribute{"project_id": runtimeProjectAttribute(), "storage_id": runtimeIDAttribute("Secret storage connection to test.")}
	case exProjectEnvironmentSync:
		return map[string]schema.Attribute{"project_id": runtimeProjectAttribute(), "environment_id": runtimeIDAttribute("Environment whose configured secret references will synchronize.")}
	case exGeneratedSSHKeyRotate:
		return map[string]schema.Attribute{
			"project_id":       runtimeProjectAttribute(),
			"key_id":           runtimeIDAttribute("Server-generated SSH key to rotate."),
			"algorithm":        schema.StringAttribute{Required: true, MarkdownDescription: "Algorithm for the replacement generated SSH key.", Validators: []validator.String{stringvalidator.OneOf("ed25519", "rsa-3072")}},
			"confirm_rotation": schema.BoolAttribute{Required: true, MarkdownDescription: "Must be true to confirm that the existing generated SSH key will be replaced."},
		}
	default:
		return nil
	}
}

func (a *exSecretOperationAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	switch a.kind {
	case exSecretStorageSync:
		a.invokeStorageSync(ctx, req, resp)
	case exSecretStorageConnectionTest:
		a.invokeStorageConnectionTest(ctx, req, resp)
	case exProjectEnvironmentSync:
		a.invokeEnvironmentSync(ctx, req, resp)
	case exGeneratedSSHKeyRotate:
		a.invokeGeneratedSSHKeyRotate(ctx, req, resp)
	default:
		resp.Diagnostics.AddError("Unsupported Secret Operation Action", "The action has no configured secret operation.")
	}
}

type exSecretStorageSyncActionModel struct {
	ProjectID          types.Int64  `tfsdk:"project_id"`
	StorageID          types.Int64  `tfsdk:"storage_id"`
	RequestID          types.String `tfsdk:"request_id"`
	ResolveOperationID types.Int64  `tfsdk:"resolve_operation_id"`
}

func (a *exSecretOperationAction) invokeStorageSync(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config exSecretStorageSyncActionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || !runtimePositive(resp, "project_id", config.ProjectID) || !runtimePositive(resp, "storage_id", config.StorageID) {
		return
	}
	if config.RequestID.IsNull() || config.RequestID.IsUnknown() || strings.TrimSpace(config.RequestID.ValueString()) == "" {
		resp.Diagnostics.AddError("Invalid Secret Storage Sync Request ID", "request_id must be a known non-empty caller-chosen idempotency key.")
		return
	}
	body := map[string]any{"request_id": config.RequestID.ValueString()}
	if !config.ResolveOperationID.IsNull() {
		if !runtimePositive(resp, "resolve_operation_id", config.ResolveOperationID) {
			return
		}
		body["resolve_operation_id"] = config.ResolveOperationID.ValueInt64()
	}
	if err := exRequest(ctx, a.client, http.MethodPost, "/project/{project_id}/secret_storages/{storage_id}/sync", map[string]string{"project_id": strconv.FormatInt(config.ProjectID.ValueInt64(), 10), "storage_id": strconv.FormatInt(config.StorageID.ValueInt64(), 10)}, body, nil); err != nil {
		resp.Diagnostics.AddError("Error Synchronizing Semaphore EX Secret Storage", err.Error())
		return
	}
	runtimeProgress(resp, "Secret-storage synchronization request accepted.")
}

type exSecretStorageConnectionTestActionModel struct {
	ProjectID types.Int64 `tfsdk:"project_id"`
	StorageID types.Int64 `tfsdk:"storage_id"`
}

func (a *exSecretOperationAction) invokeStorageConnectionTest(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config exSecretStorageConnectionTestActionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || !runtimePositive(resp, "project_id", config.ProjectID) || !runtimePositive(resp, "storage_id", config.StorageID) {
		return
	}
	if err := exRequest(ctx, a.client, http.MethodPost, "/project/{project_id}/secret_storages/{storage_id}/test", map[string]string{"project_id": strconv.FormatInt(config.ProjectID.ValueInt64(), 10), "storage_id": strconv.FormatInt(config.StorageID.ValueInt64(), 10)}, nil, nil); err != nil {
		resp.Diagnostics.AddError("Error Testing Semaphore EX Secret Storage Connection", err.Error())
		return
	}
	runtimeProgress(resp, "Secret-storage connection test completed.")
}

type exProjectEnvironmentSyncActionModel struct {
	ProjectID     types.Int64 `tfsdk:"project_id"`
	EnvironmentID types.Int64 `tfsdk:"environment_id"`
}

func (a *exSecretOperationAction) invokeEnvironmentSync(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config exProjectEnvironmentSyncActionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || !runtimePositive(resp, "project_id", config.ProjectID) || !runtimePositive(resp, "environment_id", config.EnvironmentID) {
		return
	}
	if err := exRequest(ctx, a.client, http.MethodPost, "/project/{project_id}/environment/{environment_id}/sync", map[string]string{"project_id": strconv.FormatInt(config.ProjectID.ValueInt64(), 10), "environment_id": strconv.FormatInt(config.EnvironmentID.ValueInt64(), 10)}, nil, nil); err != nil {
		resp.Diagnostics.AddError("Error Synchronizing Semaphore EX Project Environment", err.Error())
		return
	}
	runtimeProgress(resp, "Project-environment secret synchronization completed.")
}

type exGeneratedSSHKeyRotateActionModel struct {
	ProjectID       types.Int64  `tfsdk:"project_id"`
	KeyID           types.Int64  `tfsdk:"key_id"`
	Algorithm       types.String `tfsdk:"algorithm"`
	ConfirmRotation types.Bool   `tfsdk:"confirm_rotation"`
}

func (a *exSecretOperationAction) invokeGeneratedSSHKeyRotate(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config exGeneratedSSHKeyRotateActionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || !runtimePositive(resp, "project_id", config.ProjectID) || !runtimePositive(resp, "key_id", config.KeyID) {
		return
	}
	if config.Algorithm.IsNull() || config.Algorithm.IsUnknown() || (config.Algorithm.ValueString() != "ed25519" && config.Algorithm.ValueString() != "rsa-3072") {
		resp.Diagnostics.AddError("Invalid Generated SSH Key Algorithm", "algorithm must be ed25519 or rsa-3072.")
		return
	}
	if config.ConfirmRotation.IsNull() || config.ConfirmRotation.IsUnknown() || !config.ConfirmRotation.ValueBool() {
		resp.Diagnostics.AddError("Generated SSH Key Rotation Not Confirmed", "confirm_rotation must be explicitly true before a generated SSH key can be replaced.")
		return
	}
	body := map[string]any{"algorithm": config.Algorithm.ValueString(), "confirm_rotation": true}
	if err := exRequest(ctx, a.client, http.MethodPost, "/project/{project_id}/keys/{key_id}/rotate", map[string]string{"project_id": strconv.FormatInt(config.ProjectID.ValueInt64(), 10), "key_id": strconv.FormatInt(config.KeyID.ValueInt64(), 10)}, body, nil); err != nil {
		resp.Diagnostics.AddError("Error Rotating Semaphore EX Generated SSH Key", err.Error())
		return
	}
	runtimeProgress(resp, "Generated SSH key rotation completed; no private key material was returned.")
}
