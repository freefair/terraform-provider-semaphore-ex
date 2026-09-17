package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gopkg.in/yaml.v3"
)

type exPolicyGuardrailAction struct {
	client           *apiclient.SemaphoreUI
	project, publish bool
}
type exPolicyGuardrailDataSource struct {
	client  *apiclient.SemaphoreUI
	project bool
}

func NewGlobalPolicyGuardrailDraftSaveAction() action.Action { return &exPolicyGuardrailAction{} }
func NewProjectPolicyGuardrailDraftSaveAction() action.Action {
	return &exPolicyGuardrailAction{project: true}
}
func NewGlobalPolicyGuardrailPublishAction() action.Action {
	return &exPolicyGuardrailAction{publish: true}
}
func NewProjectPolicyGuardrailPublishAction() action.Action {
	return &exPolicyGuardrailAction{project: true, publish: true}
}
func NewGlobalPolicyGuardrailDataSource() datasource.DataSource {
	return &exPolicyGuardrailDataSource{}
}
func NewProjectPolicyGuardrailDataSource() datasource.DataSource {
	return &exPolicyGuardrailDataSource{project: true}
}
func (a *exPolicyGuardrailAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	scope := "global"
	if a.project {
		scope = "project"
	}
	op := "draft_save"
	if a.publish {
		op = "publish"
	}
	resp.TypeName = req.ProviderTypeName + "_" + scope + "_policy_guardrail_" + op
}
func (a *exPolicyGuardrailAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	a.client = exPolicyGuardrailActionClient(req.ProviderData, &resp.Diagnostics)
}
func (d *exPolicyGuardrailDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	scope := "global"
	if d.project {
		scope = "project"
	}
	resp.TypeName = req.ProviderTypeName + "_" + scope + "_policy_guardrail"
}
func (d *exPolicyGuardrailDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = exPolicyGuardrailActionClient(req.ProviderData, &resp.Diagnostics)
}
func exPolicyGuardrailActionClient(value any, diagnostics interface{ AddError(string, string) }) *apiclient.SemaphoreUI {
	if value == nil {
		return nil
	}
	c, ok := value.(*apiclient.SemaphoreUI)
	if !ok {
		diagnostics.AddError("Unexpected Policy Guardrail Configure Type", "Expected the configured Semaphore EX client.")
		return nil
	}
	return c
}
func (a *exPolicyGuardrailAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	attrs := map[string]schema.Attribute{"expected_revision": schema.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}}}
	if a.project {
		attrs["project_id"] = schema.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}}
	} else {
		attrs["project_id"] = schema.Int64Attribute{Optional: true}
	}
	attrs["source_yaml"] = schema.StringAttribute{Optional: true, MarkdownDescription: "Authored YAML escape hatch; set exactly one of source_yaml or policy for draft saves."}
	attrs["policy"] = schema.DynamicAttribute{Optional: true, MarkdownDescription: "Native HCL policy object. Set exactly one of policy or source_yaml for draft saves."}
	op := "Publishes the current draft as an immutable policy revision."
	if !a.publish {
		op = "Saves a revision-fenced policy guardrail draft without publishing it."
	}
	resp.Schema = schema.Schema{MarkdownDescription: op + " Invoke explicitly; no refresh performs a policy mutation.", Attributes: attrs}
}
func (a *exPolicyGuardrailAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var c struct {
		ProjectID        types.Int64   `tfsdk:"project_id"`
		ExpectedRevision types.Int64   `tfsdk:"expected_revision"`
		SourceYAML       types.String  `tfsdk:"source_yaml"`
		Policy           types.Dynamic `tfsdk:"policy"`
	}
	resp.Diagnostics.Append(req.Config.Get(ctx, &c)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if c.ExpectedRevision.IsNull() || c.ExpectedRevision.IsUnknown() {
		resp.Diagnostics.AddError("Unknown Policy Guardrail Revision", "expected_revision must be known.")
		return
	}
	params := map[string]string(nil)
	if a.project {
		if c.ProjectID.IsNull() || c.ProjectID.IsUnknown() || c.ProjectID.ValueInt64() < 1 {
			resp.Diagnostics.AddError("Invalid Policy Guardrail Project", "project_id must be known and positive.")
			return
		}
		params = map[string]string{"project_id": strconv.FormatInt(c.ProjectID.ValueInt64(), 10)}
	}
	if a.publish {
		if (!a.project && !c.ProjectID.IsNull()) || !c.SourceYAML.IsNull() || !c.Policy.IsNull() {
			resp.Diagnostics.AddError("Invalid Policy Guardrail Publish Input", "publish accepts only expected_revision (and project_id for project scope).")
			return
		}
		if err := exRequest(ctx, a.client, http.MethodPost, exPolicyGuardrailRoute(a.project, "/publish"), params, map[string]any{"expected_draft_revision": c.ExpectedRevision.ValueInt64()}, nil); err != nil {
			resp.Diagnostics.AddError("Error Publishing Semaphore EX Policy Guardrail", err.Error())
		}
		return
	}
	if !a.project && !c.ProjectID.IsNull() {
		resp.Diagnostics.AddError("Invalid Global Policy Guardrail Project", "Global actions do not accept project_id.")
		return
	}
	if (!c.SourceYAML.IsNull() && !c.Policy.IsNull()) || (c.SourceYAML.IsNull() && c.Policy.IsNull()) {
		resp.Diagnostics.AddError("Invalid Policy Guardrail Source", "Set exactly one of source_yaml or policy.")
		return
	}
	source := c.SourceYAML.ValueString()
	if !c.Policy.IsNull() {
		if c.Policy.IsUnknown() {
			resp.Diagnostics.AddError("Unknown Policy Guardrail Policy", "policy must be known.")
			return
		}
		wire, err := exWireValue(ctx, c.Policy)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Policy Guardrail Policy", err.Error())
			return
		}
		source, err = exPolicyGuardrailNativeYAML(wire)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Policy Guardrail Policy", err.Error())
			return
		}
	}
	if err := exRequest(ctx, a.client, http.MethodPut, exPolicyGuardrailRoute(a.project, "/draft"), params, map[string]any{"source_yaml": source, "expected_revision": c.ExpectedRevision.ValueInt64()}, nil); err != nil {
		resp.Diagnostics.AddError("Error Saving Semaphore EX Policy Guardrail Draft", err.Error())
	}
}

func exPolicyGuardrailNativeYAML(value any) (string, error) {
	normalized, err := exPolicyGuardrailYAMLValue(value)
	if err != nil {
		return "", err
	}
	encoded, err := yaml.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("could not encode native policy YAML")
	}
	return string(encoded), nil
}
func exPolicyGuardrailYAMLValue(value any) (any, error) {
	switch v := value.(type) {
	case json.Number:
		if integer, err := v.Int64(); err == nil {
			return integer, nil
		}
		return nil, fmt.Errorf("policy number %q is not an integer", v)
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, item := range v {
			converted, err := exPolicyGuardrailYAMLValue(item)
			if err != nil {
				return nil, err
			}
			result[key] = converted
		}
		return result, nil
	case []any:
		result := make([]any, len(v))
		for index, item := range v {
			converted, err := exPolicyGuardrailYAMLValue(item)
			if err != nil {
				return nil, err
			}
			result[index] = converted
		}
		return result, nil
	default:
		return value, nil
	}
}
func (d *exPolicyGuardrailDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := map[string]ds.Attribute{"id": ds.StringAttribute{Computed: true}, "source_yaml": ds.StringAttribute{Computed: true}, "revision": ds.Int64Attribute{Computed: true}, "active_revision": ds.Int64Attribute{Computed: true}}
	if d.project {
		attrs["project_id"] = ds.Int64Attribute{Required: true}
	} else {
		attrs["project_id"] = ds.Int64Attribute{Computed: true}
	}
	resp.Schema = ds.Schema{MarkdownDescription: "Reads a Semaphore EX policy-guardrail draft and active revision.", Attributes: attrs}
}
func exPolicyGuardrailRoute(project bool, suffix string) string {
	base := "/policy-guardrails"
	if project {
		base = "/project/{project_id}/policy-guardrails"
	}
	return base + suffix
}
func (d *exPolicyGuardrailDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var c struct {
		ID             types.String `tfsdk:"id"`
		ProjectID      types.Int64  `tfsdk:"project_id"`
		SourceYAML     types.String `tfsdk:"source_yaml"`
		Revision       types.Int64  `tfsdk:"revision"`
		ActiveRevision types.Int64  `tfsdk:"active_revision"`
	}
	resp.Diagnostics.Append(req.Config.Get(ctx, &c)...)
	if resp.Diagnostics.HasError() {
		return
	}
	params := map[string]string(nil)
	if d.project {
		id, err := exPathID(c.ProjectID)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Policy Guardrail Project", err.Error())
			return
		}
		params = map[string]string{"project_id": id}
	}
	var raw struct {
		Draft struct {
			SourceYAML     string `json:"source_yaml"`
			Revision       int64  `json:"revision"`
			ActiveRevision *int64 `json:"active_revision"`
		} `json:"draft"`
	}
	if err := exRequest(ctx, d.client, http.MethodGet, exPolicyGuardrailRoute(d.project, ""), params, nil, &raw); err != nil {
		resp.Diagnostics.AddError("Error Reading Semaphore EX Policy Guardrail", err.Error())
		return
	}
	c.SourceYAML = types.StringValue(raw.Draft.SourceYAML)
	c.Revision = types.Int64Value(raw.Draft.Revision)
	if raw.Draft.ActiveRevision == nil {
		c.ActiveRevision = types.Int64Null()
	} else {
		c.ActiveRevision = types.Int64Value(*raw.Draft.ActiveRevision)
	}
	if d.project {
		c.ID = types.StringValue("project/" + strconv.FormatInt(c.ProjectID.ValueInt64(), 10))
	} else {
		c.ID = types.StringValue("global")
		c.ProjectID = types.Int64Null()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &c)...)
}
