package provider

import (
	"context"
	"fmt"
	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client/key_store"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                     = &projectKeyResource{}
	_ resource.ResourceWithConfigure        = &projectKeyResource{}
	_ resource.ResourceWithImportState      = &projectKeyResource{}
	_ resource.ResourceWithConfigValidators = &projectKeyResource{}
	_ resource.ResourceWithValidateConfig   = &projectKeyResource{}
)

func NewProjectKeyResource() resource.Resource {
	return &projectKeyResource{}
}

type projectKeyResource struct {
	client *apiclient.SemaphoreUI
}

func (r *projectKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *projectKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_key"
}

func (r *projectKeyResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = ProjectKeySchema().GetResource(ctx)
}

func (r *projectKeyResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot(ProjectKeyTypeLoginPassword),
			path.MatchRoot(ProjectKeyTypeSSH),
			path.MatchRoot(ProjectKeyTypeNone),
			path.MatchRoot(ProjectKeyTypeString),
		),
		// Within each key type, the persisted secret attribute and its
		// write-only counterpart are mutually exclusive. The user picks one.
		resourcevalidator.Conflicting(
			path.MatchRoot(ProjectKeyTypeLoginPassword).AtName("password"),
			path.MatchRoot(ProjectKeyTypeLoginPassword).AtName("password_wo"),
		),
		resourcevalidator.Conflicting(
			path.MatchRoot(ProjectKeyTypeSSH).AtName("passphrase"),
			path.MatchRoot(ProjectKeyTypeSSH).AtName("passphrase_wo"),
		),
		resourcevalidator.Conflicting(
			path.MatchRoot(ProjectKeyTypeSSH).AtName("private_key"),
			path.MatchRoot(ProjectKeyTypeSSH).AtName("private_key_wo"),
		),
		resourcevalidator.Conflicting(path.MatchRoot(ProjectKeyTypeString).AtName("value"), path.MatchRoot(ProjectKeyTypeString).AtName("value_wo")),
	}
}

func (r *projectKeyResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config ProjectKeyModel
	// Configuration objects can be unknown even though CRUD models only need
	// to represent resolved objects. Decode each known validation input separately.
	for _, field := range []struct {
		name   string
		target any
	}{
		{"remote_reference", &config.RemoteReference},
		{ProjectKeyTypeLoginPassword, &config.LoginPassword},
		{ProjectKeyTypeSSH, &config.SSH},
		{ProjectKeyTypeString, &config.String},
		{ProjectKeyTypeNone, &config.None},
	} {
		var value types.Object
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(field.name), &value)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if !value.IsNull() && !value.IsUnknown() {
			resp.Diagnostics.Append(tfsdk.ValueAs(ctx, value, field.target)...)
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	validateProjectKeyConfig(ctx, config, &resp.Diagnostics)
}

func hasProjectKeyValue(value types.String) bool { return !value.IsNull() && !value.IsUnknown() }

func validateProjectKeyConfig(_ context.Context, config ProjectKeyModel, diagnostics *diag.Diagnostics) {
	if config.RemoteReference == nil {
		return
	}
	ref := config.RemoteReference
	if !ref.Path.IsUnknown() && (!hasProjectKeyValue(ref.Path) || ref.Path.ValueString() == "") {
		diagnostics.AddAttributeError(path.Root("remote_reference").AtName("path"), "Incomplete remote secret reference", "remote_reference requires path.")
	}
	if ref.StorageType.ValueString() == "vault" && ref.StorageID.IsNull() {
		diagnostics.AddAttributeError(path.Root("remote_reference").AtName("storage_id"), "Incomplete remote secret reference", "vault remote_reference requires storage_id.")
	}
	if ref.StorageType.ValueString() == "vault" && !ref.Field.IsUnknown() && (!hasProjectKeyValue(ref.Field) || ref.Field.ValueString() == "") {
		diagnostics.AddAttributeError(path.Root("remote_reference").AtName("field"), "Incomplete remote secret reference", "vault remote_reference requires field.")
	}
	if !ref.StorageType.IsUnknown() && ref.StorageType.ValueString() != "vault" && !ref.StorageID.IsNull() && !ref.StorageID.IsUnknown() {
		diagnostics.AddAttributeError(path.Root("remote_reference").AtName("storage_id"), "Invalid remote secret reference", "storage_id is only valid for vault remote_reference.")
	}
	if config.LoginPassword != nil && (hasProjectKeyValue(config.LoginPassword.Password) || hasProjectKeyValue(config.LoginPassword.PasswordWO)) {
		diagnostics.AddAttributeError(path.Root("login_password"), "Conflicting secret sources", "Set either remote_reference or password/password_wo.")
	}
	if config.SSH != nil && (hasProjectKeyValue(config.SSH.Passphrase) || hasProjectKeyValue(config.SSH.PassphraseWO) || hasProjectKeyValue(config.SSH.PrivateKey) || hasProjectKeyValue(config.SSH.PrivateKeyWO)) {
		diagnostics.AddAttributeError(path.Root("ssh"), "Conflicting secret sources", "Set either remote_reference or SSH secret fields.")
	}
	if config.String != nil && (hasProjectKeyValue(config.String.Value) || hasProjectKeyValue(config.String.ValueWO)) {
		diagnostics.AddAttributeError(path.Root("string"), "Conflicting secret sources", "Set either remote_reference or value/value_wo.")
	}
	if config.None != nil {
		diagnostics.AddAttributeError(path.Root("remote_reference"), "Invalid remote secret reference", "none keys cannot use remote_reference.")
	}
}

// resolvedSecrets holds the plaintext secret values bound for the API.
// The values come from either the persisted attribute (e.g. `password`)
// or its write-only counterpart (`password_wo`), whichever the user set.
// Resolution happens once and is kept out of the Terraform model so the
// write-only inputs never leak into state.
type resolvedSecrets struct {
	password    string
	passphrase  string
	privateKey  string
	stringValue string
}

func resolveSecrets(plan, config *ProjectKeyModel) resolvedSecrets {
	out := resolvedSecrets{}
	if plan.LoginPassword != nil {
		out.password = plan.LoginPassword.Password.ValueString()
		if config.LoginPassword != nil &&
			!config.LoginPassword.PasswordWO.IsNull() &&
			!config.LoginPassword.PasswordWO.IsUnknown() {
			out.password = config.LoginPassword.PasswordWO.ValueString()
		}
	}
	if plan.SSH != nil {
		out.passphrase = plan.SSH.Passphrase.ValueString()
		out.privateKey = plan.SSH.PrivateKey.ValueString()
		if config.SSH != nil {
			if !config.SSH.PassphraseWO.IsNull() && !config.SSH.PassphraseWO.IsUnknown() {
				out.passphrase = config.SSH.PassphraseWO.ValueString()
			}
			if !config.SSH.PrivateKeyWO.IsNull() && !config.SSH.PrivateKeyWO.IsUnknown() {
				out.privateKey = config.SSH.PrivateKeyWO.ValueString()
			}
		}
	}
	if plan.String != nil {
		out.stringValue = plan.String.Value.ValueString()
		if config.String != nil && !config.String.ValueWO.IsNull() && !config.String.ValueWO.IsUnknown() {
			out.stringValue = config.String.ValueWO.ValueString()
		}
	}
	return out
}

func convertProjectKeyModelToAccessKeyRequest(key ProjectKeyModel, secrets resolvedSecrets) *models.AccessKeyRequest {
	model := models.AccessKeyRequest{
		ProjectID: key.ProjectID.ValueInt64(),
		Name:      key.Name.ValueString(),
	}
	if !key.ID.IsNull() && !key.ID.IsUnknown() {
		model.ID = key.ID.ValueInt64()
	}
	if key.None != nil {
		model.Type = ProjectKeyTypeNone
	} else if key.LoginPassword != nil {
		model.Type = ProjectKeyTypeLoginPassword
		model.LoginPassword = &models.AccessKeyRequestLoginPassword{
			Login:    key.LoginPassword.Login.ValueString(),
			Password: secrets.password,
		}
	} else if key.SSH != nil {
		model.Type = ProjectKeyTypeSSH
		model.SSH = &models.AccessKeyRequestSSH{
			Login:      key.SSH.Login.ValueString(),
			Passphrase: secrets.passphrase,
			PrivateKey: secrets.privateKey,
		}
	} else if key.String != nil {
		model.Type = ProjectKeyTypeString
		model.String = secrets.stringValue
	}
	if key.RemoteReference != nil {
		storageType := key.RemoteReference.StorageType.ValueString()
		model.SourceStorageType = &storageType
		if !key.RemoteReference.StorageID.IsNull() && !key.RemoteReference.StorageID.IsUnknown() {
			storageID := key.RemoteReference.StorageID.ValueInt64()
			model.SourceStorageID = &storageID
		}
		pathValue := key.RemoteReference.Path.ValueString()
		model.SourceStorageKey = &pathValue
		model.SourceStorageMount = key.RemoteReference.Mount.ValueString()
		model.SourceStorageVersion = key.RemoteReference.Version.ValueInt64()
		model.SourceStorageField = key.RemoteReference.Field.ValueString()
	}

	return &model
}

func convertAccessKeyResponseToProjectKeyModel(key *models.AccessKey, prev *ProjectKeyModel) ProjectKeyModel {
	model := ProjectKeyModel{
		ID:        types.Int64Value(key.ID),
		ProjectID: types.Int64Value(key.ProjectID),
		Name:      types.StringValue(key.Name),
	}

	if key.SourceStorageType != nil {
		model.RemoteReference = remoteReferenceFromAccessKey(key)
	}
	// SemaphoreUI API never returns literal secret values, so retain prior state.
	switch key.Type {
	case ProjectKeyTypeNone:
		model.None = &ProjectKeyNone{}
	case ProjectKeyTypeLoginPassword:
		model.LoginPassword = prev.LoginPassword
	case ProjectKeyTypeSSH:
		model.SSH = prev.SSH
	case ProjectKeyTypeString:
		model.String = prev.String
	}

	return model
}

func (r *projectKeyResource) getProjectKeyModelFromClient(ctx context.Context, projectId types.Int64, keyId types.Int64, prev *ProjectKeyModel) (*ProjectKeyModel, error) {
	payload, err := r.client.KeyStore.GetProjectProjectIDKeysContext(ctx, &key_store.GetProjectProjectIDKeysParams{
		ProjectID: projectId.ValueInt64(),
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("could not read Keys for project ID %d: %w", projectId.ValueInt64(), err)
	}

	for _, key := range payload.Payload {
		if key.ID == keyId.ValueInt64() {
			model := ProjectKeyModel{
				ProjectID: projectId,
				ID:        keyId,
				Name:      types.StringValue(key.Name),
			}
			switch key.Type {
			case ProjectKeyTypeNone:
				model.None = &ProjectKeyNone{}
			case ProjectKeyTypeLoginPassword:
				model.LoginPassword = prev.LoginPassword
			case ProjectKeyTypeSSH:
				model.SSH = prev.SSH
			case ProjectKeyTypeString:
				model.String = prev.String
			}
			if key.SourceStorageType != nil {
				model.RemoteReference = remoteReferenceFromAccessKey(key)
			}
			return &model, nil
		}
	}
	return nil, &exAPIError{StatusCode: 404}
}

func (r *projectKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan + config. WriteOnly attributes (*_wo) live
	// in Config only — they're excluded from Plan and State by design.
	var plan, config ProjectKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	secrets := resolveSecrets(&plan, &config)

	response, err := r.client.KeyStore.PostProjectProjectIDKeysContext(ctx, &key_store.PostProjectProjectIDKeysParams{
		ProjectID: plan.ProjectID.ValueInt64(),
		AccessKey: convertProjectKeyModelToAccessKeyRequest(plan, secrets),
	}, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating SemaphoreUI Project Key",
			"Could not create project key, unexpected error: "+err.Error(),
		)
		return
	}
	plan = convertAccessKeyResponseToProjectKeyModel(response.Payload, &plan)

	// Set state to fully populated data
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *projectKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state ProjectKeyModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	model, err := r.getProjectKeyModelFromClient(ctx, state.ProjectID, state.ID, &state)
	if resourceNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Semaphore Project Keys",
			err.Error(),
		)
		return
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &model)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *projectKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan, config, and state. WriteOnly values are in
	// Config only — Plan and State have them as null.
	var plan, config, state ProjectKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	secrets := resolveSecrets(&plan, &config)

	// Create an access key based on the plan
	key := convertProjectKeyModelToAccessKeyRequest(plan, secrets)
	// Check if type of key has changed
	if !plan.Type().Equal(state.Type()) {
		// If key type has changed, we must update the secrets
		key.OverrideSecret = true
	} else {
		// Key type has not changed, so we need to check if individual fields have changed
		switch plan.Type().ValueString() {
		case ProjectKeyTypeLoginPassword:
			if !plan.LoginPassword.Login.Equal(state.LoginPassword.Login) ||
				!plan.LoginPassword.Password.Equal(state.LoginPassword.Password) ||
				!plan.LoginPassword.PasswordWOVersion.Equal(state.LoginPassword.PasswordWOVersion) ||
				!plan.Name.Equal(state.Name) {
				key.OverrideSecret = true
			} else {
				// Use empty struct when secrets haven't changed
				key.LoginPassword = &models.AccessKeyRequestLoginPassword{}
			}
		case ProjectKeyTypeSSH:
			if !plan.SSH.Login.Equal(state.SSH.Login) ||
				!plan.SSH.Passphrase.Equal(state.SSH.Passphrase) ||
				!plan.SSH.PassphraseWOVersion.Equal(state.SSH.PassphraseWOVersion) ||
				!plan.SSH.PrivateKey.Equal(state.SSH.PrivateKey) ||
				!plan.SSH.PrivateKeyWOVersion.Equal(state.SSH.PrivateKeyWOVersion) ||
				!plan.Name.Equal(state.Name) {
				key.OverrideSecret = true
			} else {
				// Use empty struct when secrets haven't changed
				key.SSH = &models.AccessKeyRequestSSH{}
			}
		case ProjectKeyTypeString:
			if !plan.String.Value.Equal(state.String.Value) || !plan.String.ValueWOVersion.Equal(state.String.ValueWOVersion) || !plan.Name.Equal(state.Name) || !remoteReferenceEqual(plan.RemoteReference, state.RemoteReference) {
				key.OverrideSecret = true
			} else {
				key.String = ""
			}
		case ProjectKeyTypeNone:
			// type None has no secrets to update, but if Name change, we need to update it
			if !plan.Name.Equal(state.Name) {
				key.OverrideSecret = true
			}
		}
	}
	if !remoteReferenceEqual(plan.RemoteReference, state.RemoteReference) {
		key.OverrideSecret = true
	}
	if key.OverrideSecret {
		// For some reason, the API will only update access keys when all the fields are set
		// So we need to set the other fields to empty strings
		switch key.Type {
		case ProjectKeyTypeLoginPassword:
			key.SSH = &models.AccessKeyRequestSSH{
				Login:      "",
				Passphrase: "",
				PrivateKey: "",
			}
		case ProjectKeyTypeSSH:
			key.LoginPassword = &models.AccessKeyRequestLoginPassword{
				Login:    "",
				Password: "",
			}
		}
	}

	// Update existing resource
	_, err := r.client.KeyStore.PutProjectProjectIDKeysKeyIDContext(ctx, &key_store.PutProjectProjectIDKeysKeyIDParams{
		ProjectID: plan.ProjectID.ValueInt64(),
		KeyID:     plan.ID.ValueInt64(),
		AccessKey: key,
	}, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating SemaphoreUI Project Key",
			"Could not update project key, unexpected error: "+err.Error(),
		)
		return
	}

	// Fetch updated values as PutProjectProjectIDKeysKeyID does not return updated projectKey
	model, err := r.getProjectKeyModelFromClient(ctx, state.ProjectID, state.ID, &plan)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Semaphore Project Keys",
			err.Error(),
		)
		return
	}

	// Update resource state with updated projectKey
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func remoteReferenceFromAccessKey(key *models.AccessKey) *ProjectKeyRemoteReference {
	if key.SourceStorageType == nil {
		return nil
	}
	model := &ProjectKeyRemoteReference{StorageType: types.StringValue(*key.SourceStorageType), Mount: types.StringValue(key.SourceStorageMount), Version: types.Int64Value(key.SourceStorageVersion), Field: types.StringValue(key.SourceStorageField)}
	if key.SourceStorageID != nil {
		model.StorageID = types.Int64Value(*key.SourceStorageID)
	} else {
		model.StorageID = types.Int64Null()
	}
	if key.SourceStorageKey != nil {
		model.Path = types.StringValue(*key.SourceStorageKey)
	} else {
		model.Path = types.StringNull()
	}
	return model
}

func remoteReferenceEqual(left, right *ProjectKeyRemoteReference) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.StorageType.Equal(right.StorageType) && left.StorageID.Equal(right.StorageID) && left.Mount.Equal(right.Mount) && left.Path.Equal(right.Path) && left.Version.Equal(right.Version) && left.Field.Equal(right.Field)
}

func (r *projectKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state ProjectKeyModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete existing resource
	_, err := r.client.KeyStore.DeleteProjectProjectIDKeysKeyIDContext(ctx, &key_store.DeleteProjectProjectIDKeysKeyIDParams{
		ProjectID: state.ProjectID.ValueInt64(),
		KeyID:     state.ID.ValueInt64(),
	}, nil)
	if err != nil && !resourceNotFound(err) {
		resp.Diagnostics.AddError(
			"Error Deleting Semaphore Project Key",
			fmt.Sprintf("Could not delete project key, unexpected error: %s", err.Error()),
		)
		return
	}
}

func (r *projectKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	fields, err := parseImportFields(req.ID, []string{"project", "key"})
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Project Key Import ID",
			"Could not parse import ID: "+err.Error(),
		)
		return
	}

	// Get the project key from the client filling required secrets with empty strings
	model, err := r.getProjectKeyModelFromClient(ctx, types.Int64Value(fields["project"]), types.Int64Value(fields["key"]), &ProjectKeyModel{
		LoginPassword: &ProjectKeyLoginPassword{
			Password: types.StringValue(""),
		},
		SSH: &ProjectKeySSH{
			PrivateKey: types.StringValue(""),
		},
		String: &ProjectKeyString{Value: types.StringValue("")},
		None:   &ProjectKeyNone{},
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Semaphore Project Keys",
			err.Error(),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
}
