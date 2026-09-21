package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rs "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	superschema "github.com/orange-cloudavenue/terraform-plugin-framework-superschema"
)

func templateSettingTypes(kind string) map[string]attr.Type {
	result := map[string]attr.Type{}
	if kind == "ansible" {
		for _, name := range []string{"allow_debug", "allow_override_inventory", "allow_override_limit", "allow_override_tags", "allow_override_skip_tags", "skip_galaxy_install", "allow_override_skip_galaxy_install", "hide_dry_run", "hide_diff"} {
			result[name] = types.BoolType
		}
		for _, name := range []string{"limit", "tags", "skip_tags", "galaxy_role_args", "galaxy_collection_args"} {
			result[name] = types.ListType{ElemType: types.StringType}
		}
	} else {
		for _, name := range []string{"allow_destroy", "allow_auto_approve", "auto_approve", "override_backend"} {
			result[name] = types.BoolType
		}
		result["backend_filename"] = types.StringType
	}
	return result
}
func templateSettingsAttribute(kind string) superschema.SingleNestedAttribute {
	attributes := map[string]superschema.Attribute{}
	for name, typ := range templateSettingTypes(kind) {
		switch typ.(type) {
		case basetypes.BoolType:
			attributes[name] = superschema.BoolAttribute{Common: &rs.BoolAttribute{MarkdownDescription: templateSettingDescription(name)}, Resource: &rs.BoolAttribute{Optional: true, Computed: true}, DataSource: &ds.BoolAttribute{Computed: true}}
		case basetypes.StringType:
			attributes[name] = superschema.StringAttribute{Common: &rs.StringAttribute{MarkdownDescription: templateSettingDescription(name)}, Resource: &rs.StringAttribute{Optional: true, Computed: true}, DataSource: &ds.StringAttribute{Computed: true}}
		case types.ListType:
			attributes[name] = superschema.ListAttribute{Common: &rs.ListAttribute{ElementType: types.StringType, MarkdownDescription: templateSettingDescription(name)}, Resource: &rs.ListAttribute{Optional: true, Computed: true}, DataSource: &ds.ListAttribute{Computed: true}}
		}
	}
	return superschema.SingleNestedAttribute{Common: &rs.SingleNestedAttribute{MarkdownDescription: "Effective " + kind + " template settings. Omitted keys preserve existing server values. Configure false, an empty string or [] to clear a setting. These are distinct from per-run task parameters."}, Resource: &rs.SingleNestedAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Object{preserveTemplateSettings{}}}, DataSource: &ds.SingleNestedAttribute{Computed: true}, Attributes: attributes}
}
func templateSettingDescription(name string) string {
	descriptions := map[string]string{
		"allow_debug":                        "Allow verbosity selection when launching Ansible tasks.",
		"allow_override_inventory":           "Allow the task inventory to override the template inventory.",
		"allow_override_limit":               "Allow the task host limit to override the template limit.",
		"allow_override_tags":                "Allow task tags to override the template tags.",
		"allow_override_skip_tags":           "Allow task skip-tags to override the template skip-tags.",
		"skip_galaxy_install":                "Skip Ansible Galaxy requirements installation by default.",
		"allow_override_skip_galaxy_install": "Allow tasks to override Galaxy installation behavior.",
		"hide_dry_run":                       "Hide the Dry Run option; the server suppresses the corresponding task flag.",
		"hide_diff":                          "Hide the Diff option; the server suppresses the corresponding task flag.",
		"limit":                              "Default Ansible host patterns.", "tags": "Default Ansible tags.", "skip_tags": "Default Ansible tags to skip.",
		"galaxy_role_args":       "Arguments for ansible-galaxy role install, validated by the server.",
		"galaxy_collection_args": "Arguments for ansible-galaxy collection install, validated by the server.",
		"allow_destroy":          "Expose the Destroy option in the task UI. This flag is not an independent server authorization boundary.",
		"allow_auto_approve":     "Expose automatic approval as a selectable task option.",
		"auto_approve":           "Automatically approve template runs. Setting true explicitly changes execution behavior; legacy task_params values are never promoted here.",
		"override_backend":       "Generate the backend override when using the internal Terraform backend.",
		"backend_filename":       "Backend override filename; an empty value uses the server default.",
	}
	return descriptions[name]
}
func legacyTemplateParamsAttribute() superschema.SingleNestedAttribute {
	attribute := TaskParamsAttribute()
	attribute.Common.MarkdownDescription = "Deprecated compatibility metadata for the former template task_params shape. It did not configure template execution. Use ansible_settings or terraform_settings explicitly; existing values are never activated automatically."
	attribute.Resource.DeprecationMessage = "Template task_params contains legacy, ineffective invocation defaults. Move intended settings explicitly to ansible_settings or terraform_settings; the provider never activates legacy defaults automatically."
	return attribute
}
func templateSettingsObject(ctx context.Context, kind string, raw map[string]any, previous types.Object) (types.Object, error) {
	fieldTypes := templateSettingTypes(kind)
	present := false
	for name := range fieldTypes {
		present = present || raw[name] != nil
	}
	if !present && (previous.IsNull() || previous.IsUnknown()) {
		return types.ObjectNull(fieldTypes), nil
	}
	values := map[string]attr.Value{}
	for name, typ := range fieldTypes {
		value := raw[name]
		if value == nil {
			switch typ.(type) {
			case basetypes.BoolType:
				values[name] = types.BoolValue(false)
			case basetypes.StringType:
				values[name] = types.StringValue("")
			case types.ListType:
				previousList := types.ListNull(types.StringType)
				if old, ok := previous.Attributes()[name].(types.List); ok {
					previousList = old
				}
				values[name] = emptyListAfterRead(previousList, types.StringType)
			}
			continue
		}
		converted, err := exTypedValue(ctx, typ, value)
		if err != nil {
			return types.ObjectNull(fieldTypes), fmt.Errorf("invalid template setting %s: %w", name, err)
		}
		values[name] = converted
	}
	object, diagnostics := types.ObjectValue(fieldTypes, values)
	if diagnostics.HasError() {
		return object, fmt.Errorf("template settings do not match the schema")
	}
	return object, nil
}
func templateSettingsPayload(ctx context.Context, objects ...types.Object) (map[string]any, error) {
	result := map[string]any{}
	for _, object := range objects {
		if object.IsNull() || object.IsUnknown() {
			continue
		}
		for name, value := range object.Attributes() {
			if value.IsNull() || value.IsUnknown() {
				continue
			}
			wire, err := exWireValue(ctx, value)
			if err != nil {
				return nil, err
			}
			result[name] = wire
		}
	}
	return result, nil
}
func readTemplateApplicationSettings(ctx context.Context, raw map[string]any, model *ProjectTemplateModel) error {
	parameters := map[string]any{}
	if raw["task_params"] != nil {
		var ok bool
		parameters, ok = raw["task_params"].(map[string]any)
		if !ok {
			return fmt.Errorf("template task_params must be an object")
		}
	}
	var err error
	model.AnsibleSettings, err = templateSettingsObject(ctx, "ansible", parameters, model.AnsibleSettings)
	if err != nil {
		return err
	}
	model.TerraformSettings, err = templateSettingsObject(ctx, "terraform", parameters, model.TerraformSettings)
	if err != nil {
		return err
	}
	// Compatibility values remain metadata and never enter the effective settings.
	if parameters["params"] != nil {
		encoded, err := json.Marshal(parameters)
		if err != nil {
			return fmt.Errorf("could not decode legacy template metadata")
		}
		var legacy models.TaskPrams
		if err = json.Unmarshal(encoded, &legacy); err != nil {
			return fmt.Errorf("invalid legacy template metadata")
		}
		model.TaskParams = convertTaskPramsToTaskParamsModel(ctx, &legacy, model.TaskParams)
	} else {
		model.TaskParams = nil
	}
	return nil
}
func mergeTemplateUpdateSettings(ctx context.Context, client *apiclient.SemaphoreUI, plan ProjectTemplateModel, config tfsdk.Config, body map[string]any) error {
	var current map[string]any
	if err := exRequest(ctx, client, http.MethodGet, "/project/{project_id}/templates/{template_id}", map[string]string{"project_id": strconv.FormatInt(plan.ProjectID.ValueInt64(), 10), "template_id": strconv.FormatInt(plan.ID.ValueInt64(), 10)}, nil, &current); err != nil {
		return err
	}
	existing := map[string]any{}
	if current["task_params"] != nil {
		var ok bool
		existing, ok = current["task_params"].(map[string]any)
		if !ok {
			return fmt.Errorf("existing template task_params must be an object")
		}
	}
	merged := map[string]any{}
	for key, value := range existing {
		merged[key] = value
	}
	// Keep the published legacy wire shape recoverable on import, but never
	// flatten its invocation defaults into effective template settings.
	for _, key := range []string{"params", "arguments", "environment", "git_branch", "message", "version", "inventory_id"} {
		delete(merged, key)
	}
	if err := addLegacyTemplateMetadata(ctx, merged, plan.TaskParams); err != nil {
		return err
	}
	if config.Schema != nil {
		for _, name := range []string{"ansible_settings", "terraform_settings"} {
			var configured types.Object
			if diagnostics := config.GetAttribute(ctx, path.Root(name), &configured); diagnostics.HasError() {
				return fmt.Errorf("could not read configured template settings")
			}
			if configured.IsUnknown() {
				return fmt.Errorf("template settings must be known before applying")
			}
			if configured.IsNull() {
				continue
			}
			for key, value := range configured.Attributes() {
				if value.IsNull() {
					continue
				}
				wire, err := exWireValue(ctx, value)
				if err != nil {
					return err
				}
				merged[key] = wire
			}
		}
	}
	body["task_params"] = merged
	return nil
}

type templateSettingsValidator struct{}

func (templateSettingsValidator) Description(context.Context) string {
	return "Template settings must match the selected application."
}
func (v templateSettingsValidator) MarkdownDescription(c context.Context) string {
	return v.Description(c)
}
func (templateSettingsValidator) ValidateResource(ctx context.Context, q resource.ValidateConfigRequest, p *resource.ValidateConfigResponse) {
	var app types.String
	var ansible, terraform types.Object
	p.Diagnostics.Append(q.Config.GetAttribute(ctx, path.Root("app"), &app)...)
	p.Diagnostics.Append(q.Config.GetAttribute(ctx, path.Root("ansible_settings"), &ansible)...)
	p.Diagnostics.Append(q.Config.GetAttribute(ctx, path.Root("terraform_settings"), &terraform)...)
	if p.Diagnostics.HasError() || app.IsUnknown() {
		return
	}
	name := app.ValueString()
	if app.IsNull() {
		name = "ansible"
	}
	if !ansible.IsNull() && name != "ansible" {
		p.Diagnostics.AddAttributeError(path.Root("ansible_settings"), "Incompatible Template Settings", "ansible_settings requires app = ansible.")
	}
	if !terraform.IsNull() && name != "terraform" && name != "tofu" && name != "terragrunt" {
		p.Diagnostics.AddAttributeError(path.Root("terraform_settings"), "Incompatible Template Settings", "terraform_settings requires a Terraform-family application.")
	}
}

func addLegacyTemplateMetadata(ctx context.Context, target map[string]any, legacy *TaskParamsModel) error {
	if legacy == nil {
		return nil
	}
	encoded, err := json.Marshal(convertTaskParamsModelToTaskPrams(ctx, legacy))
	if err != nil {
		return err
	}
	var metadata map[string]any
	if err := json.Unmarshal(encoded, &metadata); err != nil {
		return err
	}
	for key, value := range metadata {
		target[key] = value
	}
	return nil
}

type preserveTemplateSettings struct{}

func (preserveTemplateSettings) Description(context.Context) string {
	return "Preserve omitted template settings, including null values."
}
func (m preserveTemplateSettings) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}
func (preserveTemplateSettings) PlanModifyObject(ctx context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if req.ConfigValue.IsNull() && req.PlanValue.IsUnknown() {
		resp.PlanValue = req.StateValue
		return
	}
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() || req.PlanValue.IsUnknown() || req.StateValue.IsNull() || req.StateValue.IsUnknown() {
		return
	}
	planned := req.PlanValue.Attributes()
	for name, value := range req.ConfigValue.Attributes() {
		if value.IsNull() {
			planned[name] = req.StateValue.Attributes()[name]
		}
	}
	value, diagnostics := types.ObjectValue(req.PlanValue.AttributeTypes(ctx), planned)
	resp.Diagnostics.Append(diagnostics...)
	resp.PlanValue = value
}
