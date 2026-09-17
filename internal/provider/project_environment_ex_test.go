package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"

	projectclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client/project"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func projectEnvironmentTestList[T any](t *testing.T, attributeTypes map[string]attr.Type, values []T) types.List {
	t.Helper()
	list, diagnostics := types.ListValueFrom(context.Background(), types.ObjectType{AttrTypes: attributeTypes}, values)
	if diagnostics.HasError() {
		t.Fatalf("creating Terraform list: %v", diagnostics.Errors())
	}
	return list
}

func TestConvertProjectEnvironmentRemoteSecretReference(t *testing.T) {
	ctx := context.Background()
	plan := ProjectEnvironmentModel{
		ProjectID: types.Int64Value(10),
		Name:      types.StringValue("runtime references"),
		SecretStorage: &ProjectEnvironmentSecretStorageModel{
			ID:        types.Int64Value(12),
			KeyPrefix: types.StringValue("terraform-"),
		},
		SyncEnabled:  types.BoolValue(true),
		SyncInterval: types.Int64Value(30),
		Secrets: projectEnvironmentTestList(t, projectEnvironmentSecretAttributeTypes(), []ProjectEnvironmentSecretModel{{
			ID:        types.Int64Null(),
			Name:      types.StringValue("TOKEN"),
			Type:      types.StringValue("env"),
			Value:     types.StringNull(),
			StorageID: types.Int64Value(12),
			Mount:     types.StringValue("secret"),
			Path:      types.StringValue("applications/example"),
			Version:   types.Int64Value(3),
			Field:     types.StringValue("token"),
		}}),
		SyncPaths: projectEnvironmentTestList(t, projectEnvironmentSyncPathAttributeTypes(), []ProjectEnvironmentSyncPathModel{{
			ID:            types.Int64Null(),
			AccessKeyID:   types.Int64Value(41),
			Mount:         types.StringValue("secret"),
			Path:          types.StringValue("applications/example"),
			Field:         types.StringValue("token"),
			Prefix:        types.StringValue("EXAMPLE_"),
			Separator:     types.StringValue("_"),
			RemoteVersion: types.Int64Value(0),
		}}),
	}

	request := convertProjectEnvironmentModelToEnvironmentRequest(ctx, plan, &ProjectEnvironmentModel{})
	if request.SecretStorageID == nil || *request.SecretStorageID != 12 {
		t.Fatalf("secret storage ID = %v, want 12", request.SecretStorageID)
	}
	if request.SecretStorageKeyPrefix == nil || *request.SecretStorageKeyPrefix != "terraform-" {
		t.Fatalf("secret storage key prefix = %v, want terraform-", request.SecretStorageKeyPrefix)
	}
	if len(request.Secrets) != 1 {
		t.Fatalf("secret request count = %d, want 1", len(request.Secrets))
	}
	secret := request.Secrets[0]
	if secret.Secret != "" || secret.StorageID == nil || *secret.StorageID != 12 || secret.Path != "applications/example" || secret.Field != "token" || secret.Version != 3 {
		t.Fatalf("remote secret request = %#v", secret)
	}
	if len(request.SyncPaths) != 1 || request.SyncPaths[0].AccessKeyID != 41 || request.SyncPaths[0].Mount != "secret" {
		t.Fatalf("sync paths = %#v", request.SyncPaths)
	}

	storageID := int64(12)
	state := convertEnvironmentResponseToProjectEnvironmentModel(ctx, &models.Environment{
		ID:        3,
		ProjectID: 10,
		Name:      "runtime references",
		JSON:      "{}",
		Env:       "{}",
		Secrets: []*models.EnvironmentSecret{{
			ID: 1, Name: "TOKEN", Type: "env", StorageID: &storageID,
			Mount: "secret", Path: "applications/example", Version: 3, Field: "token",
		}},
		SyncPaths: []*models.SecretSyncPath{},
	}, &plan)
	var stateSecrets []ProjectEnvironmentSecretModel
	stateDiagnostics := state.Secrets.ElementsAs(ctx, &stateSecrets, false)
	if stateDiagnostics.HasError() || len(stateSecrets) != 1 || !stateSecrets[0].Value.IsNull() {
		t.Fatalf("remote secret state must not retain a plaintext value: %#v (%v)", stateSecrets, stateDiagnostics.Errors())
	}
}

func TestValidateProjectEnvironmentRejectsConflictingSecretSources(t *testing.T) {
	config := ProjectEnvironmentModel{
		Secrets: projectEnvironmentTestList(t, projectEnvironmentSecretAttributeTypes(), []ProjectEnvironmentSecretModel{{
			Name:      types.StringValue("TOKEN"),
			Type:      types.StringValue("env"),
			Value:     types.StringValue("plaintext"),
			StorageID: types.Int64Value(12),
			Path:      types.StringValue("applications/example"),
			Field:     types.StringValue("token"),
		}}),
	}
	var diagnostics diag.Diagnostics
	validateProjectEnvironmentConfig(context.Background(), config, &diagnostics)
	if !diagnostics.HasError() {
		t.Fatal("expected a conflict diagnostic for plaintext and remote secret sources")
	}
}

func TestConvertProjectEnvironmentEmptySecretStorageClearsBinding(t *testing.T) {
	plan := ProjectEnvironmentModel{
		ProjectID: types.Int64Value(10),
		Name:      types.StringValue("clear storage"),
		SecretStorage: &ProjectEnvironmentSecretStorageModel{
			ID:        types.Int64Null(),
			KeyPrefix: types.StringNull(),
		},
	}
	request := convertProjectEnvironmentModelToEnvironmentRequest(context.Background(), plan, &ProjectEnvironmentModel{})
	if request.SecretStorageID != nil || request.SecretStorageKeyPrefix != nil {
		t.Fatalf("empty secret_storage must encode null fields, got %#v", request)
	}
	body, err := projectEnvironmentUpdateBody(context.Background(), plan, ProjectEnvironmentModel{})
	if err != nil {
		t.Fatalf("encoding environment update: %v", err)
	}
	if storageID, exists := body["secret_storage_id"]; !exists || storageID != nil {
		t.Fatalf("secret_storage_id = %#v, exists = %t; want explicit null", storageID, exists)
	}
	if prefix, exists := body["secret_storage_key_prefix"]; !exists || prefix != nil {
		t.Fatalf("secret_storage_key_prefix = %#v, exists = %t; want explicit null", prefix, exists)
	}

	model := convertEnvironmentResponseToProjectEnvironmentModel(context.Background(), &models.Environment{
		ID:        3,
		ProjectID: 10,
		Name:      "clear storage",
		JSON:      "{}",
		Env:       "{}",
		Secrets:   []*models.EnvironmentSecret{},
		SyncPaths: []*models.SecretSyncPath{},
	}, &ProjectEnvironmentModel{})
	if model.SecretStorage != nil {
		t.Fatalf("empty API response secret storage = %#v, want nil", model.SecretStorage)
	}
}

func TestAcc_ProjectEnvironmentResource_remoteSecretReference(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance fixture requires TF_ACC before API setup")
	}
	testAccPreCheck(t)
	nameSuffix := acctest.RandString(8)
	mockVault := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
	}))
	defer mockVault.Close()
	mockVaultURL, err := url.Parse(mockVault.URL)
	if err != nil {
		t.Fatalf("parse mock Vault URL: %v", err)
	}
	// Runtime storage validation permits plain HTTP only for the literal
	// localhost host. httptest binds 127.0.0.1, so retain its ephemeral port.
	vaultURL := "http://localhost:" + mockVaultURL.Port()

	projectResponse, err := testClient().Project.PostProjects(&projectclient.PostProjectsParams{
		Project: &models.ProjectRequest{Name: "acceptance-runtime-project-" + nameSuffix},
	}, nil)
	if err != nil {
		t.Fatalf("create isolated project: %v", err)
	}
	projectID := projectResponse.Payload.ID
	defer func() {
		if _, deleteErr := testClient().Project.DeleteProjectProjectID(&projectclient.DeleteProjectProjectIDParams{ProjectID: projectID}, nil); deleteErr != nil {
			t.Errorf("delete isolated project: %v", deleteErr)
		}
	}()

	var storage struct {
		ID int64 `json:"id"`
	}
	if err := exRequest(context.Background(), testClient(), http.MethodPost,
		"/project/{project_id}/secret_storages", map[string]string{"project_id": strconv.FormatInt(projectID, 10)}, map[string]any{
			"project_id": projectID,
			"name":       "acceptance-runtime-storage-" + nameSuffix,
			"type":       "vault",
			"params": map[string]any{
				"url":         vaultURL,
				"auth_method": "token",
			},
			"secret": "fixture-token",
		}, &storage); err != nil {
		t.Fatalf("create isolated Vault storage: %v", err)
	}
	if storage.ID < 1 {
		t.Fatalf("created secret storage has invalid ID %d", storage.ID)
	}
	storageID := storage.ID

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectEnvironmentRemoteReferenceConfig(nameSuffix, projectID, storageID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("semaphore_ex_project_environment.test", "secret_storage.id", strconv.FormatInt(storageID, 10)),
					resource.TestCheckResourceAttr("semaphore_ex_project_environment.test", "secret_storage.key_prefix", "terraform-"),
					resource.TestCheckResourceAttr("semaphore_ex_project_environment.test", "secrets.#", "1"),
					resource.TestCheckResourceAttr("semaphore_ex_project_environment.test", "secrets.0.storage_id", strconv.FormatInt(storageID, 10)),
					resource.TestCheckResourceAttr("semaphore_ex_project_environment.test", "secrets.0.mount", "secret"),
					resource.TestCheckResourceAttr("semaphore_ex_project_environment.test", "secrets.0.path", "applications/acceptance"),
					resource.TestCheckResourceAttr("semaphore_ex_project_environment.test", "secrets.0.field", "token"),
					resource.TestCheckNoResourceAttr("semaphore_ex_project_environment.test", "secrets.0.value"),
				),
			},
			{
				Config: testAccProjectEnvironmentLiteralSecretConfig(nameSuffix, projectID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("semaphore_ex_project_environment.test", "secret_storage.id"),
					resource.TestCheckNoResourceAttr("semaphore_ex_project_environment.test", "secret_storage.key_prefix"),
					resource.TestCheckResourceAttr("semaphore_ex_project_environment.test", "secrets.#", "1"),
					resource.TestCheckResourceAttr("semaphore_ex_project_environment.test", "secrets.0.name", "REMOTE_TOKEN"),
					resource.TestCheckResourceAttr("semaphore_ex_project_environment.test", "secrets.0.value", "literal-token"),
					resource.TestCheckNoResourceAttr("semaphore_ex_project_environment.test", "secrets.0.storage_id"),
				),
			},
			{
				Config: testAccProjectEnvironmentSyncConfig(nameSuffix, projectID, storageID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("semaphore_ex_project_environment.test", "secret_storage.id", strconv.FormatInt(storageID, 10)),
					resource.TestCheckResourceAttr("semaphore_ex_project_environment.test", "sync_enabled", "false"),
					resource.TestCheckResourceAttr("semaphore_ex_project_environment.test", "sync_interval", "0"),
					resource.TestCheckResourceAttr("semaphore_ex_project_environment.test", "sync_paths.#", "1"),
					resource.TestCheckResourceAttr("semaphore_ex_project_environment.test", "sync_paths.0.mount", "secret"),
					resource.TestCheckResourceAttr("semaphore_ex_project_environment.test", "sync_paths.0.path", "applications/acceptance"),
					resource.TestCheckResourceAttr("semaphore_ex_project_environment.test", "sync_paths.0.field", "token"),
				),
			},
		},
	})
}

func testAccProjectEnvironmentLiteralSecretConfig(nameSuffix string, projectID int64) string {
	return fmt.Sprintf(`
resource "semaphore_ex_project_environment" "test" {
  project_id = %[2]d
  name       = "Test %[1]s"

  secret_storage = {}

  secrets = [{
    name  = "REMOTE_TOKEN"
    type  = "env"
    value = "literal-token"
  }]
}
`, nameSuffix, projectID)
}

func testAccProjectEnvironmentSyncConfig(nameSuffix string, projectID, storageID int64) string {
	return fmt.Sprintf(`
resource "semaphore_ex_project_environment" "test" {
  project_id = %[2]d
  name       = "Test %[1]s"

  secret_storage = {
    id = %[3]d
  }

  secrets = [{
    name  = "REMOTE_TOKEN"
    type  = "env"
    value = "literal-token"
  }]

  sync_enabled  = false
  sync_interval = 0
  sync_paths = [{
    access_key_id = semaphore_ex_project_key.sync_fixture.id
    mount         = "secret"
    path          = "applications/acceptance"
    field         = "token"
    prefix        = "EXAMPLE_"
    separator     = "_"
  }]
}

resource "semaphore_ex_project_key" "sync_fixture" {
  project_id = %[2]d
  name       = "sync-fixture-%[1]s"
  none       = {}
}
`, nameSuffix, projectID, storageID)
}

func testAccProjectEnvironmentRemoteReferenceConfig(nameSuffix string, projectID, storageID int64) string {
	return fmt.Sprintf(`
resource "semaphore_ex_project_environment" "test" {
  project_id = %[2]d
  name       = "Test %[1]s"

  secret_storage = {
    id         = %[3]d
    key_prefix = "terraform-"
  }

  secrets = [{
    name       = "REMOTE_TOKEN"
    type       = "env"
    storage_id = %[3]d
    mount      = "secret"
    path       = "applications/acceptance"
    version    = 0
    field      = "token"
  }]
}
`, nameSuffix, projectID, storageID)
}
