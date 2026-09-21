package provider

import (
	"context"
	"encoding/json"
	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client/template"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"net/http"
	"sort"
	"strconv"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                     = &projectTemplateResource{}
	_ resource.ResourceWithConfigure        = &projectTemplateResource{}
	_ resource.ResourceWithImportState      = &projectTemplateResource{}
	_ resource.ResourceWithConfigValidators = &projectTemplateResource{}
)

func NewProjectTemplateResource() resource.Resource {
	return &projectTemplateResource{}
}

type projectTemplateResource struct {
	client *apiclient.SemaphoreUI
}

func (r *projectTemplateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *projectTemplateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_template"
}

func (r *projectTemplateResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = ProjectTemplateSchema().GetResource(ctx)
}

// playbookRequiredValidator enforces that `playbook` is set for apps that
// require it. SemaphoreUI accepts an empty playbook only for `terraform` and
// `tofu` apps; for everything else (ansible, bash, powershell, python, …) the
// API returns 400 "template playbook can not be empty". See issue #26.
//
// The validator runs during ValidateConfig, which executes once per resource
// block before for_each/count expansion. When `app` and/or `playbook` depend
// on `each.value`/`count.index` they appear as Unknown here; in that case the
// check is deferred to per-instance plan-time validation, where the values
// will be known.
type playbookRequiredValidator struct{}

func (v playbookRequiredValidator) Description(_ context.Context) string {
	return "playbook is required unless app is `terraform` or `tofu`"
}

func (v playbookRequiredValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v playbookRequiredValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data ProjectTemplateModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Defer to per-instance validation when either value isn't known yet
	// (e.g. the resource uses for_each/count referencing dynamic data).
	if data.Playbook.IsUnknown() || data.App.IsUnknown() {
		return
	}
	if !data.Playbook.IsNull() && data.Playbook.ValueString() != "" {
		return
	}
	app := data.App.ValueString()
	if data.App.IsNull() {
		app = "ansible" // matches the schema default
	}
	if app == "terraform" || app == "tofu" || app == "terragrunt" {
		return
	}
	resp.Diagnostics.AddAttributeError(
		path.Root("playbook"),
		"Missing playbook",
		"playbook is required when app is not `terraform`, `tofu` or `terragrunt`. Got app="+app+".",
	)
}

func (r *projectTemplateResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{playbookRequiredValidator{}, templateSSHKeysValidator{}, templateSettingsValidator{}, surveyChoicesValidator{}, resourcevalidator.ExactlyOneOf(path.MatchRoot("environment_id"), path.MatchRoot("environment_ids"))}
}

type templateSSHKeysValidator struct{}

func (templateSSHKeysValidator) Description(_ context.Context) string {
	return "ssh_keys inherits without bindings, or selects an explicit bindings list"
}
func (v templateSSHKeysValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (templateSSHKeysValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data ProjectTemplateModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || data.SSHKeys.IsNull() || data.SSHKeys.IsUnknown() {
		return
	}
	values := data.SSHKeys.Attributes()
	inherit, ok := values["inherit"].(types.Bool)
	if !ok || inherit.IsNull() || inherit.IsUnknown() {
		return
	}
	bindings, hasBindings := values["bindings"].(types.List)
	if inherit.ValueBool() {
		if hasBindings && !bindings.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("ssh_keys"), "Invalid SSH Key Selection", "inherit=true must omit bindings.")
		}
		return
	}
	if !hasBindings || bindings.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("ssh_keys"), "Invalid SSH Key Selection", "inherit=false requires bindings, including an explicit empty list.")
	}
}

func convertProjectTemplateModelToTemplateRequest(ctx context.Context, template ProjectTemplateModel) *models.TemplateRequest {
	envIDs := []int64{}
	if !template.EnvironmentIDs.IsNull() && !template.EnvironmentIDs.IsUnknown() {
		template.EnvironmentIDs.ElementsAs(ctx, &envIDs, false)
	}
	// The legacy input changes one group; unchanged legacy configurations retain
	// all groups discovered on refresh rather than silently deleting them.
	if !template.EnvironmentID.IsNull() && !template.EnvironmentID.IsUnknown() {
		legacy := template.EnvironmentID.ValueInt64()
		found := false
		for _, id := range envIDs {
			if id == legacy {
				found = true
			}
		}
		if !found {
			envIDs = []int64{legacy}
		}
	}
	sort.Slice(envIDs, func(i, j int) bool { return envIDs[i] < envIDs[j] })
	model := models.TemplateRequest{
		ProjectID:                 template.ProjectID.ValueInt64(),
		AllowParallelTasks:        template.AllowParallelTasks.ValueBool(),
		AllowOverrideBranchInTask: template.AllowOverrideBranchInTask.ValueBool(),
		JwtParams:                 templateJWTToAPI(ctx, template.JWTParams),
		EnvironmentIds:            envIDs,
		WorkingDirectory:          template.WorkingDirectory.ValueStringPointer(),
		ExecutorImage:             template.ExecutorImage.ValueStringPointer(),
		SuppressErrorAlerts:       template.SuppressErrorAlerts.ValueBool(),
		RunnerTagMatchMode:        template.RunnerTagMatchMode.ValueString(),
		InventoryID:               template.InventoryID.ValueInt64(),
		RepositoryID:              template.RepositoryID.ValueInt64(),
		App:                       template.App.ValueString(),
		Name:                      template.Name.ValueString(),
		Playbook:                  template.Playbook.ValueString(),
		AllowOverrideArgsInTask:   template.AllowOverrideArgsInTask.ValueBool(),
		SuppressSuccessAlerts:     template.SuppressSuccessAlerts.ValueBool(),
	}
	if !template.RunnerTags.IsNull() && !template.RunnerTags.IsUnknown() {
		template.RunnerTags.ElementsAs(ctx, &model.RunnerTags, false)
	}
	if model.WorkingDirectory != nil && *model.WorkingDirectory == "" {
		model.WorkingDirectory = nil
	}
	if model.RunnerTagMatchMode == "" {
		model.RunnerTagMatchMode = "all"
	}
	if !template.ID.IsNull() && !template.ID.IsUnknown() {
		model.ID = template.ID.ValueInt64()
	}

	if !template.Description.IsNull() && !template.Description.IsUnknown() {
		model.Description = template.Description.ValueString()
	}
	if !template.GitBranch.IsNull() && !template.GitBranch.IsUnknown() {
		model.GitBranch = template.GitBranch.ValueString()
	}
	if !template.ViewID.IsNull() && !template.ViewID.IsUnknown() {
		model.ViewID = template.ViewID.ValueInt64()
	}

	if len(template.Arguments.Elements()) != 0 {
		var arguments []string
		template.Arguments.ElementsAs(ctx, &arguments, false)
		bytes, _ := json.Marshal(arguments)
		model.Arguments = string(bytes)
	} else {
		model.Arguments = "[]"
	}

	if template.Build != nil {
		model.Type = "build"
		if !template.Build.StartVersion.IsNull() && !template.Build.StartVersion.IsUnknown() {
			model.StartVersion = template.Build.StartVersion.ValueString()
		}
	}

	if template.Deploy != nil {
		model.Type = "deploy"
		model.BuildTemplateID = template.Deploy.BuildTemplateID.ValueInt64()
		model.Autorun = template.Deploy.Autorun.ValueBool()
	}

	model.SurveyVars = []*models.TemplateSurveyVar{}
	if !template.SurveyVars.IsNull() && !template.SurveyVars.IsUnknown() {
		var surveyVars []ProjectTemplateSurveyVarModel
		template.SurveyVars.ElementsAs(ctx, &surveyVars, false)
		for _, surveyVar := range surveyVars {
			surveyVarModel := models.TemplateSurveyVar{
				Name:         surveyVar.Name.ValueString(),
				Title:        surveyVar.Title.ValueString(),
				Required:     surveyVar.Required.ValueBool(),
				Type:         surveyTypeToAPI(surveyVar.Type.ValueString()),
				Target:       surveyVar.Target.ValueString(),
				DefaultValue: surveyDefaultToAPI(ctx, surveyVar),
			}
			if !surveyVar.Description.IsNull() && !surveyVar.Description.IsUnknown() {
				surveyVarModel.Description = surveyVar.Description.ValueString()
			}
			if surveyVar.Type.ValueString() == "enum" || surveyVar.Type.ValueString() == "select" {
				surveyVarModel.Values = surveyChoicesToAPI(surveyVar)
			}
			model.SurveyVars = append(model.SurveyVars, &surveyVarModel)
		}
	}

	model.Vaults = []*models.TemplateVault{}
	if !template.Vaults.IsNull() && !template.Vaults.IsUnknown() {
		var vaults []ProjectTemplateVaultModel
		template.Vaults.ElementsAs(ctx, &vaults, false)
		for _, vault := range vaults {
			vaultModel := models.TemplateVault{
				Name: vault.Name.ValueString(),
			}
			if !vault.ID.IsNull() && !vault.ID.IsUnknown() {
				vaultModel.ID = vault.ID.ValueInt64()
			}
			if vault.Password != nil {
				vaultModel.Type = "password"
				vaultModel.VaultKeyID = vault.Password.VaultKeyID.ValueInt64()
			}
			if vault.ClientScript != nil {
				vaultModel.Type = "script"
				vaultModel.Script = vault.ClientScript.Script.ValueString()
			}
			model.Vaults = append(model.Vaults, &vaultModel)
		}
	}

	// Legacy invocation-shaped parameters remain Terraform compatibility metadata.

	return &model
}

var _ sort.Interface = ByVaultID{}

type ByVaultID []*models.TemplateVault

func (a ByVaultID) Len() int           { return len(a) }
func (a ByVaultID) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByVaultID) Less(i, j int) bool { return a[i].ID < a[j].ID }

func convertTemplateResponseToProjectTemplateModel(ctx context.Context, request *models.Template, prev *ProjectTemplateModel) ProjectTemplateModel {
	envIDs := request.EnvironmentIds
	if envIDs == nil {
		envIDs = []int64{}
		if request.EnvironmentID > 0 {
			envIDs = append(envIDs, request.EnvironmentID)
		}
	}
	groups, _ := types.SetValueFrom(ctx, types.Int64Type, envIDs)
	legacy := types.Int64Null()
	if len(envIDs) > 0 {
		legacy = types.Int64Value(envIDs[0])
	}
	tags := request.RunnerTags
	if tags == nil {
		tags = []string{}
	}
	runnerTags, _ := types.SetValueFrom(ctx, types.StringType, tags)
	workingDirectory, executorImage := "", ""
	if request.WorkingDirectory != nil {
		workingDirectory = *request.WorkingDirectory
	}
	if request.ExecutorImage != nil {
		executorImage = *request.ExecutorImage
	}
	matchMode := request.RunnerTagMatchMode
	if matchMode == "" {
		matchMode = "all"
	}
	model := ProjectTemplateModel{
		ID:                        types.Int64Value(request.ID),
		ProjectID:                 types.Int64Value(request.ProjectID),
		AllowParallelTasks:        types.BoolValue(request.AllowParallelTasks),
		AllowOverrideBranchInTask: types.BoolValue(request.AllowOverrideBranchInTask),
		JWTParams:                 templateJWTFromAPI(ctx, request.JwtParams),
		EnvironmentID:             legacy,
		EnvironmentIDs:            groups,
		WorkingDirectory:          types.StringValue(workingDirectory),
		ExecutorImage:             types.StringValue(executorImage),
		SuppressErrorAlerts:       types.BoolValue(request.SuppressErrorAlerts),
		RunnerTags:                runnerTags,
		RunnerTagMatchMode:        types.StringValue(matchMode),
		InventoryID:               types.Int64Value(request.InventoryID),
		RepositoryID:              types.Int64Value(request.RepositoryID),
		App:                       types.StringValue(request.App),
		Name:                      types.StringValue(request.Name),
		Playbook:                  types.StringValue(request.Playbook),
		AllowOverrideArgsInTask:   types.BoolValue(request.AllowOverrideArgsInTask),
		SuppressSuccessAlerts:     types.BoolValue(request.SuppressSuccessAlerts),
	}

	if request.Description != "" {
		model.Description = types.StringValue(request.Description)
	} else if !prev.Description.IsNull() && prev.Description.ValueString() == "" {
		model.Description = prev.Description
	}

	if request.GitBranch != "" {
		model.GitBranch = types.StringValue(request.GitBranch)
	} else if !prev.GitBranch.IsNull() && prev.GitBranch.ValueString() == "" {
		model.GitBranch = prev.GitBranch
	}

	if request.ViewID != 0 {
		model.ViewID = types.Int64Value(request.ViewID)
	} else if !prev.ViewID.IsNull() && prev.ViewID.ValueInt64() == 0 {
		model.ViewID = prev.ViewID
	}

	var arguments []string
	if json.Unmarshal([]byte(request.Arguments), &arguments) != nil {
		model.Arguments = types.ListNull(types.StringType)
	} else {
		if len(arguments) == 0 {
			model.Arguments = types.ListNull(types.StringType)
		} else {
			args, _ := types.ListValueFrom(ctx, types.StringType, arguments)
			model.Arguments = args
		}
	}

	if request.Type == "build" {
		build := ProjectTemplateTypeBuildModel{}
		if request.StartVersion != "" {
			build.StartVersion = types.StringValue(request.StartVersion)
		} else {
			if prev.Build != nil {
				build.StartVersion = prev.Build.StartVersion
			} else {
				build.StartVersion = types.StringNull()
			}
		}
		model.Build = &build
	}

	if request.Type == "deploy" {
		model.Deploy = &ProjectTemplateTypeDeployModel{
			BuildTemplateID: types.Int64Value(request.BuildTemplateID),
			Autorun:         types.BoolValue(request.Autorun),
		}
	}

	if len(request.SurveyVars) == 0 {
		model.SurveyVars = emptyListAfterRead(prev.SurveyVars, ProjectTemplateSurveyVarType)
	} else {
		var surveyVars []ProjectTemplateSurveyVarModel
		for _, surveyVar := range request.SurveyVars {
			surveyVarModel := ProjectTemplateSurveyVarModel{
				Name:          types.StringValue(surveyVar.Name),
				Title:         types.StringValue(surveyVar.Title),
				Required:      types.BoolValue(surveyVar.Required),
				Type:          types.StringValue(surveyTypeFromAPI(surveyVar.Type)),
				Target:        stringOrNull(surveyVar.Target),
				DefaultValue:  types.StringNull(),
				DefaultValues: types.ListNull(types.StringType),
				EnumValues:    types.MapNull(types.StringType),
				Choices:       types.ListNull(surveyChoiceType()),
			}
			if surveyVar.Description != "" {
				surveyVarModel.Description = types.StringValue(surveyVar.Description)
			}
			readSurveyDefault(ctx, surveyVar.DefaultValue, &surveyVarModel)
			if surveyVar.Type == "enum" || surveyVar.Type == "select" {
				surveyVarModel.Choices = surveyChoicesFromAPI(surveyVar.Values)
				surveyVarModel.EnumValues = surveyMapFromChoices(surveyVarModel.Choices)
			}
			surveyVars = append(surveyVars, surveyVarModel)
		}
		surveyVarsModel, _ := types.ListValueFrom(ctx, ProjectTemplateSurveyVarType, &surveyVars)
		model.SurveyVars = surveyVarsModel
	}

	if len(request.Vaults) == 0 {
		model.Vaults = emptyListAfterRead(prev.Vaults, ProjectTemplateVaultType)
	} else {
		sort.Sort(ByVaultID(request.Vaults))

		var vaults []ProjectTemplateVaultModel
		for _, vault := range request.Vaults {
			vaultModel := ProjectTemplateVaultModel{
				ID:   types.Int64Value(vault.ID),
				Name: types.StringValue(vault.Name),
			}
			if vault.Type == "password" {
				vaultModel.Password = &ProjectTemplateVaultPasswordModel{
					VaultKeyID: types.Int64Value(vault.VaultKeyID),
				}
			}
			if vault.Type == "script" {
				vaultModel.ClientScript = &ProjectTemplateVaultScriptModel{
					Script: types.StringValue(vault.Script),
				}
			}
			vaults = append(vaults, vaultModel)
		}
		vaultsModel, _ := types.ListValueFrom(ctx, ProjectTemplateVaultType, &vaults)
		model.Vaults = vaultsModel
	}

	var priorTaskParams *TaskParamsModel
	if prev != nil {
		priorTaskParams = prev.TaskParams
	}
	model.TaskParams = priorTaskParams
	if prev != nil {
		model.AnsibleSettings = prev.AnsibleSettings
		model.TerraformSettings = prev.TerraformSettings
	}
	if len(model.AnsibleSettings.AttributeTypes(ctx)) == 0 {
		model.AnsibleSettings = types.ObjectNull(templateSettingTypes("ansible"))
	}
	if len(model.TerraformSettings.AttributeTypes(ctx)) == 0 {
		model.TerraformSettings = types.ObjectNull(templateSettingTypes("terraform"))
	}

	return model
}

func templateRequestWithSSHKeys(ctx context.Context, plan ProjectTemplateModel) (map[string]any, error) {
	encoded, err := json.Marshal(convertProjectTemplateModelToTemplateRequest(ctx, plan))
	if err != nil {
		return nil, err
	}
	var body map[string]any
	if err = json.Unmarshal(encoded, &body); err != nil {
		return nil, err
	}
	settings, err := templateSettingsPayload(ctx, plan.AnsibleSettings, plan.TerraformSettings)
	if err != nil {
		return nil, err
	}
	if err := addLegacyTemplateMetadata(ctx, settings, plan.TaskParams); err != nil {
		return nil, err
	}
	body["task_params"] = settings
	if plan.SSHKeys.IsNull() || plan.SSHKeys.IsUnknown() {
		return body, nil
	}
	value, err := sshKeyPolicySelectionToAPI(ctx, plan.SSHKeys)
	if err != nil {
		return nil, err
	}
	body["ssh_keys"] = value
	return body, nil
}

func readTemplateSSHKeys(ctx context.Context, client *apiclient.SemaphoreUI, model *ProjectTemplateModel) error {
	var raw map[string]any
	if err := exRequest(ctx, client, http.MethodGet, "/project/{project_id}/templates/{template_id}", map[string]string{"project_id": strconv.FormatInt(model.ProjectID.ValueInt64(), 10), "template_id": strconv.FormatInt(model.ID.ValueInt64(), 10)}, nil, &raw); err != nil {
		return err
	}
	if err := readTemplateApplicationSettings(ctx, raw, model); err != nil {
		return err
	}
	selection, err := sshKeyPolicySelectionFromAPI(raw["ssh_keys"])
	if err != nil {
		return err
	}
	model.SSHKeys = selection
	return nil
}

func (r *projectTemplateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan ProjectTemplateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, err := templateRequestWithSSHKeys(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Template Settings", err.Error())
		return
	}
	var create struct {
		ID json.Number `json:"id"`
	}
	err = exRequest(ctx, r.client, http.MethodPost, "/project/{project_id}/templates", map[string]string{"project_id": strconv.FormatInt(plan.ProjectID.ValueInt64(), 10)}, body, &create)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating SemaphoreUI Project Template",
			"Could not create project template, unexpected error: "+err.Error(),
		)
		return
	}
	createdID, parseErr := create.ID.Int64()
	if parseErr != nil || createdID < 1 {
		resp.Diagnostics.AddError("Missing Created Template Identity", "The template create request succeeded but did not return a valid ID; inspect the server before retrying.")
		return
	}

	// Create response doesn't fully capture the model, so we need to read it back
	response, err := r.client.Template.GetProjectProjectIDTemplatesTemplateIDContext(ctx, &template.GetProjectProjectIDTemplatesTemplateIDParams{
		ProjectID:  plan.ProjectID.ValueInt64(),
		TemplateID: createdID,
	}, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading SemaphoreUI Project Template",
			"Could not read project template, unexpected error: "+err.Error(),
		)
		return
	}
	model := convertTemplateResponseToProjectTemplateModel(ctx, response.Payload, &plan)
	if err = readTemplateSSHKeys(ctx, r.client, &model); err != nil {
		resp.Diagnostics.AddError("Error Reading Template SSH Keys", err.Error())
		return
	}

	// Set state to fully populated data
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *projectTemplateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state ProjectTemplateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	response, err := r.client.Template.GetProjectProjectIDTemplatesTemplateIDContext(ctx, &template.GetProjectProjectIDTemplatesTemplateIDParams{
		ProjectID:  state.ProjectID.ValueInt64(),
		TemplateID: state.ID.ValueInt64(),
	}, nil)
	if resourceNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading SemaphoreUI Project Template",
			"Could not read project template, unexpected error: "+err.Error(),
		)
		return
	}
	model := convertTemplateResponseToProjectTemplateModel(ctx, response.Payload, &state)
	if err = readTemplateSSHKeys(ctx, r.client, &model); err != nil {
		resp.Diagnostics.AddError("Error Reading Template SSH Keys", err.Error())
		return
	}

	// Set refreshed state
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *projectTemplateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan ProjectTemplateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state ProjectTemplateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Computed group IDs must remain unknown when the legacy input changes.
	// Preserve refreshed memberships only for an unchanged legacy input.
	if plan.EnvironmentIDs.IsUnknown() && plan.EnvironmentID.Equal(state.EnvironmentID) {
		plan.EnvironmentIDs = state.EnvironmentIDs
	}

	body, err := templateRequestWithSSHKeys(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Template Settings", err.Error())
		return
	}
	if err = mergeTemplateUpdateSettings(ctx, r.client, plan, req.Config, body); err != nil {
		resp.Diagnostics.AddError("Error Preserving Template Settings", err.Error())
		return
	}
	err = exRequest(ctx, r.client, http.MethodPut, "/project/{project_id}/templates/{template_id}", map[string]string{"project_id": strconv.FormatInt(plan.ProjectID.ValueInt64(), 10), "template_id": strconv.FormatInt(plan.ID.ValueInt64(), 10)}, body, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating SemaphoreUI Project Template",
			"Could not update project template, unexpected error: "+err.Error(),
		)
		return
	}

	response, err := r.client.Template.GetProjectProjectIDTemplatesTemplateIDContext(ctx, &template.GetProjectProjectIDTemplatesTemplateIDParams{
		ProjectID:  plan.ProjectID.ValueInt64(),
		TemplateID: plan.ID.ValueInt64(),
	}, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading SemaphoreUI Project Template",
			"Could not read project template, unexpected error: "+err.Error(),
		)
		return
	}
	model := convertTemplateResponseToProjectTemplateModel(ctx, response.Payload, &plan)
	if err = readTemplateSSHKeys(ctx, r.client, &model); err != nil {
		resp.Diagnostics.AddError("Error Reading Template SSH Keys", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *projectTemplateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state ProjectTemplateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Template.DeleteProjectProjectIDTemplatesTemplateIDContext(ctx, &template.DeleteProjectProjectIDTemplatesTemplateIDParams{
		ProjectID:  state.ProjectID.ValueInt64(),
		TemplateID: state.ID.ValueInt64(),
	}, nil)
	if err != nil && !resourceNotFound(err) {
		resp.Diagnostics.AddError(
			"Error Removing SemaphoreUI Project Template",
			"Could not delete project template, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *projectTemplateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	fields, err := parseImportFields(req.ID, []string{"project", "template"})
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Project Template Import ID",
			"Could not parse import ID: "+err.Error(),
		)
		return
	}

	response, err := r.client.Template.GetProjectProjectIDTemplatesTemplateIDContext(ctx, &template.GetProjectProjectIDTemplatesTemplateIDParams{
		ProjectID:  fields["project"],
		TemplateID: fields["template"],
	}, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading SemaphoreUI Project Template",
			"Could not read project template, unexpected error: "+err.Error(),
		)
		return
	}
	model := convertTemplateResponseToProjectTemplateModel(ctx, response.Payload, &ProjectTemplateModel{
		SurveyVars: types.ListNull(ProjectTemplateSurveyVarType),
		Vaults:     types.ListNull(ProjectTemplateVaultType),
	})
	if err = readTemplateSSHKeys(ctx, r.client, &model); err != nil {
		resp.Diagnostics.AddError("Error Reading Template SSH Keys", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
	if resp.Diagnostics.HasError() {
		return
	}
}
