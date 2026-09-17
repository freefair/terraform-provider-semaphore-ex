package provider

import (
	"context"
	"encoding/json"
	"fmt"
	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	schemaR "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"net/http"
	"strconv"
)

type projectSecretStorageResource struct{ client *apiclient.SemaphoreUI }

var _ resource.ResourceWithImportState = &projectSecretStorageResource{}

func NewProjectSecretStorageResource() resource.Resource { return &projectSecretStorageResource{} }
func (r *projectSecretStorageResource) Metadata(_ context.Context, q resource.MetadataRequest, p *resource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_project_secret_storage"
}
func (r *projectSecretStorageResource) Configure(_ context.Context, q resource.ConfigureRequest, p *resource.ConfigureResponse) {
	if q.ProviderData != nil {
		if c, ok := q.ProviderData.(*apiclient.SemaphoreUI); ok {
			r.client = c
		} else {
			p.Diagnostics.AddError("Unexpected Resource Configure Type", "Expected *client.SemaphoreUI.")
		}
	}
}
func (r *projectSecretStorageResource) Schema(c context.Context, _ resource.SchemaRequest, p *resource.SchemaResponse) {
	p.Schema = ProjectSecretStorageSchema().GetResource(c)
	p.Schema.Attributes["params"] = schemaR.DynamicAttribute{Optional: true}
}
func storagePayload(m ProjectSecretStorageModel, secret string) map[string]any {
	params := map[string]any{}
	p := map[string]any{"id": m.ID.ValueInt64(), "project_id": m.ProjectID.ValueInt64(), "name": m.Name.ValueString(), "type": m.Type.ValueString(), "params": params}
	if !m.Params.IsNull() && !m.Params.IsUnknown() {
		params = secretStorageParams(m.Params)
		p["params"] = params
	}
	if secret != "" {
		p["secret"] = secret
	}
	if !m.SourceType.IsNull() && !m.SourceType.IsUnknown() {
		p["source_storage_type"] = m.SourceType.ValueString()
		p["secret"] = m.SourceKey.ValueString()
	}
	if !m.SyncEnabled.IsNull() {
		p["sync_enabled"] = m.SyncEnabled.ValueBool()
		p["sync_direction"] = m.SyncDirection.ValueString()
		p["sync_interval"] = m.SyncInterval.ValueInt64()
		p["sync_revision"] = m.SyncRevision.ValueInt64()
		if !m.SyncPaths.IsNull() && !m.SyncPaths.IsUnknown() {
			var paths []ProjectSecretStorageSyncPathModel
			m.SyncPaths.ElementsAs(context.Background(), &paths, false)
			values := make([]map[string]any, 0, len(paths))
			for _, path := range paths {
				values = append(values, map[string]any{"id": path.ID.ValueInt64(), "access_key_id": path.AccessKeyID.ValueInt64(), "mount": path.Mount.ValueString(), "path": path.Path.ValueString(), "field": path.Field.ValueString(), "remote_version": path.RemoteVersion.ValueInt64()})
			}
			p["sync_paths"] = values
		}
	}
	return p
}

func preserveSecretStorageUpdateState(c context.Context, planned, prior ProjectSecretStorageModel) ProjectSecretStorageModel {
	// Optional+computed source fields are unknown for a rename that does not
	// change the reference. Keep the pair together so ValueString cannot turn
	// an existing source key into an empty literal secret.
	if planned.SourceType.IsUnknown() {
		planned.SourceType = prior.SourceType
	}
	if !planned.SourceType.IsNull() && !planned.SourceType.IsUnknown() && planned.SourceKey.IsUnknown() {
		planned.SourceKey = prior.SourceKey
	}
	if planned.SyncRevision.IsUnknown() {
		planned.SyncRevision = prior.SyncRevision
	}
	if planned.SyncPaths.IsNull() || planned.SyncPaths.IsUnknown() {
		planned.SyncPaths = prior.SyncPaths
		return planned
	}
	var paths, previous []ProjectSecretStorageSyncPathModel
	if planned.SyncPaths.ElementsAs(c, &paths, false).HasError() || prior.SyncPaths.ElementsAs(c, &previous, false).HasError() {
		return planned
	}
	byIdentity := make(map[string]ProjectSecretStorageSyncPathModel, len(previous))
	for _, path := range previous {
		byIdentity[secretStorageSyncPathIdentity(path)] = path
	}
	for index := range paths {
		previous, found := byIdentity[secretStorageSyncPathIdentity(paths[index])]
		if !found {
			continue
		}
		if paths[index].ID.IsUnknown() || paths[index].ID.IsNull() || paths[index].ID.ValueInt64() == 0 {
			paths[index].ID = previous.ID
		}
		if paths[index].RemoteVersion.IsUnknown() || paths[index].RemoteVersion.IsNull() || paths[index].RemoteVersion.ValueInt64() == 0 {
			paths[index].RemoteVersion = previous.RemoteVersion
		}
	}
	planned.SyncPaths, _ = types.ListValueFrom(c, secretStorageSyncPathType(), paths)
	return planned
}

func secretStorageSyncPathIdentity(path ProjectSecretStorageSyncPathModel) string {
	return fmt.Sprintf("%d\x00%s\x00%s\x00%s", path.AccessKeyID.ValueInt64(), path.Mount.ValueString(), path.Path.ValueString(), path.Field.ValueString())
}

func secretStorageSyncPathType() types.ObjectType {
	return types.ObjectType{AttrTypes: map[string]attr.Type{"id": types.Int64Type, "access_key_id": types.Int64Type, "mount": types.StringType, "path": types.StringType, "field": types.StringType, "remote_version": types.Int64Type}}
}
func storageModel(c context.Context, in map[string]any, prev ProjectSecretStorageModel) ProjectSecretStorageModel {
	m := prev
	if v, ok := storageInt(in["id"]); ok {
		m.ID = types.Int64Value(v)
	}
	if v, ok := storageInt(in["project_id"]); ok {
		m.ProjectID = types.Int64Value(v)
	}
	if v, ok := in["name"].(string); ok {
		m.Name = types.StringValue(v)
	}
	if v, ok := in["type"].(string); ok {
		m.Type = types.StringValue(v)
	}
	if raw, ok := in["params"].(map[string]any); ok {
		if auth, hasAuth := raw["auth_method"].(string); hasAuth && auth == "token" && !m.Params.IsNull() && !m.Params.IsUnknown() {
			if previous, ok := m.Params.UnderlyingValue().(types.Object); ok {
				if _, configured := previous.Attributes()["auth_method"]; !configured {
					configuredParams := make(map[string]any, len(raw)-1)
					for key, value := range raw {
						if key != "auth_method" {
							configuredParams[key] = value
						}
					}
					raw = configuredParams
				}
			}
		}
		if len(raw) == 0 && (m.Params.IsNull() || m.Params.IsUnknown()) {
			m.Params = types.DynamicNull()
		} else {
			m.Params = secretStorageDynamicFromAPI(raw)
		}
	}
	if v, ok := in["source_storage_type"].(string); ok {
		m.SourceType = types.StringValue(v)
	}
	if v, ok := in["readonly"].(bool); ok {
		m.ReadOnly = types.BoolValue(v)
	}
	if v, ok := in["sync_enabled"].(bool); ok {
		m.SyncEnabled = types.BoolValue(v)
	}
	if v, ok := in["sync_direction"].(string); ok {
		m.SyncDirection = types.StringValue(v)
	}
	if v, ok := storageInt(in["sync_interval"]); ok {
		m.SyncInterval = types.Int64Value(v)
	}
	if v, ok := storageInt(in["sync_revision"]); ok {
		m.SyncRevision = types.Int64Value(v)
	}
	if raw, ok := in["sync_paths"].([]any); ok {
		paths := make([]ProjectSecretStorageSyncPathModel, 0, len(raw))
		for _, entry := range raw {
			value, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			// The API omits zero-valued remote versions, but Terraform computed
			// fields must remain known so a refresh cannot create an inconsistent
			// nested sync path.
			path := ProjectSecretStorageSyncPathModel{ID: types.Int64Value(0), RemoteVersion: types.Int64Value(0)}
			if v, ok := storageInt(value["id"]); ok {
				path.ID = types.Int64Value(v)
			}
			if v, ok := storageInt(value["access_key_id"]); ok {
				path.AccessKeyID = types.Int64Value(v)
			}
			if v, ok := value["mount"].(string); ok {
				path.Mount = types.StringValue(v)
			}
			if v, ok := value["path"].(string); ok {
				path.Path = types.StringValue(v)
			}
			if v, ok := value["field"].(string); ok {
				path.Field = types.StringValue(v)
			}
			if v, ok := storageInt(value["remote_version"]); ok {
				path.RemoteVersion = types.Int64Value(v)
			}
			paths = append(paths, path)
		}
		m.SyncPaths, _ = types.ListValueFrom(c, types.ObjectType{AttrTypes: map[string]attr.Type{"id": types.Int64Type, "access_key_id": types.Int64Type, "mount": types.StringType, "path": types.StringType, "field": types.StringType, "remote_version": types.Int64Type}}, paths)
	}
	if m.SourceType.IsUnknown() {
		m.SourceType = types.StringNull()
	}
	if m.SourceKey.IsUnknown() {
		m.SourceKey = types.StringNull()
	}
	if m.SyncInterval.IsUnknown() {
		m.SyncInterval = types.Int64Value(0)
	}
	if m.SyncRevision.IsUnknown() {
		m.SyncRevision = types.Int64Value(0)
	}
	if m.SyncEnabled.IsUnknown() {
		m.SyncEnabled = types.BoolValue(false)
	}
	if m.SyncDirection.IsUnknown() {
		m.SyncDirection = types.StringValue("read_only")
	}
	if m.SyncPaths.IsUnknown() {
		m.SyncPaths = types.ListValueMust(types.ObjectType{AttrTypes: map[string]attr.Type{"id": types.Int64Type, "access_key_id": types.Int64Type, "mount": types.StringType, "path": types.StringType, "field": types.StringType, "remote_version": types.Int64Type}}, []attr.Value{})
	}
	return m
}

func storageInt(value any) (int64, bool) {
	switch value := value.(type) {
	case json.Number:
		result, err := value.Int64()
		return result, err == nil
	case float64:
		return int64(value), true
	case int64:
		return value, true
	default:
		result, err := strconv.ParseInt(fmt.Sprint(value), 10, 64)
		return result, err == nil
	}
}
func (r *projectSecretStorageResource) read(c context.Context, m ProjectSecretStorageModel) (ProjectSecretStorageModel, error) {
	var out map[string]any
	err := exRequest(c, r.client, http.MethodGet, "/project/{project_id}/secret_storages/{storage_id}", map[string]string{"project_id": fmt.Sprint(m.ProjectID.ValueInt64()), "storage_id": fmt.Sprint(m.ID.ValueInt64())}, nil, &out)
	return storageModel(c, out, m), err
}
func (r *projectSecretStorageResource) Create(c context.Context, q resource.CreateRequest, p *resource.CreateResponse) {
	var m ProjectSecretStorageModel
	p.Diagnostics.Append(q.Plan.Get(c, &m)...)
	var cfg ProjectSecretStorageModel
	p.Diagnostics.Append(q.Config.Get(c, &cfg)...)
	secret := m.Secret.ValueString()
	if !cfg.SecretWO.IsNull() && !cfg.SecretWO.IsUnknown() {
		secret = cfg.SecretWO.ValueString()
	}
	var out map[string]any
	err := exRequest(c, r.client, http.MethodPost, "/project/{project_id}/secret_storages", map[string]string{"project_id": fmt.Sprint(m.ProjectID.ValueInt64())}, storagePayload(m, secret), &out)
	if err != nil {
		p.Diagnostics.AddError("Error Creating Project Secret Storage", err.Error())
		return
	}
	m = storageModel(c, out, m)
	p.Diagnostics.Append(p.State.Set(c, &m)...)
}
func (r *projectSecretStorageResource) Read(c context.Context, q resource.ReadRequest, p *resource.ReadResponse) {
	var m ProjectSecretStorageModel
	p.Diagnostics.Append(q.State.Get(c, &m)...)
	n, e := r.read(c, m)
	if e != nil {
		if exNotFound(e) {
			p.State.RemoveResource(c)
			return
		}
		p.Diagnostics.AddError("Error Reading Project Secret Storage", "The Semaphore EX API could not read the storage.")
		return
	}
	p.Diagnostics.Append(p.State.Set(c, &n)...)
}
func (r *projectSecretStorageResource) Update(c context.Context, q resource.UpdateRequest, p *resource.UpdateResponse) {
	var m, cfg, prior ProjectSecretStorageModel
	p.Diagnostics.Append(q.Plan.Get(c, &m)...)
	p.Diagnostics.Append(q.Config.Get(c, &cfg)...)
	p.Diagnostics.Append(q.State.Get(c, &prior)...)
	m = preserveSecretStorageUpdateState(c, m, prior)
	secret := m.Secret.ValueString()
	if !cfg.SecretWO.IsNull() && !cfg.SecretWO.IsUnknown() {
		secret = cfg.SecretWO.ValueString()
	}
	var out map[string]any
	e := exRequest(c, r.client, http.MethodPut, "/project/{project_id}/secret_storages/{storage_id}", map[string]string{"project_id": fmt.Sprint(m.ProjectID.ValueInt64()), "storage_id": fmt.Sprint(m.ID.ValueInt64())}, storagePayload(m, secret), &out)
	if e != nil {
		p.Diagnostics.AddError("Error Updating Project Secret Storage", "The Semaphore EX API rejected the update.")
		return
	}
	m, e = r.read(c, m)
	if e != nil {
		p.Diagnostics.AddError("Error Reading Project Secret Storage", "The Semaphore EX API could not read the updated storage.")
		return
	}
	p.Diagnostics.Append(p.State.Set(c, &m)...)
}
func (r *projectSecretStorageResource) Delete(c context.Context, q resource.DeleteRequest, p *resource.DeleteResponse) {
	var m ProjectSecretStorageModel
	p.Diagnostics.Append(q.State.Get(c, &m)...)
	if e := exRequest(c, r.client, http.MethodDelete, "/project/{project_id}/secret_storages/{storage_id}", map[string]string{"project_id": fmt.Sprint(m.ProjectID.ValueInt64()), "storage_id": fmt.Sprint(m.ID.ValueInt64())}, nil, nil); e != nil && !exNotFound(e) {
		p.Diagnostics.AddError("Error Deleting Project Secret Storage", "The Semaphore EX API rejected deletion, possibly because references remain.")
	}
}

func (r *projectSecretStorageResource) ImportState(c context.Context, q resource.ImportStateRequest, p *resource.ImportStateResponse) {
	fields, err := parseImportFields(q.ID, []string{"project", "secret_storage"})
	if err != nil {
		p.Diagnostics.AddError("Invalid Project Secret Storage Import ID", "Use project/<project_id>/secret_storage/<storage_id>.")
		return
	}
	m, err := r.read(c, ProjectSecretStorageModel{ProjectID: types.Int64Value(fields["project"]), ID: types.Int64Value(fields["secret_storage"])})
	if err != nil {
		p.Diagnostics.AddError("Error Reading Project Secret Storage", "The Semaphore EX API could not read the storage.")
		return
	}
	p.Diagnostics.Append(p.State.Set(c, &m)...)
}
