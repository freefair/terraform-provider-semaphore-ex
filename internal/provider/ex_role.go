package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type exRoleModel struct {
	ID                 types.String `tfsdk:"id"`
	ProjectID          types.Int64  `tfsdk:"project_id"`
	Slug               types.String `tfsdk:"slug"`
	Name               types.String `tfsdk:"name"`
	ProjectPermissions types.Set    `tfsdk:"project_permissions"`
	GlobalPermissions  types.Set    `tfsdk:"global_permissions"`
	Revision           types.Int64  `tfsdk:"revision"`
}
type globalRoleModel struct {
	ID                 types.String `tfsdk:"id"`
	Slug               types.String `tfsdk:"slug"`
	Name               types.String `tfsdk:"name"`
	ProjectPermissions types.Set    `tfsdk:"project_permissions"`
	GlobalPermissions  types.Set    `tfsdk:"global_permissions"`
	Revision           types.Int64  `tfsdk:"revision"`
}
type projectRoleModel struct {
	ID                 types.String `tfsdk:"id"`
	ProjectID          types.Int64  `tfsdk:"project_id"`
	Slug               types.String `tfsdk:"slug"`
	Name               types.String `tfsdk:"name"`
	ProjectPermissions types.Set    `tfsdk:"project_permissions"`
	Revision           types.Int64  `tfsdk:"revision"`
}

func exFromGlobal(m globalRoleModel) exRoleModel {
	return exRoleModel{ID: m.ID, Slug: m.Slug, Name: m.Name, ProjectPermissions: m.ProjectPermissions, GlobalPermissions: m.GlobalPermissions, Revision: m.Revision}
}
func exToGlobal(m exRoleModel) globalRoleModel {
	return globalRoleModel{ID: m.ID, Slug: m.Slug, Name: m.Name, ProjectPermissions: m.ProjectPermissions, GlobalPermissions: m.GlobalPermissions, Revision: m.Revision}
}
func exFromProject(m projectRoleModel) exRoleModel {
	return exRoleModel{ID: m.ID, ProjectID: m.ProjectID, Slug: m.Slug, Name: m.Name, ProjectPermissions: m.ProjectPermissions, Revision: m.Revision}
}
func exToProject(m exRoleModel) projectRoleModel {
	return projectRoleModel{ID: m.ID, ProjectID: m.ProjectID, Slug: m.Slug, Name: m.Name, ProjectPermissions: m.ProjectPermissions, Revision: m.Revision}
}

type exRoleResponse struct {
	ID                string `json:"id"`
	Slug              string `json:"slug"`
	Name              string `json:"name"`
	Permissions       int64  `json:"permissions"`
	GlobalPermissions int64  `json:"global_permissions"`
	Revision          int64  `json:"revision"`
}
type exPermissionDefinition struct {
	ID         string `json:"id"`
	Permission int64  `json:"permission"`
}

type exRoleResource struct {
	client  *apiclient.SemaphoreUI
	project bool
}
type exRoleDataSource struct {
	client  *apiclient.SemaphoreUI
	project bool
}

func NewGlobalRoleResource() resource.Resource        { return &exRoleResource{} }
func NewProjectRoleResource() resource.Resource       { return &exRoleResource{project: true} }
func NewGlobalRoleDataSource() datasource.DataSource  { return &exRoleDataSource{} }
func NewProjectRoleDataSource() datasource.DataSource { return &exRoleDataSource{project: true} }

func (r *exRoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	if r.project {
		resp.TypeName = req.ProviderTypeName + "_project_role"
	} else {
		resp.TypeName = req.ProviderTypeName + "_global_role"
	}
}
func (r *exRoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", "Expected *client.SemaphoreUI.")
		return
	}
	r.client = c
}
func (r *exRoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = exRoleResourceSchema(r.project)
}
func (d *exRoleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	if d.project {
		resp.TypeName = req.ProviderTypeName + "_project_role"
	} else {
		resp.TypeName = req.ProviderTypeName + "_global_role"
	}
}
func (d *exRoleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", "Expected *client.SemaphoreUI.")
		return
	}
	d.client = c
}
func (d *exRoleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = exRoleDataSourceSchema(d.project)
}

func exRoleResourceSchema(project bool) resourceschema.Schema {
	a := map[string]resourceschema.Attribute{
		"id": resourceschema.StringAttribute{MarkdownDescription: "Server-generated opaque role ID.", Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}, "slug": resourceschema.StringAttribute{MarkdownDescription: "Server-generated role slug.", Computed: true}, "name": resourceschema.StringAttribute{MarkdownDescription: "Role display name.", Required: true}, "project_permissions": resourceschema.SetAttribute{MarkdownDescription: "Named project permissions from the server catalog.", Required: true, ElementType: types.StringType}, "revision": resourceschema.Int64Attribute{MarkdownDescription: "Server revision for optimistic concurrency control.", Computed: true},
	}
	if project {
		a["project_id"] = resourceschema.Int64Attribute{MarkdownDescription: "Project owning this role.", Required: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}}
	} else {
		a["global_permissions"] = resourceschema.SetAttribute{MarkdownDescription: "Named global permissions from the server catalog.", Required: true, ElementType: types.StringType}
	}
	return resourceschema.Schema{MarkdownDescription: "Manages a custom Semaphore EX role.", Attributes: a}
}
func exRoleDataSourceSchema(project bool) datasourceschema.Schema {
	a := map[string]datasourceschema.Attribute{"id": datasourceschema.StringAttribute{Required: true}, "slug": datasourceschema.StringAttribute{Computed: true}, "name": datasourceschema.StringAttribute{Computed: true}, "project_permissions": datasourceschema.SetAttribute{Computed: true, ElementType: types.StringType}, "revision": datasourceschema.Int64Attribute{Computed: true}}
	if project {
		a["project_id"] = datasourceschema.Int64Attribute{Required: true}
	} else {
		a["global_permissions"] = datasourceschema.SetAttribute{Computed: true, ElementType: types.StringType}
	}
	return datasourceschema.Schema{MarkdownDescription: "Reads a custom Semaphore EX role.", Attributes: a}
}

func (r *exRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan exRoleModel
	if r.project {
		var m projectRoleModel
		resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
		plan = exFromProject(m)
	} else {
		var m globalRoleModel
		resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
		plan = exFromGlobal(m)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	state, err := exCreateRole(ctx, r.client, r.project, plan)
	if err != nil {
		resp.Diagnostics.AddError("Error Creating Semaphore EX Role", err.Error())
		return
	}
	if r.project {
		m := exToProject(state)
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	} else {
		m := exToGlobal(state)
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	}
}
func (r *exRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state exRoleModel
	if r.project {
		var m projectRoleModel
		resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
		state = exFromProject(m)
	} else {
		var m globalRoleModel
		resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
		state = exFromGlobal(m)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	next, err := exReadRole(ctx, r.client, r.project, state)
	if exNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Semaphore EX Role", err.Error())
		return
	}
	if r.project {
		m := exToProject(next)
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	} else {
		m := exToGlobal(next)
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	}
}
func (r *exRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state exRoleModel
	if r.project {
		var p, s projectRoleModel
		resp.Diagnostics.Append(req.Plan.Get(ctx, &p)...)
		resp.Diagnostics.Append(req.State.Get(ctx, &s)...)
		plan = exFromProject(p)
		state = exFromProject(s)
	} else {
		var p, s globalRoleModel
		resp.Diagnostics.Append(req.Plan.Get(ctx, &p)...)
		resp.Diagnostics.Append(req.State.Get(ctx, &s)...)
		plan = exFromGlobal(p)
		state = exFromGlobal(s)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	if state.Revision.IsNull() || state.Revision.IsUnknown() || state.Revision.ValueInt64() <= 0 {
		resp.Diagnostics.AddError("Role Revision Unavailable", "Refresh the role and retry; the provider will not overwrite a concurrent role change.")
		return
	}
	plan.ID = state.ID
	plan.Slug = state.Slug
	plan.Revision = state.Revision
	next, err := exPutRole(ctx, r.client, r.project, plan)
	if err != nil {
		resp.Diagnostics.AddError("Error Updating Semaphore EX Role", err.Error())
		return
	}
	if r.project {
		m := exToProject(next)
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	} else {
		m := exToGlobal(next)
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	}
}
func (r *exRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state exRoleModel
	if r.project {
		var m projectRoleModel
		resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
		state = exFromProject(m)
	} else {
		var m globalRoleModel
		resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
		state = exFromGlobal(m)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	if state.Revision.IsNull() || state.Revision.IsUnknown() || state.Revision.ValueInt64() <= 0 {
		resp.Diagnostics.AddError("Role Revision Unavailable", "Refresh the role and retry; the provider will not delete without its prior revision.")
		return
	}
	err := exRequestWithOptions(ctx, r.client, http.MethodDelete, exRoleRoute(r.project), exRequestOptions{PathParams: exRoleParams(r.project, state), Query: map[string]string{"revision": strconv.FormatInt(state.Revision.ValueInt64(), 10)}}, nil, nil)
	if err != nil && !exNotFound(err) {
		resp.Diagnostics.AddError("Error Removing Semaphore EX Role", err.Error())
	}
}
func (r *exRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	state, err := exRoleImport(r.project, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Role Import ID", err.Error())
		return
	}
	next, err := exReadRole(ctx, r.client, r.project, state)
	if err != nil {
		resp.Diagnostics.AddError("Error Importing Semaphore EX Role", err.Error())
		return
	}
	if r.project {
		m := exToProject(next)
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	} else {
		m := exToGlobal(next)
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	}
}
func (d *exRoleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config exRoleModel
	if d.project {
		var m projectRoleModel
		resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
		config = exFromProject(m)
	} else {
		var m globalRoleModel
		resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
		config = exFromGlobal(m)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	state, err := exReadRole(ctx, d.client, d.project, config)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Semaphore EX Role", err.Error())
		return
	}
	if d.project {
		m := exToProject(state)
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	} else {
		m := exToGlobal(state)
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	}
}

func exRoleRoute(project bool) string {
	if project {
		return "/project/{project_id}/roles/{role_id}"
	}
	return "/roles/{role_id}"
}
func exRoleParams(project bool, m exRoleModel) map[string]string {
	p := map[string]string{"role_id": m.ID.ValueString()}
	if project {
		p["project_id"] = strconv.FormatInt(m.ProjectID.ValueInt64(), 10)
	}
	return p
}
func exRoleCatalogRoute(project bool, globalPermissions bool) string {
	if project {
		return "/project/{project_id}/roles/permissions"
	}
	if !globalPermissions {
		return "/roles/project-permissions"
	}
	return "/roles/permissions"
}
func exRoleCollectionRoute(project bool) string {
	if project {
		return "/project/{project_id}/roles"
	}
	return "/roles"
}
func exRoleCatalog(ctx context.Context, c *apiclient.SemaphoreUI, project bool, globalPermissions bool, m exRoleModel) (map[string]int64, error) {
	var defs []exPermissionDefinition
	o := exRequestOptions{}
	if project {
		o.PathParams = map[string]string{"project_id": strconv.FormatInt(m.ProjectID.ValueInt64(), 10)}
	}
	if err := exRequestWithOptions(ctx, c, http.MethodGet, exRoleCatalogRoute(project, globalPermissions), o, nil, &defs); err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, d := range defs {
		out[d.ID] = d.Permission
	}
	return out, nil
}
func exRoleMask(ctx context.Context, s types.Set, catalog map[string]int64) (int64, error) {
	var ids []string
	if diags := s.ElementsAs(ctx, &ids, false); diags.HasError() {
		return 0, fmt.Errorf("could not read configured role permissions")
	}
	var mask int64
	for _, id := range ids {
		v, ok := catalog[id]
		if !ok {
			return 0, fmt.Errorf("permission %q is not present in the server permission catalog", id)
		}
		mask |= v
	}
	return mask, nil
}
func exRoleSet(ctx context.Context, mask int64, catalog map[string]int64) (types.Set, error) {
	ids := []string{}
	known := int64(0)
	for id, v := range catalog {
		known |= v
		if mask&v == v {
			ids = append(ids, id)
		}
	}
	if mask&^known != 0 {
		return types.SetNull(types.StringType), fmt.Errorf("server role contains permissions absent from the current permission catalog")
	}
	set, diags := types.SetValueFrom(ctx, types.StringType, ids)
	if diags.HasError() {
		return types.SetNull(types.StringType), fmt.Errorf("could not store role permissions")
	}
	return set, nil
}
func exRoleModelFromResponse(ctx context.Context, c *apiclient.SemaphoreUI, project bool, old exRoleModel, raw exRoleResponse) (exRoleModel, error) {
	projectCatalog, err := exRoleCatalog(ctx, c, project, false, old)
	if err != nil {
		return exRoleModel{}, err
	}
	pp, err := exRoleSet(ctx, raw.Permissions, projectCatalog)
	if err != nil {
		return exRoleModel{}, err
	}
	m := exRoleModel{ID: types.StringValue(raw.ID), ProjectID: old.ProjectID, Slug: types.StringValue(raw.Slug), Name: types.StringValue(raw.Name), ProjectPermissions: pp, Revision: types.Int64Value(raw.Revision)}
	if !project {
		globalCatalog, err := exRoleCatalog(ctx, c, false, true, old)
		if err != nil {
			return exRoleModel{}, err
		}
		gp, err := exRoleSet(ctx, raw.GlobalPermissions, globalCatalog)
		if err != nil {
			return exRoleModel{}, err
		}
		m.GlobalPermissions = gp
	}
	return m, nil
}
func exCreateRole(ctx context.Context, c *apiclient.SemaphoreUI, project bool, m exRoleModel) (exRoleModel, error) {
	projectCatalog, err := exRoleCatalog(ctx, c, project, false, m)
	if err != nil {
		return exRoleModel{}, err
	}
	pp, err := exRoleMask(ctx, m.ProjectPermissions, projectCatalog)
	if err != nil {
		return exRoleModel{}, err
	}
	body := map[string]any{"name": m.Name.ValueString(), "permissions": pp}
	if !project {
		globalCatalog, e := exRoleCatalog(ctx, c, false, true, m)
		if e != nil {
			return exRoleModel{}, e
		}
		gp, e := exRoleMask(ctx, m.GlobalPermissions, globalCatalog)
		if e != nil {
			return exRoleModel{}, e
		}
		body["global_permissions"] = gp
	}
	var raw exRoleResponse
	o := exRequestOptions{}
	if project {
		o.PathParams = map[string]string{"project_id": strconv.FormatInt(m.ProjectID.ValueInt64(), 10)}
	}
	if err := exRequestWithOptions(ctx, c, http.MethodPost, exRoleCollectionRoute(project), o, body, &raw); err != nil {
		return exRoleModel{}, err
	}
	return exRoleModelFromResponse(ctx, c, project, m, raw)
}
func exPutRole(ctx context.Context, c *apiclient.SemaphoreUI, project bool, m exRoleModel) (exRoleModel, error) {
	projectCatalog, err := exRoleCatalog(ctx, c, project, false, m)
	if err != nil {
		return exRoleModel{}, err
	}
	pp, err := exRoleMask(ctx, m.ProjectPermissions, projectCatalog)
	if err != nil {
		return exRoleModel{}, err
	}
	body := map[string]any{"id": m.ID.ValueString(), "slug": m.Slug.ValueString(), "name": m.Name.ValueString(), "permissions": pp, "revision": m.Revision.ValueInt64()}
	if !project {
		globalCatalog, e := exRoleCatalog(ctx, c, false, true, m)
		if e != nil {
			return exRoleModel{}, e
		}
		gp, e := exRoleMask(ctx, m.GlobalPermissions, globalCatalog)
		if e != nil {
			return exRoleModel{}, e
		}
		body["global_permissions"] = gp
	}
	var raw exRoleResponse
	if err := exRequestWithOptions(ctx, c, http.MethodPut, exRoleRoute(project), exRequestOptions{PathParams: exRoleParams(project, m)}, body, &raw); err != nil {
		return exRoleModel{}, err
	}
	return exRoleModelFromResponse(ctx, c, project, m, raw)
}
func exReadRole(ctx context.Context, c *apiclient.SemaphoreUI, project bool, m exRoleModel) (exRoleModel, error) {
	var raw exRoleResponse
	if err := exRequestWithOptions(ctx, c, http.MethodGet, exRoleRoute(project), exRequestOptions{PathParams: exRoleParams(project, m)}, nil, &raw); err != nil {
		return exRoleModel{}, err
	}
	return exRoleModelFromResponse(ctx, c, project, m, raw)
}
func exRoleImport(project bool, id string) (exRoleModel, error) {
	if !project {
		if strings.TrimSpace(id) == "" {
			return exRoleModel{}, fmt.Errorf("global role import ID must be the opaque role ID")
		}
		return exRoleModel{ID: types.StringValue(id)}, nil
	}
	parts := strings.Split(id, "/")
	if len(parts) != 4 || parts[0] != "project" || parts[2] != "role" {
		return exRoleModel{}, fmt.Errorf("project role import ID must be project/<project_id>/role/<role_id>")
	}
	pid, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || pid <= 0 || parts[3] == "" {
		return exRoleModel{}, fmt.Errorf("project role import ID must contain a positive project ID and opaque role ID")
	}
	return exRoleModel{ID: types.StringValue(parts[3]), ProjectID: types.Int64Value(pid)}, nil
}
