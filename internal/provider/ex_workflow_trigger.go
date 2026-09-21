package provider

// Workflow triggers are an EX-only API contract. They intentionally use the
// authenticated EX transport rather than the generated upstream client.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
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

type exWorkflowTriggerModel struct {
	ID                        types.Int64  `tfsdk:"id"`
	ProjectID                 types.Int64  `tfsdk:"project_id"`
	WorkflowID                types.Int64  `tfsdk:"workflow_id"`
	Name                      types.String `tfsdk:"name"`
	Type                      types.String `tfsdk:"type"`
	Enabled                   types.Bool   `tfsdk:"enabled"`
	CronFormat                types.String `tfsdk:"cron_format"`
	InputMappings             types.List   `tfsdk:"input_mappings"`
	Revision                  types.Int64  `tfsdk:"revision"`
	CredentialRotationVersion types.Int64  `tfsdk:"credential_rotation_version"`
	SigningStageVersion       types.Int64  `tfsdk:"signing_stage_version"`
	SigningBootstrapVersion   types.Int64  `tfsdk:"signing_bootstrap_version"`
	Credential                types.String `tfsdk:"credential"`
	WebhookSigningSecret      types.String `tfsdk:"webhook_signing_secret"`
	NextWebhookSigningSecret  types.String `tfsdk:"next_webhook_signing_secret"`
	CredentialGeneration      types.Int64  `tfsdk:"credential_generation"`
	CurrentSigningKeyID       types.String `tfsdk:"current_signing_key_id"`
	NextSigningKeyID          types.String `tfsdk:"next_signing_key_id"`
	CurrentSigningGeneration  types.Int64  `tfsdk:"current_signing_generation"`
	NextSigningGeneration     types.Int64  `tfsdk:"next_signing_generation"`
	LastFired                 types.String `tfsdk:"last_fired"`
	LastResult                types.String `tfsdk:"last_result"`
}

type exWorkflowTriggerDataSourceModel struct {
	ID                       types.Int64  `tfsdk:"id"`
	ProjectID                types.Int64  `tfsdk:"project_id"`
	WorkflowID               types.Int64  `tfsdk:"workflow_id"`
	Name                     types.String `tfsdk:"name"`
	Type                     types.String `tfsdk:"type"`
	Enabled                  types.Bool   `tfsdk:"enabled"`
	CronFormat               types.String `tfsdk:"cron_format"`
	InputMappings            types.List   `tfsdk:"input_mappings"`
	Revision                 types.Int64  `tfsdk:"revision"`
	CredentialGeneration     types.Int64  `tfsdk:"credential_generation"`
	CurrentSigningKeyID      types.String `tfsdk:"current_signing_key_id"`
	NextSigningKeyID         types.String `tfsdk:"next_signing_key_id"`
	CurrentSigningGeneration types.Int64  `tfsdk:"current_signing_generation"`
	NextSigningGeneration    types.Int64  `tfsdk:"next_signing_generation"`
	LastFired                types.String `tfsdk:"last_fired"`
	LastResult               types.String `tfsdk:"last_result"`
}

type exWorkflowTriggerResource struct{ client *apiclient.SemaphoreUI }
type exWorkflowTriggerDataSource struct{ client *apiclient.SemaphoreUI }

var _ resource.ResourceWithImportState = &exWorkflowTriggerResource{}

func NewWorkflowTriggerResource() resource.Resource { return &exWorkflowTriggerResource{} }
func NewWorkflowTriggerDataSource() datasource.DataSource {
	return withNamedLookup(&exWorkflowTriggerDataSource{}, "workflow_trigger")
}

func exWorkflowTriggerFixedSecretAttributes(computed bool) map[string]rs.Attribute {
	return map[string]rs.Attribute{
		"access_key_id":        rs.Int64Attribute{Optional: !computed, Computed: computed, Validators: []validator.Int64{int64validator.AtLeast(1)}},
		"global_credential_id": rs.Int64Attribute{Optional: !computed, Computed: computed, Validators: []validator.Int64{int64validator.AtLeast(1)}},
	}
}

func exWorkflowTriggerMappingAttributes(computed bool) map[string]rs.Attribute {
	return map[string]rs.Attribute{
		"parameter":              rs.StringAttribute{Required: !computed, Computed: computed},
		"source":                 rs.StringAttribute{Required: !computed, Computed: computed, Validators: []validator.String{stringvalidator.OneOf("fixed", "request")}},
		"key":                    rs.StringAttribute{Optional: !computed, Computed: computed},
		"fixed_string":           rs.StringAttribute{Optional: !computed, Computed: computed},
		"fixed_integer":          rs.Int64Attribute{Optional: !computed, Computed: computed},
		"fixed_boolean":          rs.BoolAttribute{Optional: !computed, Computed: computed},
		"fixed_secret_reference": rs.SingleNestedAttribute{Optional: !computed, Computed: computed, Attributes: exWorkflowTriggerFixedSecretAttributes(computed)},
	}
}

func exWorkflowTriggerMappingTypes() map[string]attr.Type {
	return map[string]attr.Type{"parameter": types.StringType, "source": types.StringType, "key": types.StringType, "fixed_string": types.StringType, "fixed_integer": types.Int64Type, "fixed_boolean": types.BoolType, "fixed_secret_reference": types.ObjectType{AttrTypes: map[string]attr.Type{"access_key_id": types.Int64Type, "global_credential_id": types.Int64Type}}}
}

func exWorkflowTriggerMappingDataSourceAttributes() map[string]ds.Attribute {
	return map[string]ds.Attribute{
		"parameter": ds.StringAttribute{Computed: true}, "source": ds.StringAttribute{Computed: true}, "key": ds.StringAttribute{Computed: true},
		"fixed_string": ds.StringAttribute{Computed: true}, "fixed_integer": ds.Int64Attribute{Computed: true}, "fixed_boolean": ds.BoolAttribute{Computed: true},
		"fixed_secret_reference": ds.SingleNestedAttribute{Computed: true, Attributes: map[string]ds.Attribute{"access_key_id": ds.Int64Attribute{Computed: true}, "global_credential_id": ds.Int64Attribute{Computed: true}}},
	}
}

func exWorkflowTriggerResourceSchema() rs.Schema {
	return rs.Schema{MarkdownDescription: "Manages a revision-fenced Semaphore EX workflow trigger. Creating API or webhook triggers exposes the server-generated secret once as sensitive state; refresh never rotates, fires, or re-reads it.", Attributes: map[string]rs.Attribute{
		"id":                          rs.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
		"project_id":                  rs.Int64Attribute{Required: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}},
		"workflow_id":                 rs.Int64Attribute{Required: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}},
		"name":                        rs.StringAttribute{Required: true},
		"type":                        rs.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, Validators: []validator.String{stringvalidator.OneOf("manual", "schedule", "api", "webhook")}},
		"enabled":                     rs.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"cron_format":                 rs.StringAttribute{Optional: true, Computed: true},
		"input_mappings":              rs.ListNestedAttribute{Optional: true, Computed: true, NestedObject: rs.NestedAttributeObject{Attributes: exWorkflowTriggerMappingAttributes(false)}},
		"revision":                    rs.Int64Attribute{Computed: true},
		"credential_rotation_version": rs.Int64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}, MarkdownDescription: "Bump this value to explicitly rotate an API-trigger credential. Refresh and ordinary updates never rotate credentials."},
		"signing_stage_version":       rs.Int64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}, MarkdownDescription: "Bump this value to explicitly stage a webhook signing key."},
		"signing_bootstrap_version":   rs.Int64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}, MarkdownDescription: "Bump this value to bootstrap signing only for an imported legacy webhook without a signing key."},
		"credential":                  rs.StringAttribute{Computed: true, Sensitive: true},
		"webhook_signing_secret":      rs.StringAttribute{Computed: true, Sensitive: true},
		"next_webhook_signing_secret": rs.StringAttribute{Computed: true, Sensitive: true},
		"credential_generation":       rs.Int64Attribute{Computed: true}, "current_signing_key_id": rs.StringAttribute{Computed: true}, "next_signing_key_id": rs.StringAttribute{Computed: true},
		"current_signing_generation": rs.Int64Attribute{Computed: true}, "next_signing_generation": rs.Int64Attribute{Computed: true}, "last_fired": rs.StringAttribute{Computed: true}, "last_result": rs.StringAttribute{Computed: true},
	}}
}

func exWorkflowTriggerDataSourceSchema() ds.Schema {
	return ds.Schema{MarkdownDescription: "Reads a Semaphore EX workflow trigger without any credential or signing secret.", Attributes: map[string]ds.Attribute{
		"id": ds.Int64Attribute{Required: true}, "project_id": ds.Int64Attribute{Required: true}, "workflow_id": ds.Int64Attribute{Required: true},
		"name": ds.StringAttribute{Computed: true}, "type": ds.StringAttribute{Computed: true}, "enabled": ds.BoolAttribute{Computed: true}, "cron_format": ds.StringAttribute{Computed: true},
		"input_mappings": ds.ListNestedAttribute{Computed: true, NestedObject: ds.NestedAttributeObject{Attributes: exWorkflowTriggerMappingDataSourceAttributes()}}, "revision": ds.Int64Attribute{Computed: true},
		"credential_generation": ds.Int64Attribute{Computed: true}, "current_signing_key_id": ds.StringAttribute{Computed: true}, "next_signing_key_id": ds.StringAttribute{Computed: true},
		"current_signing_generation": ds.Int64Attribute{Computed: true}, "next_signing_generation": ds.Int64Attribute{Computed: true}, "last_fired": ds.StringAttribute{Computed: true}, "last_result": ds.StringAttribute{Computed: true},
	}}
}

func (r *exWorkflowTriggerResource) Metadata(_ context.Context, q resource.MetadataRequest, p *resource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_workflow_trigger"
}
func (d *exWorkflowTriggerDataSource) Metadata(_ context.Context, q datasource.MetadataRequest, p *datasource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_workflow_trigger"
}
func (r *exWorkflowTriggerResource) Schema(_ context.Context, _ resource.SchemaRequest, p *resource.SchemaResponse) {
	p.Schema = exWorkflowTriggerResourceSchema()
}
func (d *exWorkflowTriggerDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, p *datasource.SchemaResponse) {
	p.Schema = exWorkflowTriggerDataSourceSchema()
}
func (r *exWorkflowTriggerResource) Configure(_ context.Context, q resource.ConfigureRequest, p *resource.ConfigureResponse) {
	r.client = exWorkflowTriggerClient(q.ProviderData, &p.Diagnostics)
}
func (d *exWorkflowTriggerDataSource) Configure(_ context.Context, q datasource.ConfigureRequest, p *datasource.ConfigureResponse) {
	d.client = exWorkflowTriggerClient(q.ProviderData, &p.Diagnostics)
}
func exWorkflowTriggerClient(value any, diagnostics interface{ AddError(string, string) }) *apiclient.SemaphoreUI {
	if value == nil {
		return nil
	}
	c, ok := value.(*apiclient.SemaphoreUI)
	if !ok {
		diagnostics.AddError("Unexpected Workflow Trigger Configure Type", "Expected the configured Semaphore EX client.")
		return nil
	}
	return c
}

func exWorkflowTriggerParams(m exWorkflowTriggerModel) map[string]string {
	return map[string]string{"project_id": strconv.FormatInt(m.ProjectID.ValueInt64(), 10), "workflow_id": strconv.FormatInt(m.WorkflowID.ValueInt64(), 10), "trigger_id": strconv.FormatInt(m.ID.ValueInt64(), 10)}
}
func exWorkflowTriggerRoute(member bool) string {
	route := "/project/{project_id}/workflows/{workflow_id}/triggers"
	if member {
		route += "/{trigger_id}"
	}
	return route
}

func exWorkflowTriggerFixedValue(ctx context.Context, m types.Object) (any, error) {
	a := m.Attributes()
	values := 0
	var result any
	for name, value := range map[string]attr.Value{"fixed_string": a["fixed_string"], "fixed_integer": a["fixed_integer"], "fixed_boolean": a["fixed_boolean"]} {
		if value != nil && !value.IsNull() && !value.IsUnknown() {
			converted, err := exWireValue(ctx, value)
			if err != nil {
				return nil, err
			}
			values++
			result = converted
		}
		_ = name
	}
	secret, ok := a["fixed_secret_reference"].(types.Object)
	if !ok {
		return nil, fmt.Errorf("fixed_secret_reference is invalid")
	}
	if !secret.IsNull() && !secret.IsUnknown() {
		reference := secret.Attributes()
		access, accessOK := reference["access_key_id"].(types.Int64)
		global, globalOK := reference["global_credential_id"].(types.Int64)
		if !accessOK || !globalOK {
			return nil, fmt.Errorf("fixed_secret_reference is invalid")
		}
		if (access.IsNull() || access.IsUnknown() || access.ValueInt64() < 1) == (global.IsNull() || global.IsUnknown() || global.ValueInt64() < 1) {
			return nil, fmt.Errorf("fixed_secret_reference must set exactly one credential ID")
		}
		values++
		secretValue := map[string]any{}
		if !access.IsNull() && !access.IsUnknown() && access.ValueInt64() > 0 {
			secretValue["access_key_id"] = access.ValueInt64()
		} else {
			secretValue["global_credential_id"] = global.ValueInt64()
		}
		result = secretValue
	}
	if values != 1 {
		return nil, fmt.Errorf("a fixed input mapping must set exactly one typed fixed value")
	}
	return result, nil
}

func exWorkflowTriggerBody(ctx context.Context, m exWorkflowTriggerModel, revision int64) (map[string]any, error) {
	body := map[string]any{"name": m.Name.ValueString(), "type": m.Type.ValueString(), "enabled": m.Enabled.ValueBool()}
	if revision > 0 {
		body["revision"] = revision
	}
	if !m.CronFormat.IsNull() && !m.CronFormat.IsUnknown() {
		body["cron_format"] = m.CronFormat.ValueString()
	}
	if m.InputMappings.IsNull() || m.InputMappings.IsUnknown() {
		body["input_mappings"] = []any{}
		return body, nil
	}
	var mappings []types.Object
	if d := m.InputMappings.ElementsAs(ctx, &mappings, false); d.HasError() {
		return nil, fmt.Errorf("input_mappings are invalid")
	}
	out := make([]any, 0, len(mappings))
	for _, mapping := range mappings {
		a := mapping.Attributes()
		parameter, parameterOK := a["parameter"].(types.String)
		source, sourceOK := a["source"].(types.String)
		if !parameterOK || !sourceOK {
			return nil, fmt.Errorf("input mapping has invalid fields")
		}
		if parameter.IsNull() || parameter.IsUnknown() || parameter.ValueString() == "" || source.IsNull() || source.IsUnknown() {
			return nil, fmt.Errorf("input mapping parameter and source must be known")
		}
		item := map[string]any{"parameter": parameter.ValueString(), "source": source.ValueString()}
		switch source.ValueString() {
		case "request":
			key, keyOK := a["key"].(types.String)
			if !keyOK {
				return nil, fmt.Errorf("request input mapping key is invalid")
			}
			if key.IsNull() || key.IsUnknown() || key.ValueString() == "" {
				return nil, fmt.Errorf("a request input mapping requires key")
			}
			item["key"] = key.ValueString()
		case "fixed":
			value, err := exWorkflowTriggerFixedValue(ctx, mapping)
			if err != nil {
				return nil, err
			}
			item["value"] = value
		default:
			return nil, fmt.Errorf("input mapping source must be fixed or request")
		}
		out = append(out, item)
	}
	body["input_mappings"] = out
	return body, nil
}

func exWorkflowTriggerMappingValue(_ context.Context, raw map[string]any, typesByName map[string]attr.Type) (types.Object, error) {
	parameter, ok := raw["parameter"].(string)
	if !ok || parameter == "" {
		return types.Object{}, fmt.Errorf("workflow trigger mapping has invalid parameter")
	}
	source, ok := raw["source"].(string)
	if !ok || source == "" {
		return types.Object{}, fmt.Errorf("workflow trigger mapping has invalid source")
	}
	values := map[string]attr.Value{}
	values["parameter"] = types.StringValue(parameter)
	values["source"] = types.StringValue(source)
	values["key"] = types.StringNull()
	values["fixed_string"] = types.StringNull()
	values["fixed_integer"] = types.Int64Null()
	values["fixed_boolean"] = types.BoolNull()
	values["fixed_secret_reference"] = types.ObjectNull(map[string]attr.Type{"access_key_id": types.Int64Type, "global_credential_id": types.Int64Type})
	if key, ok := raw["key"].(string); ok {
		values["key"] = types.StringValue(key)
	}
	if value, ok := raw["value"]; ok {
		switch v := value.(type) {
		case string:
			values["fixed_string"] = types.StringValue(v)
		case bool:
			values["fixed_boolean"] = types.BoolValue(v)
		case json.Number:
			n, err := v.Int64()
			if err != nil {
				return types.Object{}, fmt.Errorf("workflow trigger fixed value must be an integer")
			}
			values["fixed_integer"] = types.Int64Value(n)
		case map[string]any:
			secret := map[string]attr.Value{"access_key_id": types.Int64Null(), "global_credential_id": types.Int64Null()}
			if id, ok := v["access_key_id"]; ok {
				parsed, err := identityNumber(id)
				if err != nil {
					return types.Object{}, err
				}
				secret["access_key_id"] = types.Int64Value(parsed)
			}
			if id, ok := v["global_credential_id"]; ok {
				parsed, err := identityNumber(id)
				if err != nil {
					return types.Object{}, err
				}
				secret["global_credential_id"] = types.Int64Value(parsed)
			}
			values["fixed_secret_reference"] = types.ObjectValueMust(map[string]attr.Type{"access_key_id": types.Int64Type, "global_credential_id": types.Int64Type}, secret)
		default:
			return types.Object{}, fmt.Errorf("workflow trigger fixed value has unsupported type")
		}
	}
	return types.ObjectValueMust(typesByName, values), nil
}

func exWorkflowTriggerState(ctx context.Context, old exWorkflowTriggerModel, raw map[string]any) (exWorkflowTriggerModel, error) {
	next := old
	if next.CronFormat.IsUnknown() {
		next.CronFormat = types.StringNull()
	}
	if next.Credential.IsUnknown() {
		next.Credential = types.StringNull()
	}
	if next.WebhookSigningSecret.IsUnknown() {
		next.WebhookSigningSecret = types.StringNull()
	}
	if next.NextWebhookSigningSecret.IsUnknown() {
		next.NextWebhookSigningSecret = types.StringNull()
	}
	if next.CurrentSigningKeyID.IsUnknown() {
		next.CurrentSigningKeyID = types.StringNull()
	}
	if next.NextSigningKeyID.IsUnknown() {
		next.NextSigningKeyID = types.StringNull()
	}
	if next.LastFired.IsUnknown() {
		next.LastFired = types.StringNull()
	}
	if next.LastResult.IsUnknown() {
		next.LastResult = types.StringNull()
	}
	if next.CredentialGeneration.IsUnknown() {
		next.CredentialGeneration = types.Int64Null()
	}
	if next.CurrentSigningGeneration.IsUnknown() {
		next.CurrentSigningGeneration = types.Int64Null()
	}
	if next.NextSigningGeneration.IsUnknown() {
		next.NextSigningGeneration = types.Int64Null()
	}
	if next.CredentialRotationVersion.IsUnknown() {
		next.CredentialRotationVersion = types.Int64Null()
	}
	if next.SigningStageVersion.IsUnknown() {
		next.SigningStageVersion = types.Int64Null()
	}
	if next.SigningBootstrapVersion.IsUnknown() {
		next.SigningBootstrapVersion = types.Int64Null()
	}
	if next.InputMappings.IsUnknown() || next.InputMappings.IsNull() {
		mappingTypes := exWorkflowTriggerMappingTypes()
		next.InputMappings = types.ListValueMust(types.ObjectType{AttrTypes: mappingTypes}, []attr.Value{})
	}
	for name, target := range map[string]*types.Int64{"id": &next.ID, "project_id": &next.ProjectID, "workflow_template_id": &next.WorkflowID, "revision": &next.Revision, "credential_generation": &next.CredentialGeneration, "current_signing_generation": &next.CurrentSigningGeneration, "next_signing_generation": &next.NextSigningGeneration} {
		if value, ok := raw[name]; ok {
			n, err := identityNumber(value)
			if err != nil {
				return old, fmt.Errorf("invalid workflow trigger %s", name)
			}
			*target = types.Int64Value(n)
		}
	}
	for name, target := range map[string]*types.String{"name": &next.Name, "type": &next.Type, "cron_format": &next.CronFormat, "current_signing_key_id": &next.CurrentSigningKeyID, "next_signing_key_id": &next.NextSigningKeyID, "last_result": &next.LastResult} {
		if value, ok := raw[name]; ok && value != nil {
			text, ok := value.(string)
			if !ok {
				return old, fmt.Errorf("invalid workflow trigger %s", name)
			}
			*target = types.StringValue(text)
		}
	}
	for name, target := range map[string]*types.String{"current_signing_key_id": &next.CurrentSigningKeyID, "next_signing_key_id": &next.NextSigningKeyID} {
		if _, present := raw[name]; !present {
			*target = types.StringNull()
		}
	}
	for name, target := range map[string]*types.Int64{"current_signing_generation": &next.CurrentSigningGeneration, "next_signing_generation": &next.NextSigningGeneration} {
		if _, present := raw[name]; !present {
			*target = types.Int64Null()
		}
	}
	if value, ok := raw["enabled"].(bool); ok {
		next.Enabled = types.BoolValue(value)
	}
	if !old.CredentialGeneration.IsNull() && !old.CredentialGeneration.IsUnknown() && !next.CredentialGeneration.IsNull() && !next.CredentialGeneration.IsUnknown() && old.CredentialGeneration.ValueInt64() != next.CredentialGeneration.ValueInt64() {
		next.Credential = types.StringNull()
	}
	if !old.CurrentSigningGeneration.Equal(next.CurrentSigningGeneration) || !old.CurrentSigningKeyID.Equal(next.CurrentSigningKeyID) {
		next.WebhookSigningSecret = types.StringNull()
	}
	if !old.NextSigningGeneration.Equal(next.NextSigningGeneration) || !old.NextSigningKeyID.Equal(next.NextSigningKeyID) {
		next.NextWebhookSigningSecret = types.StringNull()
	}
	if value, ok := raw["last_fired"].(string); ok {
		next.LastFired = types.StringValue(value)
	} else if _, ok := raw["last_fired"]; ok {
		next.LastFired = types.StringNull()
	}
	if rawMappings, ok := raw["input_mappings"].([]any); ok {
		mappingTypes := exWorkflowTriggerMappingTypes()
		items := make([]attr.Value, 0, len(rawMappings))
		for _, rawMapping := range rawMappings {
			mapping, ok := rawMapping.(map[string]any)
			if !ok {
				return old, fmt.Errorf("invalid workflow trigger input mapping")
			}
			item, err := exWorkflowTriggerMappingValue(ctx, mapping, mappingTypes)
			if err != nil {
				return old, err
			}
			items = append(items, item)
		}
		next.InputMappings = types.ListValueMust(types.ObjectType{AttrTypes: mappingTypes}, items)
	}
	return next, nil
}

func exWorkflowTriggerRead(ctx context.Context, client *apiclient.SemaphoreUI, old exWorkflowTriggerModel) (exWorkflowTriggerModel, error) {
	var raw map[string]any
	if err := exRequest(ctx, client, http.MethodGet, exWorkflowTriggerRoute(true), exWorkflowTriggerParams(old), nil, &raw); err != nil {
		return old, err
	}
	return exWorkflowTriggerState(ctx, old, raw)
}

func (r *exWorkflowTriggerResource) Create(ctx context.Context, q resource.CreateRequest, p *resource.CreateResponse) {
	var plan exWorkflowTriggerModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &plan)...)
	if p.Diagnostics.HasError() {
		return
	}
	if plan.Type.IsNull() || plan.Type.IsUnknown() {
		p.Diagnostics.AddError("Invalid Workflow Trigger Type", "type must be known before creation.")
		return
	}
	if !plan.CredentialRotationVersion.IsNull() && !plan.CredentialRotationVersion.IsUnknown() && plan.Type.ValueString() != "api" {
		p.Diagnostics.AddError("Invalid Workflow Trigger Credential Rotation", "Credential rotation requires type = api.")
		return
	}
	if !plan.SigningBootstrapVersion.IsNull() && !plan.SigningBootstrapVersion.IsUnknown() {
		p.Diagnostics.AddError("Invalid Workflow Trigger Signing Bootstrap", "Bootstrap is only for imported legacy webhook triggers without signing material.")
		return
	}
	if !plan.SigningStageVersion.IsNull() && !plan.SigningStageVersion.IsUnknown() && plan.Type.ValueString() != "webhook" {
		p.Diagnostics.AddError("Invalid Workflow Trigger Signing Operation", "Webhook signing operations require type = webhook.")
		return
	}
	body, err := exWorkflowTriggerBody(ctx, plan, 0)
	if err != nil {
		p.Diagnostics.AddError("Invalid Workflow Trigger", err.Error())
		return
	}
	var result struct {
		Trigger              map[string]any `json:"trigger"`
		Credential           string         `json:"credential"`
		WebhookSigningSecret string         `json:"webhook_signing_secret"`
	}
	if err = exRequest(ctx, r.client, http.MethodPost, exWorkflowTriggerRoute(false), exWorkflowTriggerParams(plan), body, &result); err != nil {
		p.Diagnostics.AddError("Error Creating Semaphore EX Workflow Trigger", err.Error())
		return
	}
	state, err := exWorkflowTriggerState(ctx, plan, result.Trigger)
	if err != nil {
		p.Diagnostics.AddError("Invalid Workflow Trigger Response", err.Error())
		return
	}
	if result.Credential != "" {
		state.Credential = types.StringValue(result.Credential)
	}
	if result.WebhookSigningSecret != "" {
		state.WebhookSigningSecret = types.StringValue(result.WebhookSigningSecret)
	}
	if plan.Type.ValueString() == "api" && result.Credential == "" {
		p.Diagnostics.Append(p.State.Set(ctx, &state)...)
		p.Diagnostics.AddError("Invalid Workflow Trigger Response", "Server did not return one-time API credential material.")
		return
	}
	if plan.Type.ValueString() == "webhook" && result.WebhookSigningSecret == "" {
		p.Diagnostics.Append(p.State.Set(ctx, &state)...)
		p.Diagnostics.AddError("Invalid Workflow Trigger Response", "Server did not return one-time webhook signing secret material.")
		return
	}
	if !plan.SigningStageVersion.IsNull() && !plan.SigningStageVersion.IsUnknown() {
		revision, revisionErr := exWorkflowTriggerRevision(state)
		if revisionErr != nil {
			p.Diagnostics.AddError("Workflow Trigger Revision Unavailable", revisionErr.Error())
			return
		}
		var staged struct {
			Trigger              map[string]any `json:"trigger"`
			WebhookSigningSecret string         `json:"webhook_signing_secret"`
		}
		if err = exRequest(ctx, r.client, http.MethodPost, exWorkflowTriggerRoute(true)+"/webhook-signing/stage", exWorkflowTriggerParams(state), map[string]any{"revision": revision}, &staged); err != nil {
			p.Diagnostics.Append(p.State.Set(ctx, &state)...)
			p.Diagnostics.AddError("Error Staging Semaphore EX Workflow Trigger Signing Key", err.Error())
			return
		}
		state, err = exWorkflowTriggerState(ctx, state, staged.Trigger)
		if err != nil {
			p.Diagnostics.Append(p.State.Set(ctx, &state)...)
			p.Diagnostics.AddError("Invalid Workflow Trigger Signing Response", err.Error())
			return
		}
		if staged.WebhookSigningSecret == "" {
			state.SigningStageVersion = plan.SigningStageVersion
			p.Diagnostics.Append(p.State.Set(ctx, &state)...)
			p.Diagnostics.AddError("Invalid Workflow Trigger Signing Response", "Server did not return one-time signing secret material.")
			return
		}
		state.NextWebhookSigningSecret = types.StringValue(staged.WebhookSigningSecret)
		state.SigningStageVersion = plan.SigningStageVersion
	}
	p.Diagnostics.Append(p.State.Set(ctx, &state)...)
}
func (r *exWorkflowTriggerResource) Read(ctx context.Context, q resource.ReadRequest, p *resource.ReadResponse) {
	var state exWorkflowTriggerModel
	p.Diagnostics.Append(q.State.Get(ctx, &state)...)
	if p.Diagnostics.HasError() {
		return
	}
	next, err := exWorkflowTriggerRead(ctx, r.client, state)
	if exNotFound(err) {
		p.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		p.Diagnostics.AddError("Error Reading Semaphore EX Workflow Trigger", err.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func exWorkflowTriggerRevision(m exWorkflowTriggerModel) (int64, error) {
	if m.Revision.IsNull() || m.Revision.IsUnknown() || m.Revision.ValueInt64() < 1 {
		return 0, fmt.Errorf("refresh the workflow trigger and retry; the provider will not overwrite a concurrent trigger change")
	}
	return m.Revision.ValueInt64(), nil
}
func exWorkflowTriggerVersionRequested(plan, state types.Int64) bool {
	return !plan.IsNull() && !plan.IsUnknown() && !plan.Equal(state)
}
func (r *exWorkflowTriggerResource) Update(ctx context.Context, q resource.UpdateRequest, p *resource.UpdateResponse) {
	var plan, state exWorkflowTriggerModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &plan)...)
	p.Diagnostics.Append(q.State.Get(ctx, &state)...)
	if p.Diagnostics.HasError() {
		return
	}
	revision, err := exWorkflowTriggerRevision(state)
	if err != nil {
		p.Diagnostics.AddError("Workflow Trigger Revision Unavailable", err.Error())
		return
	}
	plan.ID, plan.ProjectID, plan.WorkflowID = state.ID, state.ProjectID, state.WorkflowID
	plan.Type = state.Type
	credentialRotationRequested := exWorkflowTriggerVersionRequested(plan.CredentialRotationVersion, state.CredentialRotationVersion)
	signingStageRequested := exWorkflowTriggerVersionRequested(plan.SigningStageVersion, state.SigningStageVersion)
	signingBootstrapRequested := exWorkflowTriggerVersionRequested(plan.SigningBootstrapVersion, state.SigningBootstrapVersion)
	if credentialRotationRequested && plan.Type.ValueString() != "api" {
		p.Diagnostics.AddError("Invalid Workflow Trigger Credential Rotation", "Credential rotation requires type = api.")
		return
	}
	if (signingStageRequested || signingBootstrapRequested) && plan.Type.ValueString() != "webhook" {
		p.Diagnostics.AddError("Invalid Workflow Trigger Signing Operation", "Webhook signing operations require type = webhook.")
		return
	}
	current := state
	if !plan.Name.Equal(state.Name) || !plan.Enabled.Equal(state.Enabled) || !plan.CronFormat.Equal(state.CronFormat) || !plan.InputMappings.Equal(state.InputMappings) {
		body, bodyErr := exWorkflowTriggerBody(ctx, plan, revision)
		if bodyErr != nil {
			p.Diagnostics.AddError("Invalid Workflow Trigger", bodyErr.Error())
			return
		}
		var raw map[string]any
		if err = exRequest(ctx, r.client, http.MethodPut, exWorkflowTriggerRoute(true), exWorkflowTriggerParams(plan), body, &raw); err != nil {
			p.Diagnostics.AddError("Error Updating Semaphore EX Workflow Trigger", err.Error())
			return
		}
		responseBase := state
		responseBase.Name = plan.Name
		responseBase.Enabled = plan.Enabled
		responseBase.CronFormat = plan.CronFormat
		responseBase.InputMappings = plan.InputMappings
		current, err = exWorkflowTriggerState(ctx, responseBase, raw)
		if err != nil {
			p.Diagnostics.AddError("Invalid Workflow Trigger Response", err.Error())
			return
		}
	}
	if credentialRotationRequested {
		currentRevision, revisionErr := exWorkflowTriggerRevision(current)
		if revisionErr != nil {
			p.Diagnostics.AddError("Workflow Trigger Revision Unavailable", revisionErr.Error())
			return
		}
		var result struct {
			Trigger    map[string]any `json:"trigger"`
			Credential string         `json:"credential"`
		}
		if err = exRequest(ctx, r.client, http.MethodPost, exWorkflowTriggerRoute(true)+"/rotate", exWorkflowTriggerParams(current), map[string]any{"revision": currentRevision}, &result); err != nil {
			p.Diagnostics.Append(p.State.Set(ctx, &current)...)
			p.Diagnostics.AddError("Error Rotating Semaphore EX Workflow Trigger Credential", err.Error())
			return
		}
		current, err = exWorkflowTriggerState(ctx, current, result.Trigger)
		if err != nil {
			p.Diagnostics.AddError("Invalid Workflow Trigger Rotation Response", err.Error())
			return
		}
		if result.Credential == "" {
			current.CredentialRotationVersion = plan.CredentialRotationVersion
			p.Diagnostics.Append(p.State.Set(ctx, &current)...)
			p.Diagnostics.AddError("Invalid Workflow Trigger Rotation Response", "Server did not return one-time replacement credential material.")
			return
		}
		current.Credential = types.StringValue(result.Credential)
		current.CredentialRotationVersion = plan.CredentialRotationVersion
	}
	for _, operation := range []struct {
		changed      bool
		route, label string
		version      types.Int64
		bootstrap    bool
	}{
		{signingBootstrapRequested, "/webhook-signing/bootstrap", "Bootstrapping", plan.SigningBootstrapVersion, true},
		{signingStageRequested, "/webhook-signing/stage", "Staging", plan.SigningStageVersion, false},
	} {
		if !operation.changed {
			continue
		}
		if current.Type.ValueString() != "webhook" {
			p.Diagnostics.AddError("Invalid Workflow Trigger Signing Operation", "Webhook signing operations require type = webhook.")
			return
		}
		currentRevision, revisionErr := exWorkflowTriggerRevision(current)
		if revisionErr != nil {
			p.Diagnostics.AddError("Workflow Trigger Revision Unavailable", revisionErr.Error())
			return
		}
		var result struct {
			Trigger              map[string]any `json:"trigger"`
			WebhookSigningSecret string         `json:"webhook_signing_secret"`
		}
		if err = exRequest(ctx, r.client, http.MethodPost, exWorkflowTriggerRoute(true)+operation.route, exWorkflowTriggerParams(current), map[string]any{"revision": currentRevision}, &result); err != nil {
			p.Diagnostics.Append(p.State.Set(ctx, &current)...)
			p.Diagnostics.AddError("Error "+operation.label+" Semaphore EX Workflow Trigger Signing Key", err.Error())
			return
		}
		current, err = exWorkflowTriggerState(ctx, current, result.Trigger)
		if err != nil {
			p.Diagnostics.AddError("Invalid Workflow Trigger Signing Response", err.Error())
			return
		}
		if result.WebhookSigningSecret == "" {
			if operation.bootstrap {
				current.SigningBootstrapVersion = operation.version
			} else {
				current.SigningStageVersion = operation.version
			}
			p.Diagnostics.Append(p.State.Set(ctx, &current)...)
			p.Diagnostics.AddError("Invalid Workflow Trigger Signing Response", "Server did not return one-time signing secret material.")
			return
		}
		if operation.bootstrap {
			current.WebhookSigningSecret = types.StringValue(result.WebhookSigningSecret)
		} else {
			current.NextWebhookSigningSecret = types.StringValue(result.WebhookSigningSecret)
		}
		if operation.bootstrap {
			current.SigningBootstrapVersion = operation.version
		} else {
			current.SigningStageVersion = operation.version
		}
	}
	current.CredentialRotationVersion = plan.CredentialRotationVersion
	current.SigningStageVersion = plan.SigningStageVersion
	current.SigningBootstrapVersion = plan.SigningBootstrapVersion
	next := current
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (r *exWorkflowTriggerResource) Delete(ctx context.Context, q resource.DeleteRequest, p *resource.DeleteResponse) {
	var state exWorkflowTriggerModel
	p.Diagnostics.Append(q.State.Get(ctx, &state)...)
	if p.Diagnostics.HasError() {
		return
	}
	revision, err := exWorkflowTriggerRevision(state)
	if err != nil {
		p.Diagnostics.AddError("Workflow Trigger Revision Unavailable", err.Error())
		return
	}
	err = exRequestWithOptions(ctx, r.client, http.MethodDelete, exWorkflowTriggerRoute(true), exRequestOptions{PathParams: exWorkflowTriggerParams(state), Query: map[string]string{"expected_revision": strconv.FormatInt(revision, 10)}}, nil, nil)
	if err != nil && !exNotFound(err) {
		p.Diagnostics.AddError("Error Removing Semaphore EX Workflow Trigger", err.Error())
	}
}
func (r *exWorkflowTriggerResource) ImportState(ctx context.Context, q resource.ImportStateRequest, p *resource.ImportStateResponse) {
	fields, err := parseImportFields(q.ID, []string{"project", "workflow", "trigger"})
	if err != nil {
		p.Diagnostics.AddError("Invalid Workflow Trigger Import ID", err.Error())
		return
	}
	state := exWorkflowTriggerModel{ProjectID: types.Int64Value(fields["project"]), WorkflowID: types.Int64Value(fields["workflow"]), ID: types.Int64Value(fields["trigger"])}
	next, err := exWorkflowTriggerRead(ctx, r.client, state)
	if err != nil {
		p.Diagnostics.AddError("Error Importing Semaphore EX Workflow Trigger", err.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (d *exWorkflowTriggerDataSource) Read(ctx context.Context, q datasource.ReadRequest, p *datasource.ReadResponse) {
	var config exWorkflowTriggerDataSourceModel
	p.Diagnostics.Append(q.Config.Get(ctx, &config)...)
	if p.Diagnostics.HasError() {
		return
	}
	old := exWorkflowTriggerModel{ID: config.ID, ProjectID: config.ProjectID, WorkflowID: config.WorkflowID}
	next, err := exWorkflowTriggerRead(ctx, d.client, old)
	if err != nil {
		p.Diagnostics.AddError("Error Reading Semaphore EX Workflow Trigger", err.Error())
		return
	}
	state := exWorkflowTriggerDataSourceModel{ID: next.ID, ProjectID: next.ProjectID, WorkflowID: next.WorkflowID, Name: next.Name, Type: next.Type, Enabled: next.Enabled, CronFormat: next.CronFormat, InputMappings: next.InputMappings, Revision: next.Revision, CredentialGeneration: next.CredentialGeneration, CurrentSigningKeyID: next.CurrentSigningKeyID, NextSigningKeyID: next.NextSigningKeyID, CurrentSigningGeneration: next.CurrentSigningGeneration, NextSigningGeneration: next.NextSigningGeneration, LastFired: next.LastFired, LastResult: next.LastResult}
	p.Diagnostics.Append(p.State.Set(ctx, &state)...)
}
