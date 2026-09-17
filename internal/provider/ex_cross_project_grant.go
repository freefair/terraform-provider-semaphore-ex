package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rs "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// exCrossProjectGrantModel deliberately stores only identifiers and immutable
// template metadata. Grant operations never expose owner-side credentials.
type exCrossProjectGrantModel struct {
	ID                types.Int64  `tfsdk:"id"`
	OwnerProjectID    types.Int64  `tfsdk:"owner_project_id"`
	ConsumerProjectID types.Int64  `tfsdk:"consumer_project_id"`
	TemplateID        types.Int64  `tfsdk:"template_id"`
	MinVersion        types.Int64  `tfsdk:"min_version"`
	MaxVersion        types.Int64  `tfsdk:"max_version"`
	Operations        types.Set    `tfsdk:"operations"`
	Reason            types.String `tfsdk:"reason"`
	Accepted          types.Bool   `tfsdk:"accepted"`
	Status            types.String `tfsdk:"status"`
	Revision          types.Int64  `tfsdk:"revision"`
	Created           types.String `tfsdk:"created"`
}

type exCrossProjectGrantResponse struct {
	ID                int64  `json:"id"`
	OwnerProjectID    int64  `json:"owner_project_id"`
	ConsumerProjectID int64  `json:"consumer_project_id"`
	TemplateID        int64  `json:"template_id"`
	MinVersion        int64  `json:"min_version"`
	MaxVersion        int64  `json:"max_version"`
	Operations        int64  `json:"operations"`
	Status            string `json:"status"`
	Revision          int64  `json:"revision"`
	Reason            string `json:"reason"`
	Created           string `json:"created"`
}

type exCrossProjectGrantResource struct{ client *apiclient.SemaphoreUI }
type exCrossProjectGrantDataSource struct{ client *apiclient.SemaphoreUI }

var _ resource.ResourceWithImportState = &exCrossProjectGrantResource{}

// NewCrossProjectGrantResource and NewCrossProjectGrantDataSource are kept
// separate so provider registration can be integrated independently.
func NewCrossProjectGrantResource() resource.Resource       { return &exCrossProjectGrantResource{} }
func NewCrossProjectGrantDataSource() datasource.DataSource { return &exCrossProjectGrantDataSource{} }

func (r *exCrossProjectGrantResource) Metadata(_ context.Context, q resource.MetadataRequest, p *resource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_cross_project_grant"
}
func (d *exCrossProjectGrantDataSource) Metadata(_ context.Context, q datasource.MetadataRequest, p *datasource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_cross_project_grant"
}
func (r *exCrossProjectGrantResource) Configure(_ context.Context, q resource.ConfigureRequest, p *resource.ConfigureResponse) {
	if q.ProviderData == nil {
		return
	}
	c, ok := q.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		p.Diagnostics.AddError("Unexpected Resource Configure Type", "Expected *client.SemaphoreUI.")
		return
	}
	r.client = c
}
func (d *exCrossProjectGrantDataSource) Configure(_ context.Context, q datasource.ConfigureRequest, p *datasource.ConfigureResponse) {
	if q.ProviderData == nil {
		return
	}
	c, ok := q.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		p.Diagnostics.AddError("Unexpected Data Source Configure Type", "Expected *client.SemaphoreUI.")
		return
	}
	d.client = c
}

func exCrossProjectGrantResourceSchema() rs.Schema {
	replaceID := []planmodifier.Int64{int64planmodifier.RequiresReplace()}
	return rs.Schema{MarkdownDescription: "Manages an owner-created Semaphore EX cross-project immutable template grant. `accepted = true` explicitly asks the configured identity to accept on behalf of the consumer project; it is never accepted implicitly.", Attributes: map[string]rs.Attribute{
		"id":                  rs.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
		"owner_project_id":    rs.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}, PlanModifiers: replaceID},
		"consumer_project_id": rs.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}, PlanModifiers: replaceID},
		"template_id":         rs.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}, PlanModifiers: replaceID},
		// The API permits editing only pending grants. Replacing grant shape keeps
		// a later acceptance from turning an ordinary Terraform update into 409.
		"min_version": rs.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}, PlanModifiers: replaceID},
		"max_version": rs.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}, PlanModifiers: replaceID},
		"operations":  rs.SetAttribute{Required: true, ElementType: types.StringType, PlanModifiers: []planmodifier.Set{setplanmodifier.RequiresReplace()}},
		"reason":      rs.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"accepted":    rs.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}},
		"status":      rs.StringAttribute{Computed: true},
		"revision":    rs.Int64Attribute{Computed: true},
		"created":     rs.StringAttribute{Computed: true},
	}}
}

func (r *exCrossProjectGrantResource) Schema(_ context.Context, _ resource.SchemaRequest, p *resource.SchemaResponse) {
	p.Schema = exCrossProjectGrantResourceSchema()
}
func (d *exCrossProjectGrantDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, p *datasource.SchemaResponse) {
	p.Schema = ds.Schema{MarkdownDescription: "Reads a cross-project immutable template grant visible to the configured project administrator.", Attributes: map[string]ds.Attribute{
		"id": ds.Int64Attribute{Required: true}, "owner_project_id": ds.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}},
		"consumer_project_id": ds.Int64Attribute{Computed: true}, "template_id": ds.Int64Attribute{Computed: true},
		"min_version": ds.Int64Attribute{Computed: true}, "max_version": ds.Int64Attribute{Computed: true}, "operations": ds.SetAttribute{Computed: true, ElementType: types.StringType},
		"reason": ds.StringAttribute{Computed: true}, "accepted": ds.BoolAttribute{Computed: true}, "status": ds.StringAttribute{Computed: true}, "revision": ds.Int64Attribute{Computed: true}, "created": ds.StringAttribute{Computed: true},
	}}
}

func exCrossProjectGrantOperations(ctx context.Context, values types.Set) (int64, error) {
	var names []string
	if diagnostics := values.ElementsAs(ctx, &names, false); diagnostics.HasError() {
		return 0, fmt.Errorf("could not read configured grant operations")
	}
	var operations int64
	for _, name := range names {
		switch name {
		case "reference":
			operations |= 1
		case "run":
			operations |= 2
		default:
			return 0, fmt.Errorf("grant operation %q is not supported; use reference or run", name)
		}
	}
	if operations == 0 {
		return 0, fmt.Errorf("at least one grant operation is required")
	}
	return operations, nil
}
func exCrossProjectGrantOperationSet(ctx context.Context, operations int64) (types.Set, error) {
	names := []string{}
	if operations&1 != 0 {
		names = append(names, "reference")
	}
	if operations&2 != 0 {
		names = append(names, "run")
	}
	if operations&^int64(3) != 0 || len(names) == 0 {
		return types.SetNull(types.StringType), fmt.Errorf("server grant contains unsupported operations")
	}
	value, diagnostics := types.SetValueFrom(ctx, types.StringType, names)
	if diagnostics.HasError() {
		return types.SetNull(types.StringType), fmt.Errorf("could not store grant operations")
	}
	return value, nil
}
func exCrossProjectGrantState(ctx context.Context, old exCrossProjectGrantModel, raw exCrossProjectGrantResponse) (exCrossProjectGrantModel, error) {
	operations, err := exCrossProjectGrantOperationSet(ctx, raw.Operations)
	if err != nil {
		return old, err
	}
	if raw.ID <= 0 || raw.OwnerProjectID <= 0 || raw.ConsumerProjectID <= 0 || raw.TemplateID <= 0 || raw.MinVersion <= 0 || raw.MaxVersion < raw.MinVersion || raw.Revision <= 0 {
		return old, fmt.Errorf("server returned an invalid cross-project grant")
	}
	if raw.Status != "pending" && raw.Status != "active" && raw.Status != "revoked" {
		return old, fmt.Errorf("server returned unsupported cross-project grant status %q", raw.Status)
	}
	next := old
	next.ID, next.OwnerProjectID, next.ConsumerProjectID, next.TemplateID = types.Int64Value(raw.ID), types.Int64Value(raw.OwnerProjectID), types.Int64Value(raw.ConsumerProjectID), types.Int64Value(raw.TemplateID)
	next.MinVersion, next.MaxVersion, next.Operations = types.Int64Value(raw.MinVersion), types.Int64Value(raw.MaxVersion), operations
	next.Reason, next.Status, next.Revision, next.Created = types.StringValue(raw.Reason), types.StringValue(raw.Status), types.Int64Value(raw.Revision), types.StringValue(raw.Created)
	next.Accepted = types.BoolValue(raw.Status == "active")
	return next, nil
}
func exCrossProjectGrantParams(m exCrossProjectGrantModel) map[string]string {
	return map[string]string{"project_id": strconv.FormatInt(m.OwnerProjectID.ValueInt64(), 10), "grant_id": strconv.FormatInt(m.ID.ValueInt64(), 10)}
}
func exCrossProjectGrantConsumerParams(m exCrossProjectGrantModel) map[string]string {
	return map[string]string{"project_id": strconv.FormatInt(m.ConsumerProjectID.ValueInt64(), 10), "grant_id": strconv.FormatInt(m.ID.ValueInt64(), 10)}
}
func exCrossProjectGrantRequireRevision(m exCrossProjectGrantModel) error {
	if m.Revision.IsNull() || m.Revision.IsUnknown() || m.Revision.ValueInt64() <= 0 {
		return fmt.Errorf("refresh the cross-project grant and retry; the provider will not overwrite a concurrent grant change")
	}
	return nil
}
func exCrossProjectGrantPayload(ctx context.Context, m exCrossProjectGrantModel) (map[string]any, error) {
	operations, err := exCrossProjectGrantOperations(ctx, m.Operations)
	if err != nil {
		return nil, err
	}
	if m.MinVersion.ValueInt64() <= 0 || m.MaxVersion.ValueInt64() < m.MinVersion.ValueInt64() {
		return nil, fmt.Errorf("min_version must be positive and no greater than max_version")
	}
	if strings.TrimSpace(m.Reason.ValueString()) == "" {
		return nil, fmt.Errorf("reason must not be empty")
	}
	return map[string]any{"consumer_project_id": m.ConsumerProjectID.ValueInt64(), "min_version": m.MinVersion.ValueInt64(), "max_version": m.MaxVersion.ValueInt64(), "operations": operations, "reason": m.Reason.ValueString()}, nil
}
func exCrossProjectGrantRead(ctx context.Context, client *apiclient.SemaphoreUI, projectID, grantID int64, old exCrossProjectGrantModel) (exCrossProjectGrantModel, error) {
	var before int64
	for {
		query := map[string]string{"count": "100"}
		if before > 0 {
			query["before"] = strconv.FormatInt(before, 10)
		}
		var grants []exCrossProjectGrantResponse
		params := map[string]string{"project_id": strconv.FormatInt(projectID, 10)}
		if err := exRequestWithOptions(ctx, client, http.MethodGet, "/project/{project_id}/cross-project-template-grants", exRequestOptions{PathParams: params, Query: query}, nil, &grants); err != nil {
			return old, err
		}
		if len(grants) == 0 {
			return old, &exAPIError{StatusCode: http.StatusNotFound, method: http.MethodGet, route: "/project/{project_id}/cross-project-template-grants"}
		}
		for _, grant := range grants {
			if grant.ID == grantID {
				return exCrossProjectGrantState(ctx, old, grant)
			}
		}
		nextBefore := grants[len(grants)-1].ID
		if nextBefore <= 0 || nextBefore >= before && before > 0 {
			return old, fmt.Errorf("cross-project grant pagination did not advance")
		}
		before = nextBefore
	}
}
func (r *exCrossProjectGrantResource) transition(ctx context.Context, state exCrossProjectGrantModel, action string) (exCrossProjectGrantModel, error) {
	params := exCrossProjectGrantParams(state)
	if action == "accept" {
		params = exCrossProjectGrantConsumerParams(state)
	}
	var raw exCrossProjectGrantResponse
	if err := exRequest(ctx, r.client, http.MethodPost, "/project/{project_id}/cross-project-template-grants/{grant_id}/"+action, params, map[string]any{"expected_revision": state.Revision.ValueInt64()}, &raw); err != nil {
		return state, err
	}
	return exCrossProjectGrantState(ctx, state, raw)
}
func (r *exCrossProjectGrantResource) revoke(ctx context.Context, state exCrossProjectGrantModel) (exCrossProjectGrantModel, error) {
	var raw exCrossProjectGrantResponse
	if err := exRequest(ctx, r.client, http.MethodPost, "/project/{project_id}/cross-project-template-grants/{grant_id}/revoke", exCrossProjectGrantParams(state), map[string]any{"expected_revision": state.Revision.ValueInt64(), "reason": "Terraform resource removal"}, &raw); err != nil {
		return state, err
	}
	return exCrossProjectGrantState(ctx, state, raw)
}

// verifyConsumerProjectAccess avoids creating a pending owner-side grant that
// cannot be accepted by this provider identity. The server still authorizes
// the accept transition, which closes the permission-change race.
func (r *exCrossProjectGrantResource) verifyConsumerProjectAccess(ctx context.Context, consumerProjectID int64) error {
	var grants []exCrossProjectGrantResponse
	return exRequestWithOptions(ctx, r.client, http.MethodGet, "/project/{project_id}/cross-project-template-grants", exRequestOptions{PathParams: map[string]string{"project_id": strconv.FormatInt(consumerProjectID, 10)}, Query: map[string]string{"count": "1"}}, nil, &grants)
}
func (r *exCrossProjectGrantResource) Create(ctx context.Context, q resource.CreateRequest, p *resource.CreateResponse) {
	var plan exCrossProjectGrantModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &plan)...)
	if p.Diagnostics.HasError() {
		return
	}
	body, err := exCrossProjectGrantPayload(ctx, plan)
	if err != nil {
		p.Diagnostics.AddError("Invalid Cross-Project Grant", err.Error())
		return
	}
	if !plan.Accepted.IsNull() && !plan.Accepted.IsUnknown() && plan.Accepted.ValueBool() {
		if err := r.verifyConsumerProjectAccess(ctx, plan.ConsumerProjectID.ValueInt64()); err != nil {
			p.Diagnostics.AddError("Consumer Project Permission Required", "The configured identity must manage the consumer project before accepted can be true: "+err.Error())
			return
		}
	}
	params := map[string]string{"project_id": strconv.FormatInt(plan.OwnerProjectID.ValueInt64(), 10), "template_id": strconv.FormatInt(plan.TemplateID.ValueInt64(), 10)}
	var raw exCrossProjectGrantResponse
	if err = exRequest(ctx, r.client, http.MethodPost, "/project/{project_id}/templates/{template_id}/cross-project-grants", params, body, &raw); err != nil {
		p.Diagnostics.AddError("Error Creating Semaphore EX Cross-Project Grant", err.Error())
		return
	}
	state, err := exCrossProjectGrantState(ctx, plan, raw)
	if err != nil {
		p.Diagnostics.AddError("Invalid Cross-Project Grant Response", err.Error())
		return
	}
	if !plan.Accepted.IsNull() && !plan.Accepted.IsUnknown() && plan.Accepted.ValueBool() {
		state, err = r.transition(ctx, state, "accept")
		if err != nil {
			p.Diagnostics.AddError("Error Accepting Semaphore EX Cross-Project Grant", "The configured identity must also manage the consumer project: "+err.Error())
			return
		}
	}
	p.Diagnostics.Append(p.State.Set(ctx, &state)...)
}
func (r *exCrossProjectGrantResource) Read(ctx context.Context, q resource.ReadRequest, p *resource.ReadResponse) {
	var state exCrossProjectGrantModel
	p.Diagnostics.Append(q.State.Get(ctx, &state)...)
	if p.Diagnostics.HasError() {
		return
	}
	next, err := exCrossProjectGrantRead(ctx, r.client, state.OwnerProjectID.ValueInt64(), state.ID.ValueInt64(), state)
	if exNotFound(err) {
		p.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		p.Diagnostics.AddError("Error Reading Semaphore EX Cross-Project Grant", err.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (r *exCrossProjectGrantResource) Update(ctx context.Context, q resource.UpdateRequest, p *resource.UpdateResponse) {
	var plan, state exCrossProjectGrantModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &plan)...)
	p.Diagnostics.Append(q.State.Get(ctx, &state)...)
	if p.Diagnostics.HasError() {
		return
	}
	if err := exCrossProjectGrantRequireRevision(state); err != nil {
		p.Diagnostics.AddError("Cross-Project Grant Revision Unavailable", err.Error())
		return
	}
	current := state
	if !plan.Accepted.IsNull() && !plan.Accepted.IsUnknown() && plan.Accepted.ValueBool() && state.Status.ValueString() == "pending" {
		var err error
		current, err = r.transition(ctx, current, "accept")
		if err != nil {
			p.Diagnostics.AddError("Error Accepting Semaphore EX Cross-Project Grant", "The configured identity must manage the consumer project: "+err.Error())
			return
		}
	} else if !plan.Accepted.IsNull() && !plan.Accepted.IsUnknown() && !plan.Accepted.ValueBool() && state.Status.ValueString() != "revoked" {
		var err error
		current, err = r.revoke(ctx, current)
		if err != nil {
			p.Diagnostics.AddError("Error Revoking Semaphore EX Cross-Project Grant", err.Error())
			return
		}
	} else if !plan.Accepted.IsNull() && !plan.Accepted.IsUnknown() && plan.Accepted.ValueBool() && state.Status.ValueString() == "revoked" {
		p.Diagnostics.AddError("Cross-Project Grant Cannot Be Reaccepted", "A revoked grant is terminal. Replace the resource to create a new pending grant.")
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &current)...)
}
func (r *exCrossProjectGrantResource) Delete(ctx context.Context, q resource.DeleteRequest, p *resource.DeleteResponse) {
	var state exCrossProjectGrantModel
	p.Diagnostics.Append(q.State.Get(ctx, &state)...)
	if p.Diagnostics.HasError() {
		return
	}
	if err := exCrossProjectGrantRequireRevision(state); err != nil {
		p.Diagnostics.AddError("Cross-Project Grant Revision Unavailable", err.Error())
		return
	}
	current := state
	if current.Status.ValueString() != "revoked" {
		var err error
		current, err = r.revoke(ctx, current)
		if err != nil && !exNotFound(err) {
			p.Diagnostics.AddError("Error Revoking Semaphore EX Cross-Project Grant Before Deletion", err.Error())
			return
		}
	}
	err := exRequestWithOptions(ctx, r.client, http.MethodDelete, "/project/{project_id}/cross-project-template-grants/{grant_id}", exRequestOptions{PathParams: exCrossProjectGrantParams(current), Query: map[string]string{"expected_revision": strconv.FormatInt(current.Revision.ValueInt64(), 10)}}, nil, nil)
	if err != nil && !exNotFound(err) {
		p.Diagnostics.AddError("Error Removing Semaphore EX Cross-Project Grant", err.Error())
	}
}
func (r *exCrossProjectGrantResource) ImportState(ctx context.Context, q resource.ImportStateRequest, p *resource.ImportStateResponse) {
	parts := strings.Split(q.ID, "/")
	if len(parts) != 4 || parts[0] != "project" || parts[2] != "grant" {
		p.Diagnostics.AddError("Invalid Cross-Project Grant Import ID", "Use project/<owner_project_id>/grant/<grant_id>.")
		return
	}
	projectID, projectErr := strconv.ParseInt(parts[1], 10, 64)
	grantID, grantErr := strconv.ParseInt(parts[3], 10, 64)
	if projectErr != nil || grantErr != nil || projectID <= 0 || grantID <= 0 {
		p.Diagnostics.AddError("Invalid Cross-Project Grant Import ID", "Use positive numeric owner project and grant IDs.")
		return
	}
	next, err := exCrossProjectGrantRead(ctx, r.client, projectID, grantID, exCrossProjectGrantModel{OwnerProjectID: types.Int64Value(projectID), ID: types.Int64Value(grantID)})
	if err != nil {
		p.Diagnostics.AddError("Error Importing Semaphore EX Cross-Project Grant", err.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (d *exCrossProjectGrantDataSource) Read(ctx context.Context, q datasource.ReadRequest, p *datasource.ReadResponse) {
	var config exCrossProjectGrantModel
	p.Diagnostics.Append(q.Config.Get(ctx, &config)...)
	if p.Diagnostics.HasError() {
		return
	}
	state, err := exCrossProjectGrantRead(ctx, d.client, config.OwnerProjectID.ValueInt64(), config.ID.ValueInt64(), config)
	if err != nil {
		p.Diagnostics.AddError("Error Reading Semaphore EX Cross-Project Grant", err.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &state)...)
}
