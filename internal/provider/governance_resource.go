package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rs "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type governanceResource struct {
	client         *apiclient.SemaphoreUI
	project, draft bool
}

func NewGlobalPolicyGuardrailResource() resource.Resource { return &governanceResource{draft: true} }
func NewProjectPolicyGuardrailResource() resource.Resource {
	return &governanceResource{draft: true, project: true}
}
func NewGlobalWorkflowArtifactRetentionResource() resource.Resource { return &governanceResource{} }
func NewProjectWorkflowArtifactRetentionResource() resource.Resource {
	return &governanceResource{project: true}
}
func (r *governanceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	scope := "global"
	if r.project {
		scope = "project"
	}
	kind := "workflow_artifact_retention"
	if r.draft {
		kind = "policy_guardrail"
	}
	resp.TypeName = req.ProviderTypeName + "_" + scope + "_" + kind
}
func (r *governanceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attributes := map[string]rs.Attribute{"id": rs.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}, "revision": rs.Int64Attribute{Computed: true, MarkdownDescription: "Owned draft or policy revision. Updates use the prior state revision and never retry conflicts automatically."}}
	if r.project {
		attributes["project_id"] = rs.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}}
	}
	description := "Manages an append-only workflow artifact retention policy for this scope. Read tracks the configured policy, not inherited effective limits. Destroy forgets Terraform ownership; published history and effective server settings remain intact."
	if r.draft {
		attributes["source_yaml"] = rs.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}, MarkdownDescription: "Authored policy YAML. Use yamlencode({...}) for native HCL authoring. Saving a draft never publishes it."}
		description = "Manages the policy-guardrail draft without publishing it. Publication remains an explicit policy_guardrail_publish Action using the reviewed draft revision. Destroy forgets ownership and preserves the draft and published history."
	} else {
		for name, minimum := range map[string]int64{"retention_seconds": 3600, "max_artifact_bytes": 1, "max_run_bytes": 1} {
			attributes[name] = rs.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(minimum)}}
		}
	}
	resp.Schema = rs.Schema{MarkdownDescription: description, Attributes: attributes}
}
func (r *governanceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	var ok bool
	r.client, ok = req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Invalid Provider Client", "Expected the configured Semaphore EX client.")
	}
}
func (r *governanceResource) route(write bool) string {
	if r.draft {
		suffix := ""
		if write {
			suffix = "/draft"
		}
		return exPolicyGuardrailRoute(r.project, suffix)
	}
	return exWorkflowArtifactRetentionRoute(r.project)
}
func (r *governanceResource) identity(value types.Object) (string, map[string]string, error) {
	if !r.project {
		return "global", nil, nil
	}
	id, err := exPathID(value.Attributes()["project_id"])
	if err != nil {
		return "", nil, err
	}
	return "project/" + id, map[string]string{"project_id": id}, nil
}
func (r *governanceResource) read(ctx context.Context, value types.Object) (map[string]any, bool, error) {
	_, parameters, err := r.identity(value)
	if err != nil {
		return nil, false, err
	}
	var response map[string]any
	if err := exRequest(ctx, r.client, http.MethodGet, r.route(false), parameters, nil, &response); err != nil {
		return nil, false, err
	}
	if r.draft {
		draft, ok := response["draft"].(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("server returned no guardrail draft")
		}
		return draft, true, nil
	}
	if _, ok := response["effective"].(map[string]any); !ok {
		return nil, false, fmt.Errorf("server returned an invalid retention state")
	}
	field := "global_policy"
	if r.project {
		field = "project_policy"
	}
	if response[field] == nil {
		return nil, false, nil
	}
	policy, ok := response[field].(map[string]any)
	if !ok {
		return nil, false, fmt.Errorf("server returned an invalid owned retention policy")
	}
	return policy, true, nil
}
func (r *governanceResource) state(ctx context.Context, previous types.Object, record map[string]any) (types.Object, error) {
	id, _, err := r.identity(previous)
	if err != nil {
		return types.Object{}, err
	}
	revision, err := identityNumber(record["revision"])
	if err != nil || revision < 1 {
		return types.Object{}, fmt.Errorf("server returned an invalid governance revision")
	}
	expectedScope := "global"
	if r.project {
		expectedScope = "project"
	}
	if record["scope"] != expectedScope || (!r.project && record["project_id"] != nil) {
		return types.Object{}, fmt.Errorf("server returned a governance record from another scope")
	}
	values := previous.Attributes()
	values["id"] = types.StringValue(id)
	values["revision"] = types.Int64Value(revision)
	for _, field := range r.fields() {
		value, err := exTypedValue(ctx, previous.AttributeTypes(ctx)[field], record[field])
		if err != nil || value.IsNull() {
			return types.Object{}, fmt.Errorf("server returned an invalid governance field %s", field)
		}
		values[field] = value
	}
	if r.project {
		scope, err := identityNumber(record["project_id"])
		expected, _ := values["project_id"].(types.Int64)
		if err != nil || scope != expected.ValueInt64() {
			return types.Object{}, fmt.Errorf("server returned a governance record from another project")
		}
	}
	result, diags := types.ObjectValue(previous.AttributeTypes(ctx), values)
	if diags.HasError() {
		return types.Object{}, fmt.Errorf("governance state does not match the schema")
	}
	return result, nil
}
func (r *governanceResource) fields() []string {
	if r.draft {
		return []string{"source_yaml"}
	}
	return []string{"retention_seconds", "max_artifact_bytes", "max_run_bytes"}
}
func (r *governanceResource) write(ctx context.Context, plan types.Object, revision int64) (types.Object, error) {
	_, parameters, err := r.identity(plan)
	if err != nil {
		return types.Object{}, err
	}
	body := map[string]any{"expected_revision": revision}
	for _, field := range r.fields() {
		value := plan.Attributes()[field]
		if value.IsNull() || value.IsUnknown() {
			return types.Object{}, fmt.Errorf("%s must be known before applying", field)
		}
		wire, err := exWireValue(ctx, value)
		if err != nil {
			return types.Object{}, err
		}
		body[field] = wire
	}
	var response map[string]any
	if err := exRequest(ctx, r.client, http.MethodPut, r.route(true), parameters, body, &response); err != nil {
		return types.Object{}, err
	}
	// Retain the deterministic identity after a successful mutation even if a
	// malformed or concurrently changed response prevents complete reconciliation.
	partialRecord := map[string]any{"revision": revision + 1, "scope": "global"}
	if r.project {
		partialRecord["scope"] = "project"
	}
	for _, field := range r.fields() {
		partialRecord[field] = body[field]
	}
	if r.project {
		partialRecord["project_id"], _ = exWireValue(ctx, plan.Attributes()["project_id"])
	}
	partial, err := r.state(ctx, plan, partialRecord)
	if err != nil {
		return types.Object{}, err
	}
	if !r.draft {
		field := "global_policy"
		if r.project {
			field = "project_policy"
		}
		record, ok := response[field].(map[string]any)
		if !ok {
			return partial, fmt.Errorf("retention write returned no owned policy")
		}
		response = record
	}
	actualRevision, err := identityNumber(response["revision"])
	if err != nil || actualRevision != revision+1 {
		return partial, fmt.Errorf("governance response changed concurrently or has an invalid revision; refresh before retrying")
	}
	state, err := r.state(ctx, plan, response)
	if err != nil {
		return partial, err
	}
	return state, nil
}
func (r *governanceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan types.Object
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	current, found, err := r.read(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Governance Revision", err.Error())
		return
	}
	var revision int64
	if found {
		revision, err = identityNumber(current["revision"])
		if err != nil || revision < 1 {
			resp.Diagnostics.AddError("Invalid Governance Revision", "Refresh must return the current revision before creation.")
			return
		}
	}
	state, err := r.write(ctx, plan, revision)
	if err != nil {
		if !state.IsNull() {
			resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
		}
		resp.Diagnostics.AddError("Error Configuring Governance", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
func (r *governanceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var old types.Object
	resp.Diagnostics.Append(req.State.Get(ctx, &old)...)
	if resp.Diagnostics.HasError() {
		return
	}
	record, found, err := r.read(ctx, old)
	if err != nil {
		if r.project && resourceNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error Reading Governance", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	state, err := r.state(ctx, old, record)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Governance Response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
func (r *governanceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, old types.Object
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &old)...)
	if resp.Diagnostics.HasError() {
		return
	}
	revision, ok := old.Attributes()["revision"].(types.Int64)
	if !ok || revision.IsUnknown() || revision.IsNull() || revision.ValueInt64() < 1 {
		resp.Diagnostics.AddError("Missing Governance Revision", "Import or refresh the current revision before updating.")
		return
	}
	state, err := r.write(ctx, plan, revision.ValueInt64())
	if err != nil {
		if !state.IsNull() {
			resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
		}
		resp.Diagnostics.AddError("Error Updating Governance", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
func (r *governanceResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}
func (r *governanceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !r.project {
		if req.ID != "global" {
			resp.Diagnostics.AddError("Invalid Import Identity", "Use global for this policy scope.")
			return
		}
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "global")...)
		return
	}
	fields, err := parseImportFields(req.ID, []string{"project"})
	if err != nil {
		resp.Diagnostics.AddError("Invalid Import Identity", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "project/"+strconv.FormatInt(fields["project"], 10))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), fields["project"])...)
}

var _ resource.ResourceWithImportState = (*governanceResource)(nil)
var _ resource.ResourceWithConfigure = (*governanceResource)(nil)
