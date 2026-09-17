package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rs "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type exTemplateACLModel struct {
	ID                 types.Int64  `tfsdk:"id"`
	ProjectID          types.Int64  `tfsdk:"project_id"`
	TemplateID         types.Int64  `tfsdk:"template_id"`
	RoleSlug           types.String `tfsdk:"role_slug"`
	RoleID             types.String `tfsdk:"role_id"`
	AllowedPermissions types.Set    `tfsdk:"allowed_permissions"`
	DeniedPermissions  types.Set    `tfsdk:"denied_permissions"`
	Revision           types.Int64  `tfsdk:"revision"`
}
type exTemplateACLResponse struct {
	ID                 int64   `json:"id"`
	RoleSlug           string  `json:"role_slug"`
	RoleID             *string `json:"role_id"`
	TemplateID         int64   `json:"template_id"`
	ProjectID          int64   `json:"project_id"`
	AllowedPermissions int64   `json:"allowed_permissions"`
	DeniedPermissions  int64   `json:"denied_permissions"`
	Revision           int64   `json:"revision"`
}
type exTemplateACLResource struct{ client *apiclient.SemaphoreUI }
type exTemplateACLDataSource struct{ client *apiclient.SemaphoreUI }

var _ resource.ResourceWithImportState = &exTemplateACLResource{}

func NewTemplateACLResource() resource.Resource       { return &exTemplateACLResource{} }
func NewTemplateACLDataSource() datasource.DataSource { return &exTemplateACLDataSource{} }
func (r *exTemplateACLResource) Metadata(_ context.Context, q resource.MetadataRequest, p *resource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_template_acl"
}
func (d *exTemplateACLDataSource) Metadata(_ context.Context, q datasource.MetadataRequest, p *datasource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_template_acl"
}
func (r *exTemplateACLResource) Configure(_ context.Context, q resource.ConfigureRequest, p *resource.ConfigureResponse) {
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
func (d *exTemplateACLDataSource) Configure(_ context.Context, q datasource.ConfigureRequest, p *datasource.ConfigureResponse) {
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
func exTemplateACLSchema() map[string]rs.Attribute {
	return map[string]rs.Attribute{"id": rs.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}}, "project_id": rs.Int64Attribute{Required: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}}, "template_id": rs.Int64Attribute{Required: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}}, "role_slug": rs.StringAttribute{Optional: true, Computed: true}, "role_id": rs.StringAttribute{Optional: true, Computed: true}, "allowed_permissions": rs.SetAttribute{Required: true, ElementType: types.StringType}, "denied_permissions": rs.SetAttribute{Required: true, ElementType: types.StringType}, "revision": rs.Int64Attribute{Computed: true}}
}
func (r *exTemplateACLResource) Schema(_ context.Context, _ resource.SchemaRequest, p *resource.SchemaResponse) {
	p.Schema = rs.Schema{MarkdownDescription: "Manages a role-scoped Semaphore EX template ACL.", Attributes: exTemplateACLSchema()}
}
func (d *exTemplateACLDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, p *datasource.SchemaResponse) {
	p.Schema = ds.Schema{MarkdownDescription: "Reads a Semaphore EX template ACL.", Attributes: map[string]ds.Attribute{"id": ds.Int64Attribute{Required: true}, "project_id": ds.Int64Attribute{Required: true}, "template_id": ds.Int64Attribute{Required: true}, "role_slug": ds.StringAttribute{Computed: true}, "role_id": ds.StringAttribute{Computed: true}, "allowed_permissions": ds.SetAttribute{Computed: true, ElementType: types.StringType}, "denied_permissions": ds.SetAttribute{Computed: true, ElementType: types.StringType}, "revision": ds.Int64Attribute{Computed: true}}}
}
func aclParams(m exTemplateACLModel) map[string]string {
	return map[string]string{"project_id": strconv.FormatInt(m.ProjectID.ValueInt64(), 10), "template_id": strconv.FormatInt(m.TemplateID.ValueInt64(), 10), "perm_id": strconv.FormatInt(m.ID.ValueInt64(), 10)}
}
func aclCatalog(ctx context.Context, c *apiclient.SemaphoreUI, m exTemplateACLModel) (map[string]int64, error) {
	var defs []exPermissionDefinition
	err := exRequest(ctx, c, http.MethodGet, "/project/{project_id}/templates/{template_id}/perms/catalog", map[string]string{"project_id": strconv.FormatInt(m.ProjectID.ValueInt64(), 10), "template_id": strconv.FormatInt(m.TemplateID.ValueInt64(), 10)}, nil, &defs)
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, d := range defs {
		out[d.ID] = d.Permission
	}
	return out, nil
}
func aclMask(ctx context.Context, s types.Set, c map[string]int64) (int64, error) {
	return exRoleMask(ctx, s, c)
}
func aclSet(ctx context.Context, n int64, c map[string]int64) (types.Set, error) {
	return exRoleSet(ctx, n, c)
}
func aclState(ctx context.Context, c *apiclient.SemaphoreUI, old exTemplateACLModel, raw exTemplateACLResponse) (exTemplateACLModel, error) {
	catalog, err := aclCatalog(ctx, c, old)
	if err != nil {
		return old, err
	}
	a, err := aclSet(ctx, raw.AllowedPermissions, catalog)
	if err != nil {
		return old, err
	}
	d, err := aclSet(ctx, raw.DeniedPermissions, catalog)
	if err != nil {
		return old, err
	}
	next := old
	next.ID = types.Int64Value(raw.ID)
	next.ProjectID = types.Int64Value(raw.ProjectID)
	next.TemplateID = types.Int64Value(raw.TemplateID)
	next.RoleSlug = types.StringValue(raw.RoleSlug)
	if raw.RoleID == nil {
		next.RoleID = types.StringNull()
	} else {
		next.RoleID = types.StringValue(*raw.RoleID)
	}
	next.AllowedPermissions = a
	next.DeniedPermissions = d
	next.Revision = types.Int64Value(raw.Revision)
	return next, nil
}
func aclBody(ctx context.Context, m exTemplateACLModel, c map[string]int64, revision int64) (map[string]any, error) {
	if (m.RoleSlug.IsNull() || m.RoleSlug.IsUnknown() || m.RoleSlug.ValueString() == "") == (m.RoleID.IsNull() || m.RoleID.IsUnknown() || m.RoleID.ValueString() == "") {
		return nil, fmt.Errorf("set exactly one of role_slug or role_id")
	}
	a, e := aclMask(ctx, m.AllowedPermissions, c)
	if e != nil {
		return nil, e
	}
	d, e := aclMask(ctx, m.DeniedPermissions, c)
	if e != nil {
		return nil, e
	}
	b := map[string]any{"allowed_permissions": a, "denied_permissions": d}
	if !m.RoleSlug.IsNull() && !m.RoleSlug.IsUnknown() && m.RoleSlug.ValueString() != "" {
		b["role_slug"] = m.RoleSlug.ValueString()
	} else {
		b["role_id"] = m.RoleID.ValueString()
	}
	if revision > 0 {
		b["revision"] = revision
	}
	return b, nil
}
func aclRead(ctx context.Context, c *apiclient.SemaphoreUI, m exTemplateACLModel) (exTemplateACLModel, error) {
	var raw exTemplateACLResponse
	e := exRequest(ctx, c, http.MethodGet, "/project/{project_id}/templates/{template_id}/perms/{perm_id}", aclParams(m), nil, &raw)
	if e != nil {
		return m, e
	}
	return aclState(ctx, c, m, raw)
}
func (r *exTemplateACLResource) Create(ctx context.Context, q resource.CreateRequest, p *resource.CreateResponse) {
	var plan exTemplateACLModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &plan)...)
	if p.Diagnostics.HasError() {
		return
	}
	catalog, e := aclCatalog(ctx, r.client, plan)
	if e != nil {
		p.Diagnostics.AddError("Error Reading Template Permission Catalog", e.Error())
		return
	}
	body, e := aclBody(ctx, plan, catalog, 0)
	if e != nil {
		p.Diagnostics.AddError("Invalid Template ACL", e.Error())
		return
	}
	var raw exTemplateACLResponse
	e = exRequest(ctx, r.client, http.MethodPost, "/project/{project_id}/templates/{template_id}/perms", aclParams(plan), body, &raw)
	if e != nil {
		p.Diagnostics.AddError("Error Creating Template ACL", e.Error())
		return
	}
	state, e := aclState(ctx, r.client, plan, raw)
	if e != nil {
		p.Diagnostics.AddError("Invalid Template ACL Response", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &state)...)
}
func (r *exTemplateACLResource) Read(ctx context.Context, q resource.ReadRequest, p *resource.ReadResponse) {
	var state exTemplateACLModel
	p.Diagnostics.Append(q.State.Get(ctx, &state)...)
	if p.Diagnostics.HasError() {
		return
	}
	next, e := aclRead(ctx, r.client, state)
	if exNotFound(e) {
		p.State.RemoveResource(ctx)
		return
	}
	if e != nil {
		p.Diagnostics.AddError("Error Reading Template ACL", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (r *exTemplateACLResource) Update(ctx context.Context, q resource.UpdateRequest, p *resource.UpdateResponse) {
	var plan, state exTemplateACLModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &plan)...)
	p.Diagnostics.Append(q.State.Get(ctx, &state)...)
	if p.Diagnostics.HasError() {
		return
	}
	if state.Revision.IsNull() || state.Revision.IsUnknown() || state.Revision.ValueInt64() <= 0 {
		p.Diagnostics.AddError("Template ACL Revision Unavailable", "Refresh and retry; the provider will not overwrite a concurrent ACL change.")
		return
	}
	plan.ID = state.ID
	plan.ProjectID = state.ProjectID
	plan.TemplateID = state.TemplateID
	catalog, e := aclCatalog(ctx, r.client, plan)
	if e != nil {
		p.Diagnostics.AddError("Error Reading Template Permission Catalog", e.Error())
		return
	}
	body, e := aclBody(ctx, plan, catalog, state.Revision.ValueInt64())
	if e != nil {
		p.Diagnostics.AddError("Invalid Template ACL", e.Error())
		return
	}
	var raw exTemplateACLResponse
	e = exRequest(ctx, r.client, http.MethodPut, "/project/{project_id}/templates/{template_id}/perms/{perm_id}", aclParams(plan), body, &raw)
	if e != nil {
		p.Diagnostics.AddError("Error Updating Template ACL", e.Error())
		return
	}
	next, e := aclState(ctx, r.client, plan, raw)
	if e != nil {
		p.Diagnostics.AddError("Invalid Template ACL Response", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (r *exTemplateACLResource) Delete(ctx context.Context, q resource.DeleteRequest, p *resource.DeleteResponse) {
	var state exTemplateACLModel
	p.Diagnostics.Append(q.State.Get(ctx, &state)...)
	if p.Diagnostics.HasError() {
		return
	}
	if state.Revision.IsNull() || state.Revision.IsUnknown() || state.Revision.ValueInt64() <= 0 {
		p.Diagnostics.AddError("Template ACL Revision Unavailable", "Refresh and retry; the provider will not delete without its prior revision.")
		return
	}
	e := exRequestWithOptions(ctx, r.client, http.MethodDelete, "/project/{project_id}/templates/{template_id}/perms/{perm_id}", exRequestOptions{PathParams: aclParams(state), Query: map[string]string{"revision": strconv.FormatInt(state.Revision.ValueInt64(), 10)}}, nil, nil)
	if e != nil && !exNotFound(e) {
		p.Diagnostics.AddError("Error Removing Template ACL", e.Error())
	}
}
func (r *exTemplateACLResource) ImportState(ctx context.Context, q resource.ImportStateRequest, p *resource.ImportStateResponse) {
	x := strings.Split(q.ID, "/")
	if len(x) != 6 || x[0] != "project" || x[2] != "template" || x[4] != "acl" {
		p.Diagnostics.AddError("Invalid Template ACL Import ID", "Use project/<project_id>/template/<template_id>/acl/<acl_id>.")
		return
	}
	pid, e1 := strconv.ParseInt(x[1], 10, 64)
	tid, e2 := strconv.ParseInt(x[3], 10, 64)
	id, e3 := strconv.ParseInt(x[5], 10, 64)
	if e1 != nil || e2 != nil || e3 != nil || pid <= 0 || tid <= 0 || id <= 0 {
		p.Diagnostics.AddError("Invalid Template ACL Import ID", "Import IDs must be positive.")
		return
	}
	next, e := aclRead(ctx, r.client, exTemplateACLModel{ProjectID: types.Int64Value(pid), TemplateID: types.Int64Value(tid), ID: types.Int64Value(id)})
	if e != nil {
		p.Diagnostics.AddError("Error Importing Template ACL", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (d *exTemplateACLDataSource) Read(ctx context.Context, q datasource.ReadRequest, p *datasource.ReadResponse) {
	var m exTemplateACLModel
	p.Diagnostics.Append(q.Config.Get(ctx, &m)...)
	if p.Diagnostics.HasError() {
		return
	}
	next, e := aclRead(ctx, d.client, m)
	if e != nil {
		p.Diagnostics.AddError("Error Reading Template ACL", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
