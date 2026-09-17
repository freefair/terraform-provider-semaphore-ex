package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecretStorageResponsePreservesRedactedCredentialState(t *testing.T) {
	previous := ProjectSecretStorageModel{Secret: types.StringValue("configured"), SourceKey: types.StringValue("source")}
	model := storageModel(context.Background(), map[string]any{"id": float64(4), "project_id": float64(3), "name": "store", "type": "vault", "sync_revision": float64(2)}, previous)
	if model.Secret.ValueString() != "configured" || model.SourceKey.ValueString() != "source" || model.ID.ValueInt64() != 4 || model.SyncRevision.ValueInt64() != 2 {
		t.Fatal("redacted storage response did not preserve configured state")
	}
}

func TestSecretStorageParamsPreserveBooleanAndNumber(t *testing.T) {
	params := types.DynamicValue(types.ObjectValueMust(map[string]attr.Type{"use_iam_role": types.BoolType, "retries": types.Int64Type}, map[string]attr.Value{"use_iam_role": types.BoolValue(true), "retries": types.Int64Value(3)}))
	result := secretStorageParams(params)
	if result["use_iam_role"] != true || result["retries"] != int64(3) {
		t.Fatal("dynamic params changed boolean or number type")
	}
}

func TestSecretStorageImportedParamsInferNestedNativeValues(t *testing.T) {
	value := secretStorageDynamicFromAPI(map[string]any{"enabled": true, "timeout": json.Number("5"), "nested": map[string]any{"name": "vault", "empty": nil}, "ports": []any{json.Number("8200"), json.Number("8201")}})
	object, ok := value.UnderlyingValue().(types.Object)
	if !ok {
		t.Fatal("imported params are not an object")
	}
	if _, ok := object.Attributes()["enabled"].(types.Bool); !ok {
		t.Fatal("boolean parameter lost its type")
	}
	if _, ok := object.Attributes()["timeout"].(types.Number); !ok {
		t.Fatal("numeric parameter lost its type")
	}
	if _, ok := object.Attributes()["nested"].(types.Object); !ok {
		t.Fatal("nested object lost its type")
	}
	if _, ok := object.Attributes()["ports"].(types.List); !ok {
		t.Fatal("homogeneous list lost its type")
	}
}

func TestSecretStorageEmptyParamsRemainNullWhenUnconfigured(t *testing.T) {
	model := storageModel(context.Background(), map[string]any{"params": map[string]any{}}, ProjectSecretStorageModel{Params: types.DynamicNull()})
	if !model.Params.IsNull() {
		t.Fatal("empty server params must not turn an omitted configuration into an object")
	}
}

func TestSecretStorageResponseReflectsChangedKnownParams(t *testing.T) {
	previous := ProjectSecretStorageModel{Params: secretStorageDynamicFromAPI(map[string]any{"url": "https://old.example", "retries": json.Number("1")})}
	model := storageModel(context.Background(), map[string]any{"params": map[string]any{"url": "https://new.example", "retries": json.Number("2")}}, previous)
	object, ok := model.Params.UnderlyingValue().(types.Object)
	if !ok {
		t.Fatal("changed params are not an object")
	}
	if object.Attributes()["url"].(types.String).ValueString() != "https://new.example" {
		t.Fatalf("params did not reflect the server value: %#v", object.Attributes())
	}
	if _, ok := object.Attributes()["retries"].(types.Number); !ok {
		t.Fatalf("JSON numeric parameter lost its native type: %#v", object.Attributes()["retries"])
	}

	empty := storageModel(context.Background(), map[string]any{"params": map[string]any{}}, previous)
	emptyObject, ok := empty.Params.UnderlyingValue().(types.Object)
	if !ok || len(emptyObject.Attributes()) != 0 {
		t.Fatalf("known params cleared remotely must become an empty object: %#v", empty.Params)
	}
}

func TestSecretStoragePreservesImplicitTokenAuthMethodShape(t *testing.T) {
	previous := ProjectSecretStorageModel{Params: secretStorageDynamicFromAPI(map[string]any{"url": "http://127.0.0.1:8200"})}
	model := storageModel(context.Background(), map[string]any{"params": map[string]any{"url": "http://127.0.0.1:8200", "auth_method": "token"}}, previous)
	object, ok := model.Params.UnderlyingValue().(types.Object)
	if !ok {
		t.Fatal("params are not an object")
	}
	if _, exists := object.Attributes()["auth_method"]; exists {
		t.Fatal("implicit token auth_method must not change omitted configuration shape")
	}
}

func TestSecretStorageSyncPreservesOmittedPathMetadata(t *testing.T) {
	ctx := context.Background()
	path := ProjectSecretStorageSyncPathModel{ID: types.Int64Value(9), AccessKeyID: types.Int64Value(3), Mount: types.StringValue("kv"), Path: types.StringValue("apps/test"), Field: types.StringValue("token"), RemoteVersion: types.Int64Value(7)}
	prior := ProjectSecretStorageModel{SyncRevision: types.Int64Value(4)}
	prior.SyncPaths, _ = types.ListValueFrom(ctx, secretStorageSyncPathType(), []ProjectSecretStorageSyncPathModel{path})
	planned := ProjectSecretStorageModel{SyncRevision: types.Int64Unknown(), SyncPaths: types.ListUnknown(secretStorageSyncPathType())}
	preserved := preserveSecretStorageUpdateState(ctx, planned, prior)
	if preserved.SyncRevision.ValueInt64() != 4 || preserved.SyncPaths.IsUnknown() {
		t.Fatal("omitted sync fields must retain the prior request metadata")
	}
	payload := storagePayload(ProjectSecretStorageModel{SyncEnabled: types.BoolValue(true), SyncDirection: types.StringValue("outbound"), SyncInterval: types.Int64Value(10), SyncRevision: preserved.SyncRevision, SyncPaths: preserved.SyncPaths}, "")
	paths := payload["sync_paths"].([]map[string]any)
	if paths[0]["id"] != int64(9) || paths[0]["remote_version"] != int64(7) {
		t.Fatal("sync path identity metadata was not retained")
	}
}

func TestSecretStorageUpdatePreservesUnknownExternalSource(t *testing.T) {
	prior := ProjectSecretStorageModel{SourceType: types.StringValue("env"), SourceKey: types.StringValue("EXTERNAL_SECRET")}
	planned := ProjectSecretStorageModel{SourceType: types.StringUnknown(), SourceKey: types.StringUnknown()}
	preserved := preserveSecretStorageUpdateState(context.Background(), planned, prior)
	if preserved.SourceType.ValueString() != "env" || preserved.SourceKey.ValueString() != "EXTERNAL_SECRET" {
		t.Fatalf("unknown source reference was not retained: %#v", preserved)
	}
}

func TestSecretStorageSyncPathDecodesJSONNumbers(t *testing.T) {
	model := storageModel(context.Background(), map[string]any{"sync_paths": []any{map[string]any{
		"id": json.Number("9"), "access_key_id": json.Number("3"), "mount": "kv", "path": "apps/test", "field": "token", "remote_version": json.Number("7"),
	}}}, ProjectSecretStorageModel{})
	var paths []ProjectSecretStorageSyncPathModel
	if diagnostics := model.SyncPaths.ElementsAs(context.Background(), &paths, false); diagnostics.HasError() || len(paths) != 1 {
		t.Fatal("sync path did not decode")
	}
	if paths[0].ID.ValueInt64() != 9 || paths[0].AccessKeyID.ValueInt64() != 3 || paths[0].RemoteVersion.ValueInt64() != 7 {
		t.Fatalf("decoded path metadata = %#v", paths[0])
	}
}

func TestAcc_ProjectSecretStorage_local(t *testing.T) {
	suffix := acctest.RandString(8)
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	defer mock.Close()
	vaultURL := strings.Replace(mock.URL, "127.0.0.1", "localhost", 1)
	config := fmt.Sprintf(`resource "semaphore_ex_project" "test" { name = "storage-%[1]s" }
resource "semaphore_ex_project_secret_storage" "test" {
  project_id = semaphore_ex_project.test.id
  name = "local-%[1]s"
  type = "vault"
  params = { url = "%[2]s", auth_method = "token" }
  secret_wo = "fixture-token"
  secret_wo_version = 1
}

data "semaphore_ex_project_secret_storage" "test" {
  project_id = semaphore_ex_project.test.id
  id = semaphore_ex_project_secret_storage.test.id
}`, suffix, vaultURL)
	updatedConfig := strings.Replace(config, "local-"+suffix, "updated-"+suffix, 1)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: config, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttrSet("semaphore_ex_project_secret_storage.test", "id"), resource.TestCheckResourceAttr("data.semaphore_ex_project_secret_storage.test", "name", fmt.Sprintf("local-%s", suffix)))}, {Config: updatedConfig, Check: resource.TestCheckResourceAttr("semaphore_ex_project_secret_storage.test", "name", fmt.Sprintf("updated-%s", suffix))}, {ResourceName: "semaphore_ex_project_secret_storage.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"secret_wo_version"}, ImportStateIdFunc: func(s *terraform.State) (string, error) {
		r := s.RootModule().Resources["semaphore_ex_project_secret_storage.test"]
		return fmt.Sprintf("project/%s/secret_storage/%s", r.Primary.Attributes["project_id"], r.Primary.Attributes["id"]), nil
	}}}})
}

func TestAcc_ProjectSecretStorage_syncLifecycle(t *testing.T) {
	suffix := acctest.RandString(8)
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	defer mock.Close()
	vaultURL := strings.Replace(mock.URL, "127.0.0.1", "localhost", 1)
	config := fmt.Sprintf(`
resource "semaphore_ex_project" "test" { name = "storage-sync-project-%[1]s" }
resource "semaphore_ex_project_key" "sync" {
  project_id = semaphore_ex_project.test.id
  name = "storage-sync-key-%[1]s"
  none = {}
}
resource "semaphore_ex_project_secret_storage" "test" {
  project_id = semaphore_ex_project.test.id
  name = "storage-sync-%[1]s"
  type = "vault"
  params = { url = "%[2]s", auth_method = "token" }
  secret_wo = "fixture-token"
  secret_wo_version = 1
  sync_enabled = true
  sync_direction = "outbound"
  sync_interval = 5
  sync_paths = [{
    access_key_id = semaphore_ex_project_key.sync.id
    mount = "secret"
    path = "applications/acceptance"
    field = "token"
  }]
}`, suffix, vaultURL)
	updatedConfig := strings.Replace(config, "storage-sync-"+suffix, "storage-sync-renamed-"+suffix, 1)
	var pathID, remoteVersion string
	captureMetadata := func(s *terraform.State) error {
		attributes := s.RootModule().Resources["semaphore_ex_project_secret_storage.test"].Primary.Attributes
		pathID, remoteVersion = attributes["sync_paths.0.id"], attributes["sync_paths.0.remote_version"]
		if pathID == "" || remoteVersion == "" {
			return fmt.Errorf("sync path metadata is absent: id=%q remote_version=%q", pathID, remoteVersion)
		}
		return nil
	}
	assertMetadata := func(s *terraform.State) error {
		attributes := s.RootModule().Resources["semaphore_ex_project_secret_storage.test"].Primary.Attributes
		if attributes["sync_paths.0.id"] != pathID || attributes["sync_paths.0.remote_version"] != remoteVersion {
			return fmt.Errorf("sync path metadata changed: id=%q remote_version=%q, want id=%q remote_version=%q", attributes["sync_paths.0.id"], attributes["sync_paths.0.remote_version"], pathID, remoteVersion)
		}
		return nil
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_project_secret_storage.test", "sync_revision", "1"), resource.TestCheckResourceAttr("semaphore_ex_project_secret_storage.test", "sync_paths.#", "1"), captureMetadata)},
		{Config: updatedConfig, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_project_secret_storage.test", "sync_revision", "2"), assertMetadata)},
		{ResourceName: "semaphore_ex_project_secret_storage.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"secret_wo_version"}, ImportStateIdFunc: func(s *terraform.State) (string, error) {
			r := s.RootModule().Resources["semaphore_ex_project_secret_storage.test"]
			return fmt.Sprintf("project/%s/secret_storage/%s", r.Primary.Attributes["project_id"], r.Primary.Attributes["id"]), nil
		}, Check: assertMetadata},
	}})
}
