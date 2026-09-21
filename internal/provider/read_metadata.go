package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type readMetadataSpec struct {
	route      string
	parameters map[string]string
	fields     map[string]ds.Attribute
	aliases    map[string]string
	list       bool
}
type readMetadataDataSource struct {
	datasource.DataSource
	spec   readMetadataSpec
	client *apiclient.SemaphoreUI
}

func withReadMetadata(source datasource.DataSource, name string) datasource.DataSource {
	spec := readMetadataSpec{parameters: map[string]string{}, fields: map[string]ds.Attribute{}}
	stringFields := func(names ...string) {
		for _, name := range names {
			spec.fields[name] = ds.StringAttribute{Computed: true, MarkdownDescription: "Read-only server metadata; null when unavailable."}
		}
	}
	intFields := func(names ...string) {
		for _, name := range names {
			spec.fields[name] = ds.Int64Attribute{Computed: true, MarkdownDescription: "Read-only server metadata; null when unavailable."}
		}
	}
	switch name {
	case "runner", "project_runner":
		spec.route = "/runners/{runner_id}"
		spec.parameters["runner_id"] = "id"
		if name == "project_runner" {
			spec.route = "/project/{project_id}/runners/{runner_id}"
			spec.parameters["project_id"] = "project_id"
		}
		stringFields("status", "version", "platform", "executor_type", "docker_policy_hash", "k8s_cluster_alias", "k8s_namespace", "k8s_policy_hash", "touched", "started_at", "cleaning_requested", "registration_kind", "security_reason", "security_remediation", "transport_trust", "security_checked_at")
		intFields("current_load", "docker_policy_revision", "k8s_policy_revision", "security_protocol_version")
		spec.fields["security_compliant"] = ds.BoolAttribute{Computed: true}
	case "project_schedule":
		spec.route = "/project/{project_id}/schedules/{schedule_id}"
		spec.parameters = map[string]string{"project_id": "project_id", "schedule_id": "id"}
		stringFields("effective_timezone", "next_run")
	case "project_environment", "project_secret_storage":
		spec.route = "/project/{project_id}/environment/{object_id}"
		if name == "project_secret_storage" {
			spec.route = "/project/{project_id}/secret_storages/{object_id}"
		}
		spec.parameters = map[string]string{"project_id": "project_id", "object_id": "id"}
		stringFields("last_synced_at", "last_sync_failed_at")
		spec.fields["sync_path_status"] = ds.ListNestedAttribute{Computed: true, MarkdownDescription: "Read-only path synchronization identity and fingerprint; no secret contents.", NestedObject: ds.NestedAttributeObject{Attributes: map[string]ds.Attribute{"id": ds.Int64Attribute{Computed: true}, "path": ds.StringAttribute{Computed: true}, "content_fingerprint": ds.StringAttribute{Computed: true}, "remote_version": ds.Int64Attribute{Computed: true}}}}
		spec.aliases = map[string]string{"sync_path_status": "sync_paths"}
	case "workflow_definition":
		spec.route = "/project/{project_id}/workflows/{workflow_id}"
		spec.parameters = map[string]string{"project_id": "project_id", "workflow_id": "id"}
		intFields("current_version_id")
	case "workflow_trigger":
		spec.route = "/project/{project_id}/workflows/{workflow_id}/triggers/{trigger_id}"
		spec.parameters = map[string]string{"project_id": "project_id", "workflow_id": "workflow_id", "trigger_id": "id"}
		intFields("owner_user_id")
	case "ldap_configuration":
		spec.route = "/capabilities/ldap"
		spec.list = true
		intFields("recovery_admin_user_id")
		stringFields("created", "updated")
		spec.fields["eligible_user_ids"] = ds.ListAttribute{Computed: true, ElementType: types.Int64Type}
		spec.fields["readiness"] = ds.SingleNestedAttribute{Computed: true, Attributes: map[string]ds.Attribute{"status": ds.StringAttribute{Computed: true}, "connection": ds.BoolAttribute{Computed: true}, "search": ds.BoolAttribute{Computed: true}, "bind": ds.BoolAttribute{Computed: true}, "recovery": ds.BoolAttribute{Computed: true}, "code": ds.StringAttribute{Computed: true}, "checked_at": ds.StringAttribute{Computed: true}}}
	default:
		return source
	}
	return &readMetadataDataSource{DataSource: source, spec: spec}
}
func (d *readMetadataDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	d.DataSource.Schema(ctx, req, resp)
	for name, field := range d.spec.fields {
		resp.Schema.Attributes[name] = field
	}
}
func (d *readMetadataDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	var ok bool
	d.client, ok = req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Invalid Provider Client", "Expected the configured Semaphore EX client.")
		return
	}
	if inner, ok := d.DataSource.(datasource.DataSourceWithConfigure); ok {
		inner.Configure(ctx, req, resp)
	}
}
func (d *readMetadataDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config types.Object
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var base datasource.SchemaResponse
	d.DataSource.Schema(ctx, datasource.SchemaRequest{}, &base)
	resp.Diagnostics.Append(base.Diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	projected := map[string]attr.Value{}
	projectedTypes := map[string]attr.Type{}
	for name, attribute := range base.Schema.Attributes {
		projectedTypes[name] = attribute.GetType()
		projected[name] = config.Attributes()[name]
	}
	baseObject, diags := types.ObjectValue(projectedTypes, projected)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	raw, err := baseObject.ToTerraformValue(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Read Configuration", err.Error())
		return
	}
	inner := datasource.ReadResponse{State: tfsdk.State{Schema: base.Schema}}
	d.DataSource.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: base.Schema, Raw: raw}}, &inner)
	resp.Diagnostics.Append(inner.Diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state types.Object
	resp.Diagnostics.Append(inner.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	metadata, err := d.read(ctx, state)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Runtime Metadata", err.Error())
		return
	}
	values := state.Attributes()
	for name, field := range d.spec.fields {
		key := name
		if alias, ok := d.spec.aliases[name]; ok {
			key = alias
		}
		value, err := exTypedValue(ctx, field.GetType(), metadata[key])
		if err != nil {
			resp.Diagnostics.AddError("Invalid Runtime Metadata", fmt.Sprintf("Field %s: %s", name, err))
			return
		}
		values[name] = value
	}
	result, diags := types.ObjectValue(config.AttributeTypes(ctx), values)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, result)...)
}
func (d *readMetadataDataSource) read(ctx context.Context, state types.Object) (map[string]any, error) {
	parameters := map[string]string{}
	for parameter, attribute := range d.spec.parameters {
		id, err := exPathID(state.Attributes()[attribute])
		if err != nil {
			return nil, err
		}
		parameters[parameter] = id
	}
	if d.spec.list {
		var records []map[string]any
		if err := exRequest(ctx, d.client, http.MethodGet, d.spec.route, parameters, nil, &records); err != nil {
			return nil, err
		}
		id, err := exPathID(state.Attributes()["id"])
		if err != nil {
			return nil, err
		}
		for _, record := range records {
			if got, ok := record["id"].(string); ok && strings.EqualFold(got, id) {
				return record, nil
			}
		}
		return nil, fmt.Errorf("metadata record disappeared during the read")
	}
	var result map[string]any
	err := exRequest(ctx, d.client, http.MethodGet, d.spec.route, parameters, nil, &result)
	return result, err
}
