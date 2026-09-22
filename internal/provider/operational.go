package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework/action"
	as "github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	es "github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type operationalAction struct {
	spec   operationalSpec
	client *apiclient.SemaphoreUI
}
type operationalPreview struct {
	spec   operationalSpec
	client *apiclient.SemaphoreUI
}

func (a *operationalAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + a.spec.name
}
func (a *operationalAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	description := a.spec.description + " Executes only through explicit Action invocation. Progress reports a bounded, non-secret summary."
	if a.spec.preview {
		description += " Full preview results are available through the matching ephemeral resource."
	}
	if a.spec.name == "ldap_group_apply" || a.spec.name == "oidc_group_preview" {
		description += actionWriteOnlyLimitation
	}
	resp.Schema = as.Schema{MarkdownDescription: description, Attributes: a.spec.inputs}
}
func (a *operationalAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	var ok bool
	a.client, ok = req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Invalid Provider Client", "Expected the configured Semaphore EX client.")
	}
}
func (a *operationalAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var config types.Object
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := a.spec.execute(ctx, a.client, config)
	if err != nil {
		resp.Diagnostics.AddError("Semaphore EX Operation Failed", err.Error())
		return
	}
	if resp.SendProgress != nil {
		resp.SendProgress(action.InvokeProgressEvent{Message: a.spec.name + " completed: " + operationSummary(result)})
	}
}
func (d *operationalPreview) Metadata(_ context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + d.spec.name
}
func (d *operationalPreview) Schema(_ context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	attributes := map[string]es.Attribute{}
	for name, field := range d.spec.inputs {
		converted, err := operationEphemeralAttribute(field)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Preview Schema", err.Error())
			return
		}
		attributes[name] = converted
	}
	attributes["result"] = es.DynamicAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Complete native result, excluding any preview token. It is ephemeral and never stored in Terraform state. Sensitive because results may describe identity or execution metadata."}
	if d.spec.token {
		attributes["preview_token"] = es.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Ephemeral preview token. LDAP apply accepts it for the same provider after rechecking freshness. OIDC has no public apply operation."}
	}
	resp.Schema = es.Schema{MarkdownDescription: d.spec.description + " Opening this ephemeral resource evaluates the preview; results are not persisted in Terraform state. LDAP/OIDC previews record server-side reconciliation history. No automatic apply, renewal or deletion of audit history is performed.", Attributes: attributes}
}
func (d *operationalPreview) Configure(_ context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	var ok bool
	d.client, ok = req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Invalid Provider Client", "Expected the configured Semaphore EX client.")
	}
}
func (d *operationalPreview) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var config types.Object
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := d.spec.execute(ctx, d.client, config)
	if err != nil {
		resp.Diagnostics.AddError("Semaphore EX Preview Failed", err.Error())
		return
	}
	values := config.Attributes()
	if d.spec.token {
		object, ok := result.(map[string]any)
		if !ok {
			resp.Diagnostics.AddError("Invalid Preview Result", "Expected a structured preview with a token.")
			return
		}
		token, ok := object["token"].(string)
		if !ok || token == "" {
			resp.Diagnostics.AddError("Invalid Preview Result", "The server returned no preview token.")
			return
		}
		values["preview_token"] = types.StringValue(token)
		delete(object, "token")
	}
	values["result"] = dynamicValueFromAPI(result)
	value, diags := types.ObjectValue(config.AttributeTypes(ctx), values)
	resp.Diagnostics.Append(diags...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.Result.Set(ctx, value)...)
	}
}
func operationEphemeralAttribute(field as.Attribute) (es.Attribute, error) {
	switch field := field.(type) {
	case as.StringAttribute:
		return es.StringAttribute{Required: field.Required, Optional: field.Optional, MarkdownDescription: field.MarkdownDescription, Validators: field.Validators, Sensitive: field.WriteOnly}, nil
	case as.Int64Attribute:
		return es.Int64Attribute{Required: field.Required, Optional: field.Optional, MarkdownDescription: field.MarkdownDescription, Validators: field.Validators}, nil
	case as.BoolAttribute:
		return es.BoolAttribute{Required: field.Required, Optional: field.Optional, MarkdownDescription: field.MarkdownDescription, Validators: field.Validators}, nil
	case as.DynamicAttribute:
		return es.DynamicAttribute{Required: field.Required, Optional: field.Optional, MarkdownDescription: field.MarkdownDescription, Validators: field.Validators, Sensitive: field.WriteOnly}, nil
	case as.ListAttribute:
		return es.ListAttribute{Required: field.Required, Optional: field.Optional, MarkdownDescription: field.MarkdownDescription, ElementType: field.ElementType, Validators: field.Validators}, nil
	case as.SingleNestedAttribute:
		nested := map[string]es.Attribute{}
		for name, child := range field.Attributes {
			converted, err := operationEphemeralAttribute(child)
			if err != nil {
				return nil, err
			}
			nested[name] = converted
		}
		return es.SingleNestedAttribute{Required: field.Required, Optional: field.Optional, MarkdownDescription: field.MarkdownDescription, Attributes: nested, Validators: field.Validators}, nil
	default:
		return nil, fmt.Errorf("unsupported preview attribute type %T", field)
	}
}
func (s operationalSpec) execute(ctx context.Context, client *apiclient.SemaphoreUI, config types.Object) (any, error) {
	values := config.Attributes()
	for name, attribute := range s.inputs {
		value := values[name]
		if value == nil || value.IsUnknown() || (attribute.IsRequired() && value.IsNull()) {
			return nil, fmt.Errorf("%s must be known and supplied when required", name)
		}
	}
	options := exRequestOptions{PathParams: map[string]string{}, Query: map[string]string{}}
	for parameter, name := range s.paths {
		id, err := exPathID(values[name])
		if err != nil {
			return nil, err
		}
		if strings.ContainsAny(id, "/\\?#%") || id == "." || id == ".." {
			return nil, fmt.Errorf("%s must be one path component", name)
		}
		options.PathParams[parameter] = id
	}
	for parameter, name := range s.query {
		if values[name].IsNull() {
			continue
		}
		value, err := exWireValue(ctx, values[name])
		if err != nil {
			return nil, err
		}
		options.Query[parameter] = fmt.Sprint(value)
	}
	body := map[string]any{}
	if s.bodyRoot != "" {
		value, err := exWireValue(ctx, values[s.bodyRoot])
		if err != nil {
			return nil, err
		}
		object, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s must be a native HCL object", s.bodyRoot)
		}
		body = object
		if _, typed := s.inputs[s.bodyRoot].(as.SingleNestedAttribute); typed {
			omitOperationNullFields(body)
		}
	}
	for _, name := range s.body {
		if values[name].IsNull() {
			continue
		}
		value, err := exWireValue(ctx, values[name])
		if err != nil {
			return nil, err
		}
		body[name] = value
	}
	if s.name == "workflow_trigger_test" && body["inputs"] == nil {
		body["inputs"] = map[string]any{}
	}
	if err := s.bindScope(body, values); err != nil {
		return nil, err
	}
	if value, present := body["inputs"]; present && s.scopeMode != "guardrail_inputs" {
		if _, ok := value.(map[string]any); !ok {
			return nil, fmt.Errorf("inputs must be a native HCL object")
		}
	}
	var payload any
	if len(body) > 0 {
		payload = body
	}
	var result any
	if err := exRequestWithOptions(ctx, client, s.method, s.route, options, payload, &result); err != nil {
		return nil, err
	}
	return result, nil
}
func (s operationalSpec) bindScope(body map[string]any, values map[string]attr.Value) error {
	projectID, project := values["project_id"].(types.Int64)
	projectNumber := projectID.ValueInt64()
	switch s.scopeMode {
	case "notification":
		scope := "global"
		if project {
			scope = "project"
		}
		if prior, ok := body["scope"]; ok && prior != nil && prior != scope {
			return fmt.Errorf("event scope must match the selected operation")
		}
		body["scope"] = scope
		if prior := body["project_id"]; prior != nil {
			id, err := identityNumber(prior)
			if err != nil || !project || id != projectNumber {
				return fmt.Errorf("event project must match project_id")
			}
		}
		if project {
			body["project_id"] = projectNumber
		} else {
			delete(body, "project_id")
		}
	case "kubernetes":
		alias, _ := values["cluster_alias"].(types.String)
		body["cluster_alias"] = alias.ValueString()
	case "guardrail_input":
		return guardrailOperationInputScope(body["input"], project, projectNumber)
	case "guardrail_inputs":
		inputs, ok := body["inputs"].([]any)
		if !ok || len(inputs) < 1 || len(inputs) > 100 {
			return fmt.Errorf("inputs must contain 1–100 native objects")
		}
		for _, input := range inputs {
			if err := guardrailOperationInputScope(input, project, projectNumber); err != nil {
				return err
			}
		}
	case "deployment":
		if body["project_id"] != nil {
			return fmt.Errorf("deployment policy scope is selected only by project_id on the resource/action")
		}
		hasTemplate, hasWorkflow := body["template_id"] != nil, body["workflow_id"] != nil
		if hasTemplate == hasWorkflow {
			return fmt.Errorf("deployment preview requires exactly one template_id or workflow_id")
		}
	}
	return nil
}
func guardrailOperationInputScope(raw any, project bool, projectID int64) error {
	input, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("guardrail input must be a native object")
	}
	if input["project_id"] == nil && project {
		input["project_id"] = projectID
	}
	id, err := identityNumber(input["project_id"])
	if err != nil || id < 1 || (project && id != projectID) {
		return fmt.Errorf("guardrail input project_id must be positive and match the operation scope")
	}
	return nil
}
func operationSummary(result any) string {
	summary := map[string]any{}
	switch value := result.(type) {
	case []any:
		summary["items"] = len(value)
	case map[string]any:
		for _, name := range []string{"id", "status", "allowed", "denied", "valid", "rule", "code", "reason", "reason_code", "fingerprint", "revision", "mapping_revision", "directory_revision", "claim_revision", "invocation_id", "workflow_run_id", "run_id", "result", "next_eligible_at", "rule_count", "preserved"} {
			switch field := value[name].(type) {
			case string, bool, json.Number:
				summary[name] = field
			}
		}
		for _, name := range []string{"additions", "removals", "unresolved", "collisions", "protected_admin_violations", "unknown_values", "issues", "findings", "evaluations", "added", "removed", "changed"} {
			if items, ok := value[name].([]any); ok {
				summary[name+"_count"] = len(items)
			}
		}
	}
	encoded, err := json.Marshal(summary)
	if err != nil || len(encoded) > 4096 {
		return "result available; use the ephemeral preview for details"
	}
	return string(encoded)
}

// Typed request fields are optional; omission lets the API use its native defaults.
// Dynamic inputs retain authored nulls because those can be meaningful payload data.
func omitOperationNullFields(object map[string]any) {
	for name, value := range object {
		if value == nil {
			delete(object, name)
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			omitOperationNullFields(nested)
		}
	}
}
