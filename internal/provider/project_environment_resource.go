package provider

import (
	"context"
	"encoding/json"
	"fmt"
	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client/variable_group"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"sort"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                   = &projectEnvironmentResource{}
	_ resource.ResourceWithConfigure      = &projectEnvironmentResource{}
	_ resource.ResourceWithImportState    = &projectEnvironmentResource{}
	_ resource.ResourceWithModifyPlan     = &projectEnvironmentResource{}
	_ resource.ResourceWithValidateConfig = &projectEnvironmentResource{}
)

func NewProjectEnvironmentResource() resource.Resource {
	return &projectEnvironmentResource{}
}

type projectEnvironmentResource struct {
	client *apiclient.SemaphoreUI
}

func (r *projectEnvironmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*apiclient.SemaphoreUI)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			"Expected *client.SemaphoreUI, got %T. Please report this issue to the provider developers.",
		)
		return
	}
	r.client = client
}

func (r *projectEnvironmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_environment"
}

func (model ProjectEnvironmentModel) SecretValue(ctx context.Context, name string, varType string) types.String {
	if model.Secrets.IsNull() || model.Secrets.IsUnknown() {
		return types.StringValue("")
	}
	var secrets []ProjectEnvironmentSecretModel
	diags := model.Secrets.ElementsAs(ctx, &secrets, false)
	if diags.HasError() {
		return types.StringValue("")
	}
	for _, secret := range secrets {
		if secret.Name.Equal(types.StringValue(name)) && secret.Type.Equal(types.StringValue(varType)) {
			return secret.Value
		}
	}
	return types.StringValue("")
}

func (model ProjectEnvironmentModel) Secret(ctx context.Context, id types.Int64) *ProjectEnvironmentSecretModel {
	if model.Secrets.IsNull() || model.Secrets.IsUnknown() {
		return nil
	}

	var secrets []ProjectEnvironmentSecretModel
	diags := model.Secrets.ElementsAs(ctx, &secrets, false)
	if diags.HasError() {
		return nil
	}

	for _, secret := range secrets {
		if secret.ID.Equal(id) {
			return &secret
		}
	}
	return nil
}

func (r *projectEnvironmentResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = ProjectEnvironmentSchema().GetResource(ctx)
}

func (r *projectEnvironmentResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config ProjectEnvironmentModel
	// Variables and environment maps are unrelated to these checks and may
	// contain unknowns that the resolved CRUD model cannot represent.
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("secrets"), &config.Secrets)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("sync_paths"), &config.SyncPaths)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("sync_enabled"), &config.SyncEnabled)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("sync_interval"), &config.SyncInterval)...)
	var storage types.Object
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("secret_storage"), &storage)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !storage.IsNull() && !storage.IsUnknown() {
		resp.Diagnostics.Append(tfsdk.ValueAs(ctx, storage, &config.SecretStorage)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	validateProjectEnvironmentConfig(ctx, config, &resp.Diagnostics)
}

func (r *projectEnvironmentResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var syncPaths types.List
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("sync_paths"), &syncPaths)...)
	if resp.Diagnostics.HasError() || syncPaths.IsNull() || syncPaths.IsUnknown() || len(syncPaths.Elements()) == 0 {
		return
	}
	var storage types.Object
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("secret_storage"), &storage)...)
	if resp.Diagnostics.HasError() || storage.IsUnknown() {
		return
	}
	var storageID types.Int64
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("secret_storage").AtName("id"), &storageID)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if storageID.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("sync_paths"), "Missing secret storage", "sync_paths require secret_storage.id.")
	}
}

func configured(value types.String) bool {
	return !value.IsNull() && !value.IsUnknown()
}

func validateProjectEnvironmentConfig(ctx context.Context, config ProjectEnvironmentModel, diagnostics *diag.Diagnostics) {
	if !config.Secrets.IsNull() && !config.Secrets.IsUnknown() {
		var secrets []types.Object
		diagnostics.Append(config.Secrets.ElementsAs(ctx, &secrets, false)...)
		for index, value := range secrets {
			if value.IsNull() || value.IsUnknown() {
				continue
			}
			var secret ProjectEnvironmentSecretModel
			diags := tfsdk.ValueAs(ctx, value, &secret)
			diagnostics.Append(diags...)
			if diags.HasError() {
				continue
			}
			secretPath := path.Root("secrets").AtListIndex(index)
			hasStorage := !secret.StorageID.IsNull() && !secret.StorageID.IsUnknown()
			hasReferencePart := hasStorage || configured(secret.Mount) || configured(secret.Path) || configured(secret.Field) || (!secret.Version.IsNull() && !secret.Version.IsUnknown())
			hasValue := configured(secret.Value)

			if hasStorage && hasValue {
				diagnostics.AddAttributeError(secretPath, "Conflicting secret sources", "Set either value for a plaintext secret or storage_id with its remote reference, not both.")
			}
			if secret.StorageID.IsNull() && hasReferencePart {
				diagnostics.AddAttributeError(secretPath, "Incomplete remote secret reference", "mount, path, version, and field require storage_id.")
			}
			if !hasStorage && !hasValue && !hasReferencePart &&
				!secret.StorageID.IsUnknown() && !secret.Value.IsUnknown() &&
				!secret.Mount.IsUnknown() && !secret.Path.IsUnknown() &&
				!secret.Field.IsUnknown() && !secret.Version.IsUnknown() {
				diagnostics.AddAttributeError(secretPath, "Missing secret source", "Set value for a plaintext secret or storage_id, path, and field for a remote secret reference.")
			}
			missingPath := !secret.Path.IsUnknown() && (!configured(secret.Path) || secret.Path.ValueString() == "")
			missingField := !secret.Field.IsUnknown() && (!configured(secret.Field) || secret.Field.ValueString() == "")
			if hasStorage && (missingPath || missingField) {
				diagnostics.AddAttributeError(secretPath, "Incomplete remote secret reference", "Remote secret references require both path and field.")
			}
		}
	}
	if config.SecretStorage != nil && config.SecretStorage.ID.IsNull() && configured(config.SecretStorage.KeyPrefix) {
		diagnostics.AddAttributeError(path.Root("secret_storage"), "Incomplete secret storage", "secret_storage.key_prefix requires secret_storage.id.")
	}

	if config.SyncEnabled.ValueBool() && !config.SyncInterval.IsUnknown() && !config.SyncInterval.IsNull() && config.SyncInterval.ValueInt64() <= 0 {
		diagnostics.AddAttributeError(path.Root("sync_interval"), "Invalid synchronization interval", "sync_interval must be positive when sync_enabled is true.")
	}
	if !config.SyncPaths.IsNull() && !config.SyncPaths.IsUnknown() {
		var syncPaths []types.Object
		diagnostics.Append(config.SyncPaths.ElementsAs(ctx, &syncPaths, false)...)
		for index, value := range syncPaths {
			if value.IsNull() || value.IsUnknown() {
				continue
			}
			var syncPath ProjectEnvironmentSyncPathModel
			diags := tfsdk.ValueAs(ctx, value, &syncPath)
			diagnostics.Append(diags...)
			if diags.HasError() {
				continue
			}
			pathValue := path.Root("sync_paths").AtListIndex(index)
			if syncPath.AccessKeyID.IsNull() || (!syncPath.AccessKeyID.IsUnknown() && syncPath.AccessKeyID.ValueInt64() <= 0) ||
				(!syncPath.Mount.IsUnknown() && (!configured(syncPath.Mount) || syncPath.Mount.ValueString() == "")) ||
				(!syncPath.Path.IsUnknown() && (!configured(syncPath.Path) || syncPath.Path.ValueString() == "")) ||
				(!syncPath.Field.IsUnknown() && (!configured(syncPath.Field) || syncPath.Field.ValueString() == "")) {
				diagnostics.AddAttributeError(pathValue, "Incomplete synchronization path", "sync_paths entries require access_key_id, mount, path, and field.")
			}
		}
	}
}

func projectEnvironmentUpdateBody(ctx context.Context, plan, state ProjectEnvironmentModel) (map[string]any, error) {
	request := convertProjectEnvironmentModelToEnvironmentRequest(ctx, plan, &state)
	encoded, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal environment update: %w", err)
	}
	var body map[string]any
	if err := json.Unmarshal(encoded, &body); err != nil {
		return nil, fmt.Errorf("unmarshal environment update: %w", err)
	}
	if plan.SecretStorage != nil && plan.SecretStorage.ID.IsNull() {
		body["secret_storage_id"] = nil
		body["secret_storage_key_prefix"] = nil
	}
	return body, nil
}

func convertProjectEnvironmentModelToEnvironmentRequest(ctx context.Context, env ProjectEnvironmentModel, prev *ProjectEnvironmentModel) *models.EnvironmentRequest {
	model := models.EnvironmentRequest{
		ProjectID: env.ProjectID.ValueInt64(),
		Name:      env.Name.ValueString(),
	}
	if !env.ID.IsNull() && !env.ID.IsUnknown() {
		model.ID = env.ID.ValueInt64()
	}

	if env.Variables == nil {
		model.JSON = "{}"
	} else {
		bytes, _ := json.Marshal(env.Variables)
		model.JSON = string(bytes)
	}

	if env.Environment == nil {
		model.Env = "{}"
	} else {
		bytes, _ := json.Marshal(env.Environment)
		model.Env = string(bytes)
	}
	if env.SecretStorage != nil {
		if !env.SecretStorage.ID.IsNull() && !env.SecretStorage.ID.IsUnknown() {
			storageID := env.SecretStorage.ID.ValueInt64()
			model.SecretStorageID = &storageID
		}
		if !env.SecretStorage.KeyPrefix.IsNull() && !env.SecretStorage.KeyPrefix.IsUnknown() {
			prefix := env.SecretStorage.KeyPrefix.ValueString()
			model.SecretStorageKeyPrefix = &prefix
		}
	}
	if !env.SyncEnabled.IsNull() && !env.SyncEnabled.IsUnknown() {
		model.SyncEnabled = env.SyncEnabled.ValueBool()
	}
	if !env.SyncInterval.IsNull() && !env.SyncInterval.IsUnknown() {
		model.SyncInterval = env.SyncInterval.ValueInt64()
	}
	if !env.SyncPaths.IsNull() && !env.SyncPaths.IsUnknown() {
		var syncPaths []ProjectEnvironmentSyncPathModel
		env.SyncPaths.ElementsAs(ctx, &syncPaths, false)
		model.SyncPaths = make([]*models.SecretSyncPath, 0, len(syncPaths))
		for _, syncPath := range syncPaths {
			model.SyncPaths = append(model.SyncPaths, &models.SecretSyncPath{
				ID:            syncPath.ID.ValueInt64(),
				Path:          syncPath.Path.ValueString(),
				Prefix:        syncPath.Prefix.ValueString(),
				Separator:     syncPath.Separator.ValueString(),
				AccessKeyID:   syncPath.AccessKeyID.ValueInt64(),
				Mount:         syncPath.Mount.ValueString(),
				Field:         syncPath.Field.ValueString(),
				RemoteVersion: syncPath.RemoteVersion.ValueInt64(),
			})
		}
	}

	var secrets []*models.EnvironmentSecretRequest
	var envSecrets, prevSecrets []ProjectEnvironmentSecretModel
	if env.Secrets.IsNull() || env.Secrets.IsUnknown() {
		envSecrets = []ProjectEnvironmentSecretModel{}
	} else {
		env.Secrets.ElementsAs(ctx, &envSecrets, false)
	}
	if prev.Secrets.IsUnknown() || prev.Secrets.IsNull() {
		prevSecrets = []ProjectEnvironmentSecretModel{}
	} else {
		prev.Secrets.ElementsAs(ctx, &prevSecrets, false)
	}

	for _, secret := range envSecrets {
		modelSecret := models.EnvironmentSecretRequest{
			Name: secret.Name.ValueString(),
			Type: secret.Type.ValueString(),
		}
		if !secret.StorageID.IsNull() && !secret.StorageID.IsUnknown() {
			storageID := secret.StorageID.ValueInt64()
			modelSecret.StorageID = &storageID
			modelSecret.Mount = secret.Mount.ValueString()
			modelSecret.Path = secret.Path.ValueString()
			modelSecret.Version = secret.Version.ValueInt64()
			modelSecret.Field = secret.Field.ValueString()
		}
		// Create all secrets from env missing an ID
		if secret.ID.IsUnknown() || secret.ID.IsNull() {
			modelSecret.Operation = "create"
			if modelSecret.StorageID == nil {
				modelSecret.Secret = secret.Value.ValueString()
			}
		} else {
			modelSecret.ID = secret.ID.ValueInt64()
			// Find the previous secret
			prevSecret := prev.Secret(ctx, secret.ID)
			if prevSecret != nil {
				// Update if any field has changed.
				// Note: the Semaphore API ignores type changes on update — only
				// name/secret are persisted. The schema treats `type` as
				// RequiresReplace on the secret list element to prevent the silent
				// no-op.
				if !secret.Name.Equal(prevSecret.Name) || !secret.Value.Equal(prevSecret.Value) || !secret.Type.Equal(prevSecret.Type) || !secret.StorageID.Equal(prevSecret.StorageID) || !secret.Mount.Equal(prevSecret.Mount) || !secret.Path.Equal(prevSecret.Path) || !secret.Version.Equal(prevSecret.Version) || !secret.Field.Equal(prevSecret.Field) {
					modelSecret.Operation = "update"
					if !secret.Name.Equal(prevSecret.Name) {
						modelSecret.Name = secret.Name.ValueString()
					}
					if modelSecret.StorageID == nil && !secret.Value.Equal(prevSecret.Value) {
						modelSecret.Secret = secret.Value.ValueString()
					}
					if !secret.Type.Equal(prevSecret.Type) {
						modelSecret.Type = secret.Type.ValueString()
					}
				}
			}
		}
		secrets = append(secrets, &modelSecret)
	}

	// Delete all secrets from prev with an ID missing from env
	for _, prevSecret := range prevSecrets {
		secret := env.Secret(ctx, prevSecret.ID)
		if secret == nil {
			secrets = append(secrets, &models.EnvironmentSecretRequest{
				ID: prevSecret.ID.ValueInt64(),
				// Can't delete a secret without sending the Type
				Type:      prevSecret.Type.ValueString(),
				Operation: "delete",
			})
		}
	}

	model.Secrets = secrets

	return &model
}

var _ sort.Interface = ByEnvironmentID{}

type ByEnvironmentID []*models.EnvironmentSecret

func (a ByEnvironmentID) Len() int           { return len(a) }
func (a ByEnvironmentID) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByEnvironmentID) Less(i, j int) bool { return a[i].ID < a[j].ID }

func convertEnvironmentResponseToProjectEnvironmentModel(ctx context.Context, environment *models.Environment, prev *ProjectEnvironmentModel) ProjectEnvironmentModel {
	model := ProjectEnvironmentModel{
		ID:        types.Int64Value(environment.ID),
		ProjectID: types.Int64Value(environment.ProjectID),
		Name:      types.StringValue(environment.Name),
	}
	if environment.SecretStorageID != nil || environment.SecretStorageKeyPrefix != nil {
		model.SecretStorage = &ProjectEnvironmentSecretStorageModel{
			ID:        types.Int64Null(),
			KeyPrefix: types.StringNull(),
		}
		if environment.SecretStorageID != nil {
			model.SecretStorage.ID = types.Int64Value(*environment.SecretStorageID)
		}
		if environment.SecretStorageKeyPrefix != nil {
			model.SecretStorage.KeyPrefix = types.StringValue(*environment.SecretStorageKeyPrefix)
		}
	} else if prev != nil && prev.SecretStorage != nil && prev.SecretStorage.ID.IsNull() {
		// `secret_storage = {}` is an explicit clear. The API returns no
		// binding fields afterwards, while Terraform must retain the configured
		// empty object rather than replacing it with a null object.
		model.SecretStorage = prev.SecretStorage
	}
	model.SyncEnabled = types.BoolValue(environment.SyncEnabled)
	model.SyncInterval = types.Int64Value(environment.SyncInterval)

	if json.Unmarshal([]byte(environment.JSON), &model.Variables) != nil {
		model.Variables = &map[string]string{}
	}
	if len(*model.Variables) == 0 && prev.Variables == nil {
		model.Variables = nil
	}

	if json.Unmarshal([]byte(environment.Env), &model.Environment) != nil {
		model.Environment = &map[string]string{}
	}
	if len(*model.Environment) == 0 && prev.Environment == nil {
		model.Environment = nil
	}

	sort.Sort(ByEnvironmentID(environment.Secrets))

	var secrets []ProjectEnvironmentSecretModel
	for _, secret := range environment.Secrets {
		modelSecret := ProjectEnvironmentSecretModel{
			ID:   types.Int64Value(secret.ID),
			Type: types.StringValue(secret.Type),
			Name: types.StringValue(secret.Name),
		}
		if secret.StorageID != nil {
			modelSecret.StorageID = types.Int64Value(*secret.StorageID)
			modelSecret.Mount = types.StringValue(secret.Mount)
			modelSecret.Path = types.StringValue(secret.Path)
			modelSecret.Version = types.Int64Value(secret.Version)
			modelSecret.Field = types.StringValue(secret.Field)
		} else {
			modelSecret.StorageID = types.Int64Null()
			modelSecret.Mount = types.StringNull()
			modelSecret.Path = types.StringNull()
			modelSecret.Version = types.Int64Null()
			modelSecret.Field = types.StringNull()
		}
		// Remote references never return plaintext. Keep their value absent from
		// state; plaintext secrets retain the configured value as before.
		if secret.StorageID != nil {
			modelSecret.Value = types.StringNull()
		} else {
			prevSecret := prev.Secret(ctx, modelSecret.ID)
			if prevSecret != nil {
				modelSecret.Value = prevSecret.Value
			} else {
				modelSecret.Value = prev.SecretValue(ctx, secret.Name, secret.Type)
			}
		}
		secrets = append(secrets, modelSecret)
	}
	if len(secrets) == 0 && !prev.Secrets.IsNull() && !prev.Secrets.IsUnknown() {
		prev.Secrets.ElementsAs(ctx, &secrets, false)
	}

	envSecrets, _ := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: projectEnvironmentSecretAttributeTypes()}, secrets)

	model.Secrets = envSecrets

	syncPaths := make([]ProjectEnvironmentSyncPathModel, 0, len(environment.SyncPaths))
	for _, syncPath := range environment.SyncPaths {
		syncPaths = append(syncPaths, ProjectEnvironmentSyncPathModel{
			ID:            types.Int64Value(syncPath.ID),
			Path:          types.StringValue(syncPath.Path),
			Prefix:        types.StringValue(syncPath.Prefix),
			Separator:     types.StringValue(syncPath.Separator),
			AccessKeyID:   types.Int64Value(syncPath.AccessKeyID),
			Mount:         types.StringValue(syncPath.Mount),
			Field:         types.StringValue(syncPath.Field),
			RemoteVersion: types.Int64Value(syncPath.RemoteVersion),
		})
	}
	if len(syncPaths) == 0 && (prev == nil || prev.SyncPaths.IsNull()) {
		model.SyncPaths = types.ListNull(types.ObjectType{AttrTypes: projectEnvironmentSyncPathAttributeTypes()})
	} else {
		model.SyncPaths, _ = types.ListValueFrom(ctx, types.ObjectType{AttrTypes: projectEnvironmentSyncPathAttributeTypes()}, syncPaths)
	}

	return model
}

func projectEnvironmentSyncPathAttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id": types.Int64Type, "path": types.StringType, "prefix": types.StringType,
		"separator": types.StringType, "access_key_id": types.Int64Type, "mount": types.StringType,
		"field": types.StringType, "remote_version": types.Int64Type,
	}
}

func projectEnvironmentSecretAttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id": types.Int64Type, "type": types.StringType, "name": types.StringType, "value": types.StringType,
		"storage_id": types.Int64Type, "mount": types.StringType, "path": types.StringType,
		"version": types.Int64Type, "field": types.StringType,
	}
}

func (r *projectEnvironmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan ProjectEnvironmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Create new projectEnvironment
	response, err := r.client.VariableGroup.PostProjectProjectIDEnvironment(&variable_group.PostProjectProjectIDEnvironmentParams{
		ProjectID:   plan.ProjectID.ValueInt64(),
		Environment: convertProjectEnvironmentModelToEnvironmentRequest(ctx, plan, &ProjectEnvironmentModel{}),
	}, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating SemaphoreUI Project Environment",
			"Could not create project environment, unexpected error: "+err.Error(),
		)
		return
	}

	payload, err := r.client.VariableGroup.GetProjectProjectIDEnvironmentEnvironmentID(&variable_group.GetProjectProjectIDEnvironmentEnvironmentIDParams{
		ProjectID:     response.Payload.ProjectID,
		EnvironmentID: response.Payload.ID,
	}, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading SemaphoreUI Project Environment",
			"Could not read project environment, unexpected error: "+err.Error(),
		)
		return
	}
	plan = convertEnvironmentResponseToProjectEnvironmentModel(ctx, payload.Payload, &plan)

	// Set state to fully populated data
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *projectEnvironmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state ProjectEnvironmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	response, err := r.client.VariableGroup.GetProjectProjectIDEnvironmentEnvironmentID(&variable_group.GetProjectProjectIDEnvironmentEnvironmentIDParams{
		ProjectID:     state.ProjectID.ValueInt64(),
		EnvironmentID: state.ID.ValueInt64(),
	}, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading SemaphoreUI Project Environment",
			"Could not read project environment, unexpected error: "+err.Error(),
		)
		return
	}
	model := convertEnvironmentResponseToProjectEnvironmentModel(ctx, response.Payload, &state)

	// Set refreshed state
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *projectEnvironmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan, state ProjectEnvironmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, err := projectEnvironmentUpdateBody(ctx, plan, state)
	if err != nil {
		resp.Diagnostics.AddError("Error Encoding SemaphoreUI Project Environment", err.Error())
		return
	}
	err = exRequest(ctx, r.client, "PUT", "/project/{project_id}/environment/{environment_id}", map[string]string{
		"project_id":     strconv.FormatInt(plan.ProjectID.ValueInt64(), 10),
		"environment_id": strconv.FormatInt(plan.ID.ValueInt64(), 10),
	}, body, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating SemaphoreUI Project Key",
			"Could not update project key, unexpected error: "+err.Error(),
		)
		return
	}

	response, err := r.client.VariableGroup.GetProjectProjectIDEnvironmentEnvironmentID(&variable_group.GetProjectProjectIDEnvironmentEnvironmentIDParams{
		ProjectID:     plan.ProjectID.ValueInt64(),
		EnvironmentID: plan.ID.ValueInt64(),
	}, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading SemaphoreUI Project Environment",
			"Could not read project environment, unexpected error: "+err.Error(),
		)
		return
	}
	model := convertEnvironmentResponseToProjectEnvironmentModel(ctx, response.Payload, &plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *projectEnvironmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state ProjectEnvironmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete existing resource
	_, err := r.client.VariableGroup.DeleteProjectProjectIDEnvironmentEnvironmentID(&variable_group.DeleteProjectProjectIDEnvironmentEnvironmentIDParams{
		ProjectID:     state.ProjectID.ValueInt64(),
		EnvironmentID: state.ID.ValueInt64(),
	}, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Semaphore Project Environment",
			fmt.Sprintf("Could not delete project environment, unexpected error: %s", err.Error()),
		)
		return
	}
}

func (r *projectEnvironmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	fields, err := parseImportFields(req.ID, []string{"project", "environment"})
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Project Environment Import ID",
			"Could not parse import ID: "+err.Error(),
		)
		return
	}

	response, err := r.client.VariableGroup.GetProjectProjectIDEnvironmentEnvironmentID(&variable_group.GetProjectProjectIDEnvironmentEnvironmentIDParams{
		ProjectID:     fields["project"],
		EnvironmentID: fields["environment"],
	}, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading SemaphoreUI Project Environment",
			"Could not read project environment, unexpected error: "+err.Error(),
		)
		return
	}
	model := convertEnvironmentResponseToProjectEnvironmentModel(ctx, response.Payload, &ProjectEnvironmentModel{})

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
}
