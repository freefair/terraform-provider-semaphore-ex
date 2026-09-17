package provider

import (
	"context"
	"fmt"
	"net/http"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rs "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

type exDockerExecutionPolicyModel struct {
	ID                  types.String `tfsdk:"id"`
	Revision            types.Int64  `tfsdk:"revision"`
	Hash                types.String `tfsdk:"hash"`
	AllowedImages       types.Set    `tfsdk:"allowed_images"`
	RequireDigest       types.Bool   `tfsdk:"require_digest"`
	AllowedNetworks     types.Set    `tfsdk:"allowed_networks"`
	Network             types.String `tfsdk:"network"`
	User                types.String `tfsdk:"user"`
	NanoCPUs            types.Int64  `tfsdk:"nano_cpus"`
	MemoryBytes         types.Int64  `tfsdk:"memory_bytes"`
	PidsLimit           types.Int64  `tfsdk:"pids_limit"`
	PullTimeoutSeconds  types.Int64  `tfsdk:"pull_timeout_seconds"`
	MaxImageSizeBytes   types.Int64  `tfsdk:"max_image_size_bytes"`
	SeccompProfile      types.String `tfsdk:"seccomp_profile"`
	AppArmorProfile     types.String `tfsdk:"apparmor_profile"`
	AllowPrivileged     types.Bool   `tfsdk:"allow_privileged"`
	AllowBindMounts     types.Bool   `tfsdk:"allow_bind_mounts"`
	AllowDevices        types.Bool   `tfsdk:"allow_devices"`
	AllowHostNamespaces types.Bool   `tfsdk:"allow_host_namespaces"`
}

type exKubernetesExecutionPolicyModel struct {
	ID                       types.String `tfsdk:"id"`
	ClusterAlias             types.String `tfsdk:"cluster_alias"`
	Revision                 types.Int64  `tfsdk:"revision"`
	Hash                     types.String `tfsdk:"hash"`
	AllowedNamespaces        types.Set    `tfsdk:"allowed_namespaces"`
	AllowedImages            types.Set    `tfsdk:"allowed_images"`
	AllowedServiceAccounts   types.Set    `tfsdk:"allowed_service_accounts"`
	AllowedRuntimeClasses    types.Set    `tfsdk:"allowed_runtime_classes"`
	RuntimeClass             types.String `tfsdk:"runtime_class"`
	AllowedVolumeTypes       types.Set    `tfsdk:"allowed_volume_types"`
	AllowedNetworkProfiles   types.Set    `tfsdk:"allowed_network_profiles"`
	NetworkProfile           types.String `tfsdk:"network_profile"`
	NetworkPolicyEnforcement types.String `tfsdk:"network_policy_enforcement"`
	Resources                types.Object `tfsdk:"resources"`
	TerminalRetentionSeconds types.Int64  `tfsdk:"terminal_retention_seconds"`
}

type exExecutorPolicyResource struct {
	client     *apiclient.SemaphoreUI
	kubernetes bool
}
type exExecutorPolicyDataSource struct {
	client     *apiclient.SemaphoreUI
	kubernetes bool
}

var exKubernetesExecutionResourcesType = map[string]attr.Type{
	"cpu_request_milli": types.Int64Type, "cpu_limit_milli": types.Int64Type,
	"memory_request_bytes": types.Int64Type, "memory_limit_bytes": types.Int64Type,
	"ephemeral_storage_request_bytes": types.Int64Type, "ephemeral_storage_limit_bytes": types.Int64Type,
}

var _ resource.ResourceWithImportState = &exExecutorPolicyResource{}

func NewDockerExecutionPolicyResource() resource.Resource       { return &exExecutorPolicyResource{} }
func NewDockerExecutionPolicyDataSource() datasource.DataSource { return &exExecutorPolicyDataSource{} }
func NewKubernetesExecutionPolicyResource() resource.Resource {
	return &exExecutorPolicyResource{kubernetes: true}
}
func NewKubernetesExecutionPolicyDataSource() datasource.DataSource {
	return &exExecutorPolicyDataSource{kubernetes: true}
}

func exExecutorPolicyName(kubernetes bool) string {
	if kubernetes {
		return "kubernetes_execution_policy"
	}
	return "docker_execution_policy"
}
func (r *exExecutorPolicyResource) Metadata(_ context.Context, q resource.MetadataRequest, p *resource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_" + exExecutorPolicyName(r.kubernetes)
}
func (d *exExecutorPolicyDataSource) Metadata(_ context.Context, q datasource.MetadataRequest, p *datasource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_" + exExecutorPolicyName(d.kubernetes)
}
func (r *exExecutorPolicyResource) Configure(_ context.Context, q resource.ConfigureRequest, p *resource.ConfigureResponse) {
	r.client = exExecutorPolicyClient(q.ProviderData, &p.Diagnostics)
}
func (d *exExecutorPolicyDataSource) Configure(_ context.Context, q datasource.ConfigureRequest, p *datasource.ConfigureResponse) {
	d.client = exExecutorPolicyClient(q.ProviderData, &p.Diagnostics)
}
func exExecutorPolicyClient(value any, diagnostics interface{ AddError(string, string) }) *apiclient.SemaphoreUI {
	if value == nil {
		return nil
	}
	c, ok := value.(*apiclient.SemaphoreUI)
	if !ok {
		diagnostics.AddError("Unexpected Executor Policy Configure Type", "Expected *client.SemaphoreUI.")
	}
	return c
}
func (r *exExecutorPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, p *resource.SchemaResponse) {
	p.Schema = exExecutorPolicyResourceSchema(r.kubernetes)
}
func (d *exExecutorPolicyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, p *datasource.SchemaResponse) {
	p.Schema = exExecutorPolicyDataSourceSchema(d.kubernetes)
}

func exExecutorPolicyResourceSchema(kubernetes bool) rs.Schema {
	if kubernetes {
		return rs.Schema{MarkdownDescription: "Manages the revision-fenced execution policy for one Semaphore EX Kubernetes cluster.", Attributes: exKubernetesExecutionPolicyResourceAttributes()}
	}
	return rs.Schema{MarkdownDescription: "Manages the revision-fenced global Semaphore EX Docker execution policy.", Attributes: exDockerExecutionPolicyResourceAttributes()}
}
func exExecutorPolicyDataSourceSchema(kubernetes bool) ds.Schema {
	if kubernetes {
		return ds.Schema{MarkdownDescription: "Reads the execution policy for one Semaphore EX Kubernetes cluster.", Attributes: exKubernetesExecutionPolicyDataSourceAttributes()}
	}
	return ds.Schema{MarkdownDescription: "Reads the global Semaphore EX Docker execution policy.", Attributes: exDockerExecutionPolicyDataSourceAttributes()}
}
func exDockerExecutionPolicyResourceAttributes() map[string]rs.Attribute {
	a := map[string]rs.Attribute{
		"id":       rs.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"revision": rs.Int64Attribute{Computed: true}, "hash": rs.StringAttribute{Computed: true},
	}
	for _, name := range []string{"allowed_images", "allowed_networks"} {
		a[name] = rs.SetAttribute{Required: true, ElementType: types.StringType}
	}
	for _, name := range []string{"require_digest", "allow_privileged", "allow_bind_mounts", "allow_devices", "allow_host_namespaces"} {
		a[name] = rs.BoolAttribute{Required: true}
	}
	for _, name := range []string{"network", "user", "seccomp_profile", "apparmor_profile"} {
		a[name] = rs.StringAttribute{Required: true}
	}
	for _, name := range []string{"nano_cpus", "memory_bytes", "pids_limit", "pull_timeout_seconds", "max_image_size_bytes"} {
		a[name] = rs.Int64Attribute{Required: true}
	}
	return a
}
func exDockerExecutionPolicyDataSourceAttributes() map[string]ds.Attribute {
	a := map[string]ds.Attribute{"id": ds.StringAttribute{Computed: true}, "revision": ds.Int64Attribute{Computed: true}, "hash": ds.StringAttribute{Computed: true}}
	for _, name := range []string{"allowed_images", "allowed_networks"} {
		a[name] = ds.SetAttribute{Computed: true, ElementType: types.StringType}
	}
	for _, name := range []string{"require_digest", "allow_privileged", "allow_bind_mounts", "allow_devices", "allow_host_namespaces"} {
		a[name] = ds.BoolAttribute{Computed: true}
	}
	for _, name := range []string{"network", "user", "seccomp_profile", "apparmor_profile"} {
		a[name] = ds.StringAttribute{Computed: true}
	}
	for _, name := range []string{"nano_cpus", "memory_bytes", "pids_limit", "pull_timeout_seconds", "max_image_size_bytes"} {
		a[name] = ds.Int64Attribute{Computed: true}
	}
	return a
}
func exKubernetesExecutionPolicyResourceAttributes() map[string]rs.Attribute {
	a := map[string]rs.Attribute{
		"id":            rs.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"cluster_alias": rs.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}}, "revision": rs.Int64Attribute{Computed: true}, "hash": rs.StringAttribute{Computed: true},
		"runtime_class": rs.StringAttribute{Required: true}, "network_profile": rs.StringAttribute{Required: true}, "network_policy_enforcement": rs.StringAttribute{Required: true}, "terminal_retention_seconds": rs.Int64Attribute{Required: true},
		"resources": rs.SingleNestedAttribute{Required: true, Attributes: exKubernetesExecutionResourcesResourceAttributes()},
	}
	for _, name := range []string{"allowed_namespaces", "allowed_images", "allowed_service_accounts", "allowed_runtime_classes", "allowed_volume_types", "allowed_network_profiles"} {
		a[name] = rs.SetAttribute{Required: true, ElementType: types.StringType}
	}
	return a
}
func exKubernetesExecutionResourcesResourceAttributes() map[string]rs.Attribute {
	a := map[string]rs.Attribute{}
	for name := range exKubernetesExecutionResourcesType {
		a[name] = rs.Int64Attribute{Required: true}
	}
	return a
}
func exKubernetesExecutionPolicyDataSourceAttributes() map[string]ds.Attribute {
	a := map[string]ds.Attribute{"id": ds.StringAttribute{Computed: true}, "cluster_alias": ds.StringAttribute{Required: true}, "revision": ds.Int64Attribute{Computed: true}, "hash": ds.StringAttribute{Computed: true}, "runtime_class": ds.StringAttribute{Computed: true}, "network_profile": ds.StringAttribute{Computed: true}, "network_policy_enforcement": ds.StringAttribute{Computed: true}, "terminal_retention_seconds": ds.Int64Attribute{Computed: true}, "resources": ds.SingleNestedAttribute{Computed: true, Attributes: exKubernetesExecutionResourcesDataSourceAttributes()}}
	for _, name := range []string{"allowed_namespaces", "allowed_images", "allowed_service_accounts", "allowed_runtime_classes", "allowed_volume_types", "allowed_network_profiles"} {
		a[name] = ds.SetAttribute{Computed: true, ElementType: types.StringType}
	}
	return a
}
func exKubernetesExecutionResourcesDataSourceAttributes() map[string]ds.Attribute {
	a := map[string]ds.Attribute{}
	for name := range exKubernetesExecutionResourcesType {
		a[name] = ds.Int64Attribute{Computed: true}
	}
	return a
}

func (r *exExecutorPolicyResource) Create(ctx context.Context, q resource.CreateRequest, p *resource.CreateResponse) {
	if r.kubernetes {
		var plan exKubernetesExecutionPolicyModel
		p.Diagnostics.Append(q.Plan.Get(ctx, &plan)...)
		if p.Diagnostics.HasError() {
			return
		}
		current, e := exReadKubernetesExecutionPolicy(ctx, r.client, plan.ClusterAlias.ValueString())
		if e != nil {
			p.Diagnostics.AddError("Error Reading Kubernetes Execution Policy", e.Error())
			return
		}
		plan.Revision = current.Revision
		next, e := exWriteKubernetesExecutionPolicy(ctx, r.client, plan)
		if e != nil {
			p.Diagnostics.AddError("Error Creating Kubernetes Execution Policy", e.Error())
			return
		}
		p.Diagnostics.Append(p.State.Set(ctx, &next)...)
		return
	}
	var plan exDockerExecutionPolicyModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &plan)...)
	if p.Diagnostics.HasError() {
		return
	}
	current, e := exReadDockerExecutionPolicy(ctx, r.client)
	if e != nil {
		p.Diagnostics.AddError("Error Reading Docker Execution Policy", e.Error())
		return
	}
	plan.Revision = current.Revision
	next, e := exWriteDockerExecutionPolicy(ctx, r.client, plan)
	if e != nil {
		p.Diagnostics.AddError("Error Creating Docker Execution Policy", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (r *exExecutorPolicyResource) Read(ctx context.Context, q resource.ReadRequest, p *resource.ReadResponse) {
	if r.kubernetes {
		var state exKubernetesExecutionPolicyModel
		p.Diagnostics.Append(q.State.Get(ctx, &state)...)
		if p.Diagnostics.HasError() {
			return
		}
		next, e := exReadKubernetesExecutionPolicy(ctx, r.client, state.ClusterAlias.ValueString())
		if e != nil {
			p.Diagnostics.AddError("Error Reading Kubernetes Execution Policy", e.Error())
			return
		}
		p.Diagnostics.Append(p.State.Set(ctx, &next)...)
		return
	}
	next, e := exReadDockerExecutionPolicy(ctx, r.client)
	if e != nil {
		p.Diagnostics.AddError("Error Reading Docker Execution Policy", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (r *exExecutorPolicyResource) Update(ctx context.Context, q resource.UpdateRequest, p *resource.UpdateResponse) {
	if r.kubernetes {
		var plan, state exKubernetesExecutionPolicyModel
		p.Diagnostics.Append(q.Plan.Get(ctx, &plan)...)
		p.Diagnostics.Append(q.State.Get(ctx, &state)...)
		if p.Diagnostics.HasError() {
			return
		}
		plan.ID, plan.Revision = state.ID, state.Revision
		next, e := exWriteKubernetesExecutionPolicy(ctx, r.client, plan)
		if e != nil {
			p.Diagnostics.AddError("Error Updating Kubernetes Execution Policy", e.Error())
			return
		}
		p.Diagnostics.Append(p.State.Set(ctx, &next)...)
		return
	}
	var plan, state exDockerExecutionPolicyModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &plan)...)
	p.Diagnostics.Append(q.State.Get(ctx, &state)...)
	if p.Diagnostics.HasError() {
		return
	}
	plan.ID, plan.Revision = state.ID, state.Revision
	next, e := exWriteDockerExecutionPolicy(ctx, r.client, plan)
	if e != nil {
		p.Diagnostics.AddError("Error Updating Docker Execution Policy", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (r *exExecutorPolicyResource) Delete(ctx context.Context, q resource.DeleteRequest, p *resource.DeleteResponse) {
	if r.kubernetes {
		var state exKubernetesExecutionPolicyModel
		p.Diagnostics.Append(q.State.Get(ctx, &state)...)
		if p.Diagnostics.HasError() {
			return
		}
		if _, e := exWriteKubernetesExecutionPolicy(ctx, r.client, exDefaultKubernetesExecutionPolicy(state.ClusterAlias, state.Revision)); e != nil {
			p.Diagnostics.AddError("Error Resetting Kubernetes Execution Policy", e.Error())
		}
		return
	}
	var state exDockerExecutionPolicyModel
	p.Diagnostics.Append(q.State.Get(ctx, &state)...)
	if p.Diagnostics.HasError() {
		return
	}
	next := exDefaultDockerExecutionPolicy(state.Revision)
	if _, e := exWriteDockerExecutionPolicy(ctx, r.client, next); e != nil {
		p.Diagnostics.AddError("Error Resetting Docker Execution Policy", e.Error())
	}
}
func (r *exExecutorPolicyResource) ImportState(ctx context.Context, q resource.ImportStateRequest, p *resource.ImportStateResponse) {
	if r.kubernetes {
		next, e := exReadKubernetesExecutionPolicy(ctx, r.client, q.ID)
		if e != nil {
			p.Diagnostics.AddError("Error Importing Kubernetes Execution Policy", e.Error())
			return
		}
		p.Diagnostics.Append(p.State.Set(ctx, &next)...)
		return
	}
	next, e := exReadDockerExecutionPolicy(ctx, r.client)
	if e != nil {
		p.Diagnostics.AddError("Error Importing Docker Execution Policy", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}
func (d *exExecutorPolicyDataSource) Read(ctx context.Context, q datasource.ReadRequest, p *datasource.ReadResponse) {
	if d.kubernetes {
		var config exKubernetesExecutionPolicyModel
		p.Diagnostics.Append(q.Config.Get(ctx, &config)...)
		if p.Diagnostics.HasError() {
			return
		}
		next, e := exReadKubernetesExecutionPolicy(ctx, d.client, config.ClusterAlias.ValueString())
		if e != nil {
			p.Diagnostics.AddError("Error Reading Kubernetes Execution Policy", e.Error())
			return
		}
		p.Diagnostics.Append(p.State.Set(ctx, &next)...)
		return
	}
	next, e := exReadDockerExecutionPolicy(ctx, d.client)
	if e != nil {
		p.Diagnostics.AddError("Error Reading Docker Execution Policy", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &next)...)
}

func exReadDockerExecutionPolicy(ctx context.Context, client *apiclient.SemaphoreUI) (exDockerExecutionPolicyModel, error) {
	var raw map[string]any
	if e := exRequest(ctx, client, http.MethodGet, "/runners/docker-policy", nil, nil, &raw); e != nil {
		return exDockerExecutionPolicyModel{}, e
	}
	return exDockerExecutionPolicyModelFromWire(ctx, raw)
}
func exReadKubernetesExecutionPolicy(ctx context.Context, client *apiclient.SemaphoreUI, alias string) (exKubernetesExecutionPolicyModel, error) {
	var raw map[string]any
	if e := exRequest(ctx, client, http.MethodGet, "/runners/kubernetes-policies/{cluster_alias}", map[string]string{"cluster_alias": alias}, nil, &raw); e != nil {
		return exKubernetesExecutionPolicyModel{}, e
	}
	return exKubernetesExecutionPolicyModelFromWire(ctx, raw)
}
func exWriteDockerExecutionPolicy(ctx context.Context, client *apiclient.SemaphoreUI, model exDockerExecutionPolicyModel) (exDockerExecutionPolicyModel, error) {
	body, e := exDockerExecutionPolicyWire(ctx, model)
	if e != nil {
		return model, e
	}
	var raw map[string]any
	if e = exRequest(ctx, client, http.MethodPut, "/runners/docker-policy", nil, body, &raw); e != nil {
		return model, e
	}
	return exDockerExecutionPolicyModelFromWire(ctx, raw)
}
func exWriteKubernetesExecutionPolicy(ctx context.Context, client *apiclient.SemaphoreUI, model exKubernetesExecutionPolicyModel) (exKubernetesExecutionPolicyModel, error) {
	body, e := exKubernetesExecutionPolicyWire(ctx, model)
	if e != nil {
		return model, e
	}
	var raw map[string]any
	if e = exRequest(ctx, client, http.MethodPut, "/runners/kubernetes-policies/{cluster_alias}", map[string]string{"cluster_alias": model.ClusterAlias.ValueString()}, body, &raw); e != nil {
		return model, e
	}
	return exKubernetesExecutionPolicyModelFromWire(ctx, raw)
}

func exDockerExecutionPolicyWire(ctx context.Context, m exDockerExecutionPolicyModel) (map[string]any, error) {
	return exExecutorPolicyWire(ctx, m, []string{"revision", "allowed_images", "require_digest", "allowed_networks", "network", "user", "nano_cpus", "memory_bytes", "pids_limit", "pull_timeout_seconds", "max_image_size_bytes", "seccomp_profile", "apparmor_profile", "allow_privileged", "allow_bind_mounts", "allow_devices", "allow_host_namespaces"})
}
func exKubernetesExecutionPolicyWire(ctx context.Context, m exKubernetesExecutionPolicyModel) (map[string]any, error) {
	return exExecutorPolicyWire(ctx, m, []string{"cluster_alias", "revision", "allowed_namespaces", "allowed_images", "allowed_service_accounts", "allowed_runtime_classes", "runtime_class", "allowed_volume_types", "allowed_network_profiles", "network_profile", "network_policy_enforcement", "resources", "terminal_retention_seconds"})
}
func exExecutorPolicyWire(ctx context.Context, model any, names []string) (map[string]any, error) {
	attrs := exExecutorPolicyAttributes(model)
	if attrs == nil {
		return nil, fmt.Errorf("executor policy configuration is invalid")
	}
	result := make(map[string]any, len(names))
	for _, name := range names {
		wire, err := exWireValue(ctx, attrs[name])
		if err != nil {
			return nil, fmt.Errorf("%s is invalid: %w", name, err)
		}
		result[name] = wire
	}
	return result, nil
}
func exExecutorPolicyAttributes(model any) map[string]attr.Value {
	switch m := model.(type) {
	case exDockerExecutionPolicyModel:
		return map[string]attr.Value{"revision": m.Revision, "allowed_images": m.AllowedImages, "require_digest": m.RequireDigest, "allowed_networks": m.AllowedNetworks, "network": m.Network, "user": m.User, "nano_cpus": m.NanoCPUs, "memory_bytes": m.MemoryBytes, "pids_limit": m.PidsLimit, "pull_timeout_seconds": m.PullTimeoutSeconds, "max_image_size_bytes": m.MaxImageSizeBytes, "seccomp_profile": m.SeccompProfile, "apparmor_profile": m.AppArmorProfile, "allow_privileged": m.AllowPrivileged, "allow_bind_mounts": m.AllowBindMounts, "allow_devices": m.AllowDevices, "allow_host_namespaces": m.AllowHostNamespaces}
	case exKubernetesExecutionPolicyModel:
		return map[string]attr.Value{"cluster_alias": m.ClusterAlias, "revision": m.Revision, "allowed_namespaces": m.AllowedNamespaces, "allowed_images": m.AllowedImages, "allowed_service_accounts": m.AllowedServiceAccounts, "allowed_runtime_classes": m.AllowedRuntimeClasses, "runtime_class": m.RuntimeClass, "allowed_volume_types": m.AllowedVolumeTypes, "allowed_network_profiles": m.AllowedNetworkProfiles, "network_profile": m.NetworkProfile, "network_policy_enforcement": m.NetworkPolicyEnforcement, "resources": m.Resources, "terminal_retention_seconds": m.TerminalRetentionSeconds}
	}
	return nil
}
func exDockerExecutionPolicyModelFromWire(ctx context.Context, raw map[string]any) (exDockerExecutionPolicyModel, error) {
	var model exDockerExecutionPolicyModel
	err := exExecutorPolicyDecode(ctx, false, raw, "docker", &model)
	return model, err
}

func exKubernetesExecutionPolicyModelFromWire(ctx context.Context, raw map[string]any) (exKubernetesExecutionPolicyModel, error) {
	var model exKubernetesExecutionPolicyModel
	err := exExecutorPolicyDecode(ctx, true, raw, raw["cluster_alias"], &model)
	return model, err
}

func exExecutorPolicyDecode(ctx context.Context, kubernetes bool, raw map[string]any, id any, target any) error {
	response := make(map[string]any, len(raw)+1)
	for name, value := range raw {
		response[name] = value
	}
	response["id"] = id
	value, err := exTypedValue(ctx, exExecutorPolicyResourceSchema(kubernetes).Type(), response)
	if err != nil {
		return err
	}
	object, ok := value.(types.Object)
	if !ok {
		return fmt.Errorf("API returned an invalid executor policy object")
	}
	if diagnostics := object.As(ctx, target, basetypes.ObjectAsOptions{}); diagnostics.HasError() {
		return fmt.Errorf("API executor policy does not match its declared schema")
	}
	return nil
}

func exDefaultDockerExecutionPolicy(revision types.Int64) exDockerExecutionPolicyModel {
	values, _ := types.SetValueFrom(context.Background(), types.StringType, []string{})
	networks, _ := types.SetValueFrom(context.Background(), types.StringType, []string{"none"})
	return exDockerExecutionPolicyModel{ID: types.StringValue("docker"), Revision: revision, AllowedImages: values, RequireDigest: types.BoolValue(true), AllowedNetworks: networks, Network: types.StringValue("none"), User: types.StringValue("65534:0"), NanoCPUs: types.Int64Value(1_000_000_000), MemoryBytes: types.Int64Value(512 * 1024 * 1024), PidsLimit: types.Int64Value(256), PullTimeoutSeconds: types.Int64Value(300), MaxImageSizeBytes: types.Int64Value(2 * 1024 * 1024 * 1024), SeccompProfile: types.StringValue("default"), AppArmorProfile: types.StringValue("docker-default"), AllowPrivileged: types.BoolValue(false), AllowBindMounts: types.BoolValue(false), AllowDevices: types.BoolValue(false), AllowHostNamespaces: types.BoolValue(false)}
}

func exDefaultKubernetesExecutionPolicy(alias types.String, revision types.Int64) exKubernetesExecutionPolicyModel {
	empty, _ := types.SetValueFrom(context.Background(), types.StringType, []string{})
	resources, _ := types.ObjectValue(exKubernetesExecutionResourcesType, map[string]attr.Value{
		"cpu_request_milli": types.Int64Value(0), "cpu_limit_milli": types.Int64Value(0),
		"memory_request_bytes": types.Int64Value(0), "memory_limit_bytes": types.Int64Value(0),
		"ephemeral_storage_request_bytes": types.Int64Value(0), "ephemeral_storage_limit_bytes": types.Int64Value(0),
	})
	return exKubernetesExecutionPolicyModel{
		ID: alias, ClusterAlias: alias, Revision: revision,
		AllowedNamespaces: empty, AllowedImages: empty, AllowedServiceAccounts: empty,
		AllowedRuntimeClasses: empty, RuntimeClass: types.StringValue(""), AllowedVolumeTypes: empty,
		AllowedNetworkProfiles: empty, NetworkProfile: types.StringValue(""),
		NetworkPolicyEnforcement: types.StringValue("unsupported"), Resources: resources,
		TerminalRetentionSeconds: types.Int64Value(0),
	}
}
