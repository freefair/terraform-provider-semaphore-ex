package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func taskStartParamsAttribute() schema.SingleNestedAttribute {
	fields := map[string]schema.Attribute{}
	for name, description := range map[string]string{
		"debug": "Enable Ansible verbosity; requires template allow_debug.", "dry_run": "Request Ansible check mode; rejected when the template hides it.", "diff": "Request Ansible diffs; rejected when the template hides them.", "skip_galaxy_install": "Override Galaxy installation; requires the template override flag.",
		"plan": "Run Terraform plan only.", "destroy": "Request Terraform destroy mode. The backend does not independently enforce the template's UI allow_destroy flag.", "auto_approve": "Request Terraform auto-approval when the template permits it; a forced template default cannot be disabled per run.", "upgrade": "Upgrade Terraform initialization dependencies.", "reconfigure": "Reconfigure the Terraform backend during initialization.",
	} {
		fields[name] = schema.BoolAttribute{Optional: true, MarkdownDescription: description}
	}
	for _, name := range []string{"limit", "tags", "skip_tags"} {
		fields[name] = schema.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Per-run Ansible override. Requires the matching template allow_override flag; [] explicitly clears the default."}
	}
	fields["debug_level"] = schema.Int64Attribute{Optional: true, Validators: []validator.Int64{int64validator.Between(0, 6)}, MarkdownDescription: "Ansible verbosity 0–6. A positive value requires debug = true."}
	return schema.SingleNestedAttribute{Optional: true, MarkdownDescription: "Typed per-run parameters, distinct from template settings. Only parameters for the selected application's family are accepted.", Attributes: fields}
}

func taskStartTemplateID(ctx context.Context, client *apiclient.SemaphoreUI, projectID int64, id types.Int64, name types.String) (int64, error) {
	if id.IsUnknown() || name.IsUnknown() || id.IsNull() == name.IsNull() {
		return 0, fmt.Errorf("configure exactly one known template_id or template_name")
	}
	if !id.IsNull() {
		if id.ValueInt64() < 1 {
			return 0, fmt.Errorf("template_id must be positive")
		}
		return id.ValueInt64(), nil
	}
	if name.ValueString() == "" {
		return 0, fmt.Errorf("template_name must not be empty")
	}
	records, err := readCollection(ctx, client, lookupSpecs()["project_template"], map[string]string{"project_id": strconv.FormatInt(projectID, 10)})
	if err != nil {
		return 0, err
	}
	var found int64
	for _, record := range records {
		if record["name"] != name.ValueString() {
			continue
		}
		if found != 0 {
			return 0, fmt.Errorf("template_name is ambiguous; use template_id")
		}
		found, err = identityNumber(record["id"])
		if err != nil || found < 1 {
			return 0, fmt.Errorf("template lookup returned an invalid identity")
		}
	}
	if found == 0 {
		return 0, fmt.Errorf("no accessible template has the requested exact name")
	}
	return found, nil
}

func taskStartParameters(ctx context.Context, client *apiclient.SemaphoreUI, projectID, templateID int64, params types.Object, overrides map[string]bool) (map[string]any, error) {
	if params.IsUnknown() {
		return nil, fmt.Errorf("task parameters must be known")
	}
	wire := map[string]any{}
	for name, value := range params.Attributes() {
		if value.IsNull() {
			continue
		}
		converted, err := exWireValue(ctx, value)
		if err != nil {
			return nil, fmt.Errorf("parameter %s must be known and valid", name)
		}
		wire[name] = converted
	}
	if len(wire) == 0 && len(overrides) == 0 {
		if params.IsNull() {
			return nil, nil
		}
		return wire, nil
	}
	var template map[string]any
	if err := exRequest(ctx, client, http.MethodGet, "/project/{project_id}/templates/{template_id}", map[string]string{"project_id": strconv.FormatInt(projectID, 10), "template_id": strconv.FormatInt(templateID, 10)}, nil, &template); err != nil {
		return nil, err
	}
	app, _ := template["app"].(string)
	if app == "" {
		app = "ansible"
	}
	settings, _ := template["task_params"].(map[string]any)
	for name := range overrides {
		switch name {
		case "git_branch", "commit_hash":
			if template["allow_override_branch_in_task"] != true {
				return nil, fmt.Errorf("%s requires template allow_override_branch_in_task = true; the server would ignore it", name)
			}
		case "arguments":
			if template["allow_override_args_in_task"] != true {
				return nil, fmt.Errorf("arguments requires template allow_override_args_in_task = true")
			}
		case "inventory_id":
			if app != "ansible" || settings["allow_override_inventory"] != true {
				return nil, fmt.Errorf("inventory_id requires an Ansible template with allow_override_inventory = true")
			}
		}
	}

	terraformFields := map[string]bool{"plan": true, "destroy": true, "auto_approve": true, "upgrade": true, "reconfigure": true}
	terraformApp := app == "terraform" || app == "tofu" || app == "terragrunt"
	for name := range wire {
		if (app == "ansible" && terraformFields[name]) || (terraformApp && !terraformFields[name]) || (!terraformApp && app != "ansible") {
			return nil, fmt.Errorf("parameter %s is not supported for application %s", name, app)
		}
	}
	if app == "ansible" {
		for name, permission := range map[string]string{"limit": "allow_override_limit", "tags": "allow_override_tags", "skip_tags": "allow_override_skip_tags", "skip_galaxy_install": "allow_override_skip_galaxy_install"} {
			if _, present := wire[name]; present && settings[permission] != true {
				return nil, fmt.Errorf("parameter %s requires template %s = true; the server would ignore the override", name, permission)
			}
		}
		if wire["debug"] == true && settings["allow_debug"] != true {
			return nil, fmt.Errorf("debug requires template allow_debug = true")
		}
		if level, ok := wire["debug_level"].(int64); ok && level > 0 && wire["debug"] != true {
			return nil, fmt.Errorf("positive debug_level requires debug = true")
		}
		for name, hidden := range map[string]string{"diff": "hide_diff", "dry_run": "hide_dry_run"} {
			if wire[name] == true && settings[hidden] == true {
				return nil, fmt.Errorf("parameter %s is suppressed by template %s", name, hidden)
			}
		}
	} else {
		if wire["auto_approve"] == true && settings["auto_approve"] != true && settings["allow_auto_approve"] != true {
			return nil, fmt.Errorf("auto_approve requires template allow_auto_approve or auto_approve")
		}
		if wire["auto_approve"] == false && settings["auto_approve"] == true {
			return nil, fmt.Errorf("the template forces auto_approve; a per-run false value cannot disable it")
		}
	}
	if params.IsNull() {
		return nil, nil
	}
	return wire, nil
}

func taskSecretObject(value types.Dynamic) bool {
	if value.IsNull() {
		return true
	}
	if value.IsUnknown() {
		return false
	}
	switch object := value.UnderlyingValue().(type) {
	case types.Object:
		return !object.IsUnknown()
	case types.Map:
		return !object.IsUnknown()
	default:
		return false
	}
}
