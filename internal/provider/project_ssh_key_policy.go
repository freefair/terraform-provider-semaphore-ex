package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	schemaD "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	schemaR "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var sshKeyBindingType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"access_key_id": types.Int64Type,
	"hosts":         types.ListType{ElemType: types.StringType},
}}

type projectSSHKeyPolicyModel struct {
	ID             types.Int64 `tfsdk:"id"`
	ProjectID      types.Int64 `tfsdk:"project_id"`
	DefaultSSHKeys types.List  `tfsdk:"default_ssh_keys"`
	AlwaysSSHKeys  types.List  `tfsdk:"always_ssh_keys"`
}

type projectSSHKeyPolicyResource struct{ client *apiclient.SemaphoreUI }
type projectSSHKeyPolicyDataSource struct{ client *apiclient.SemaphoreUI }

func NewProjectSSHKeyPolicyResource() resource.Resource { return &projectSSHKeyPolicyResource{} }
func NewProjectSSHKeyPolicyDataSource() datasource.DataSource {
	return &projectSSHKeyPolicyDataSource{}
}

func (r *projectSSHKeyPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_ssh_key_policy"
}
func (d *projectSSHKeyPolicyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_ssh_key_policy"
}

func (r *projectSSHKeyPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", "Expected the configured Semaphore EX client.")
		return
	}
	r.client = client
}
func (d *projectSSHKeyPolicyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", "Expected the configured Semaphore EX client.")
		return
	}
	d.client = client
}

func sshKeyPolicyBindingsAttribute(description string) schemaR.ListNestedAttribute {
	return schemaR.ListNestedAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}, MarkdownDescription: description, NestedObject: schemaR.NestedAttributeObject{Attributes: map[string]schemaR.Attribute{
		"access_key_id": schemaR.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}},
		"hosts":         schemaR.ListAttribute{Optional: true, ElementType: types.StringType, MarkdownDescription: "Optional exact lowercase DNS or IP host mapping; server validation decides when host routing is required."},
	}}}
}

func sshKeyPolicyResourceSchema() schemaR.Schema {
	return schemaR.Schema{MarkdownDescription: "Singleton policy owning the project default and always SSH key binding selections. Removing it clears only those two project fields.", Attributes: map[string]schemaR.Attribute{
		"id":               schemaR.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
		"project_id":       schemaR.Int64Attribute{Required: true, MarkdownDescription: "Project owning this singleton policy.", Validators: []validator.Int64{int64validator.AtLeast(1)}, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}},
		"default_ssh_keys": sshKeyPolicyBindingsAttribute("Default selection inherited by templates and tasks that do not override it. An empty list explicitly selects none."),
		"always_ssh_keys":  sshKeyPolicyBindingsAttribute("Keys added to every effective template or task selection. An empty list explicitly selects none."),
	}}
}

func (r *projectSSHKeyPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = sshKeyPolicyResourceSchema()
}
func (d *projectSSHKeyPolicyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schemaD.Schema{MarkdownDescription: "Reads a project SSH key policy.", Attributes: map[string]schemaD.Attribute{
		"id":               schemaD.Int64Attribute{Computed: true},
		"project_id":       schemaD.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}},
		"default_ssh_keys": sshKeyPolicyDataSourceBindingsAttribute("Default SSH key selection."),
		"always_ssh_keys":  sshKeyPolicyDataSourceBindingsAttribute("Always-added SSH key selection."),
	}}
}

func sshKeyPolicyDataSourceBindingsAttribute(description string) schemaD.ListNestedAttribute {
	return schemaD.ListNestedAttribute{Computed: true, MarkdownDescription: description, NestedObject: schemaD.NestedAttributeObject{Attributes: map[string]schemaD.Attribute{
		"access_key_id": schemaD.Int64Attribute{Computed: true}, "hosts": schemaD.ListAttribute{Computed: true, ElementType: types.StringType},
	}}}
}

func (r *projectSSHKeyPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectSSHKeyPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, plan, &resp.State, &resp.Diagnostics, false)
}
func (r *projectSSHKeyPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan projectSSHKeyPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, plan, &resp.State, &resp.Diagnostics, false)
}
func (r *projectSSHKeyPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	r.read(ctx, req.State, &resp.State, &resp.Diagnostics)
}
func (r *projectSSHKeyPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectSSHKeyPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, state, nil, &resp.Diagnostics, true)
}

func (r *projectSSHKeyPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil || id < 1 {
		resp.Diagnostics.AddError("Invalid Project SSH Key Policy Import ID", "Use the positive project ID.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), id)...)
}

func (r *projectSSHKeyPolicyResource) read(ctx context.Context, source tfsdk.State, destination *tfsdk.State, diagnostics *diag.Diagnostics) {
	var state projectSSHKeyPolicyModel
	diagnostics.Append(source.Get(ctx, &state)...)
	if diagnostics.HasError() {
		return
	}
	project, err := projectSSHKeyPolicyRead(ctx, r.client, state.ProjectID.ValueInt64())
	if exNotFound(err) {
		destination.RemoveResource(ctx)
		return
	}
	if err != nil {
		diagnostics.AddError("Error Reading Project SSH Key Policy", err.Error())
		return
	}
	model, err := projectSSHKeyPolicyState(state.ProjectID, project)
	if err != nil {
		diagnostics.AddError("Invalid Project SSH Key Policy Response", err.Error())
		return
	}
	diagnostics.Append(destination.Set(ctx, &model)...)
}

func (r *projectSSHKeyPolicyResource) write(ctx context.Context, plan projectSSHKeyPolicyModel, destination *tfsdk.State, diagnostics *diag.Diagnostics, resetPolicy bool) {
	project, err := projectSSHKeyPolicyRead(ctx, r.client, plan.ProjectID.ValueInt64())
	if resetPolicy && exNotFound(err) {
		return
	}
	if err != nil {
		diagnostics.AddError("Error Reading Project SSH Key Policy", err.Error())
		return
	}
	if resetPolicy {
		project["default_ssh_keys"], project["always_ssh_keys"] = nil, nil
	} else {
		for name, bindings := range map[string]types.List{"default_ssh_keys": plan.DefaultSSHKeys, "always_ssh_keys": plan.AlwaysSSHKeys} {
			if bindings.IsNull() || bindings.IsUnknown() {
				continue
			}
			value, conversionErr := sshKeyBindingsToAPI(ctx, bindings)
			if conversionErr != nil {
				diagnostics.AddAttributeError(path.Root(name), "Invalid SSH Key Selection", conversionErr.Error())
				return
			}
			project[name] = value
		}
	}
	if err = exRequest(ctx, r.client, http.MethodPut, "/project/{project_id}", map[string]string{"project_id": strconv.FormatInt(plan.ProjectID.ValueInt64(), 10)}, project, nil); err != nil {
		diagnostics.AddError("Error Updating Project SSH Key Policy", err.Error())
		return
	}
	if destination == nil {
		return
	}
	project, err = projectSSHKeyPolicyRead(ctx, r.client, plan.ProjectID.ValueInt64())
	if err != nil {
		diagnostics.AddError("Error Reading Updated Project SSH Key Policy", err.Error())
		return
	}
	model, err := projectSSHKeyPolicyState(plan.ProjectID, project)
	if err != nil {
		diagnostics.AddError("Invalid Project SSH Key Policy Response", err.Error())
		return
	}
	diagnostics.Append(destination.Set(ctx, &model)...)
}

func (d *projectSSHKeyPolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config projectSSHKeyPolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	project, err := projectSSHKeyPolicyRead(ctx, d.client, config.ProjectID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Project SSH Key Policy", err.Error())
		return
	}
	model, err := projectSSHKeyPolicyState(config.ProjectID, project)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Project SSH Key Policy Response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func projectSSHKeyPolicyRead(ctx context.Context, client *apiclient.SemaphoreUI, projectID int64) (map[string]any, error) {
	var project map[string]any
	err := exRequest(ctx, client, http.MethodGet, "/project/{project_id}", map[string]string{"project_id": strconv.FormatInt(projectID, 10)}, nil, &project)
	return project, err
}
func projectSSHKeyPolicyState(projectID types.Int64, project map[string]any) (projectSSHKeyPolicyModel, error) {
	defaults, err := sshKeyBindingsFromAPI(project["default_ssh_keys"])
	if err != nil {
		return projectSSHKeyPolicyModel{}, err
	}
	always, err := sshKeyBindingsFromAPI(project["always_ssh_keys"])
	if err != nil {
		return projectSSHKeyPolicyModel{}, err
	}
	return projectSSHKeyPolicyModel{ID: projectID, ProjectID: projectID, DefaultSSHKeys: defaults, AlwaysSSHKeys: always}, nil
}

func sshKeyPolicySelectionFromAPI(raw any) (types.Object, error) {
	inherit := raw == nil
	bindings := []attr.Value{}
	if !inherit {
		items, ok := raw.([]any)
		if !ok {
			return types.Object{}, fmt.Errorf("API returned a non-array SSH key selection")
		}
		for _, item := range items {
			binding, ok := item.(map[string]any)
			if !ok {
				return types.Object{}, fmt.Errorf("API returned a non-object SSH key binding")
			}
			key, err := exTypedValue(context.Background(), types.Int64Type, binding["access_key_id"])
			if err != nil {
				return types.Object{}, err
			}
			hosts, err := exTypedValue(context.Background(), types.ListType{ElemType: types.StringType}, binding["hosts"])
			if err != nil {
				return types.Object{}, err
			}
			value, diagnostics := types.ObjectValue(sshKeyBindingType.AttrTypes, map[string]attr.Value{"access_key_id": key, "hosts": hosts})
			if diagnostics.HasError() {
				return types.Object{}, fmt.Errorf("API returned an invalid SSH key binding")
			}
			bindings = append(bindings, value)
		}
	}
	list, diagnostics := types.ListValue(sshKeyBindingType, bindings)
	if inherit {
		list = types.ListNull(sshKeyBindingType)
	}
	if diagnostics.HasError() {
		return types.Object{}, fmt.Errorf("API returned an invalid SSH key selection")
	}
	value, diagnostics := types.ObjectValue(map[string]attr.Type{"inherit": types.BoolType, "bindings": types.ListType{ElemType: sshKeyBindingType}}, map[string]attr.Value{"inherit": types.BoolValue(inherit), "bindings": list})
	if diagnostics.HasError() {
		return types.Object{}, fmt.Errorf("API returned an invalid SSH key selection")
	}
	return value, nil
}

func sshKeyBindingsFromAPI(raw any) (types.List, error) {
	if raw == nil {
		return types.ListNull(sshKeyBindingType), nil
	}
	items, ok := raw.([]any)
	if !ok {
		return types.List{}, fmt.Errorf("API returned a non-array SSH key selection")
	}
	bindings := make([]attr.Value, 0, len(items))
	for _, item := range items {
		binding, ok := item.(map[string]any)
		if !ok {
			return types.List{}, fmt.Errorf("API returned a non-object SSH key binding")
		}
		key, err := exTypedValue(context.Background(), types.Int64Type, binding["access_key_id"])
		if err != nil {
			return types.List{}, err
		}
		hosts, err := exTypedValue(context.Background(), types.ListType{ElemType: types.StringType}, binding["hosts"])
		if err != nil {
			return types.List{}, err
		}
		value, diagnostics := types.ObjectValue(sshKeyBindingType.AttrTypes, map[string]attr.Value{"access_key_id": key, "hosts": hosts})
		if diagnostics.HasError() {
			return types.List{}, fmt.Errorf("API returned an invalid SSH key binding")
		}
		bindings = append(bindings, value)
	}
	value, diagnostics := types.ListValue(sshKeyBindingType, bindings)
	if diagnostics.HasError() {
		return types.List{}, fmt.Errorf("API returned an invalid SSH key selection")
	}
	return value, nil
}

func sshKeyBindingsToAPI(ctx context.Context, bindings types.List) (any, error) {
	if bindings.IsNull() || bindings.IsUnknown() {
		return nil, fmt.Errorf("SSH key bindings must be known")
	}
	items := make([]any, 0, len(bindings.Elements()))
	for _, item := range bindings.Elements() {
		object, ok := item.(types.Object)
		if !ok || object.IsNull() || object.IsUnknown() {
			return nil, fmt.Errorf("SSH key binding must be a known object")
		}
		binding := object.Attributes()
		key, err := exWireValue(ctx, binding["access_key_id"])
		if err != nil {
			return nil, err
		}
		wire := map[string]any{"access_key_id": key, "hosts": nil}
		hosts := binding["hosts"]
		if !hosts.IsNull() && !hosts.IsUnknown() {
			value, hostErr := exWireValue(ctx, hosts)
			if hostErr != nil {
				return nil, hostErr
			}
			wire["hosts"] = value
		}
		items = append(items, wire)
	}
	return items, nil
}

func sshKeyPolicySelectionToAPI(ctx context.Context, selection types.Object) (any, error) {
	if selection.IsNull() {
		return nil, fmt.Errorf("selection must be known")
	}
	values := selection.Attributes()
	inherit, inheritOK := values["inherit"].(types.Bool)
	bindings, bindingsOK := values["bindings"].(types.List)
	if !bindingsOK {
		return nil, fmt.Errorf("bindings has an invalid type")
	}
	if !inheritOK || inherit.IsNull() || inherit.IsUnknown() {
		return nil, fmt.Errorf("inherit must be known")
	}
	if inherit.ValueBool() {
		if !bindings.IsNull() && len(bindings.Elements()) != 0 {
			return nil, fmt.Errorf("inherit=true cannot be combined with bindings")
		}
		return nil, nil
	}
	if bindings.IsNull() {
		return nil, fmt.Errorf("bindings must be known when inherit=false")
	}
	items := make([]any, 0, len(bindings.Elements()))
	for _, item := range bindings.Elements() {
		wire, err := exWireValue(ctx, item)
		if err != nil {
			return nil, err
		}
		items = append(items, wire)
	}
	return items, nil
}
