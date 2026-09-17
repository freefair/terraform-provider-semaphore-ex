package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func globalCredentialReference(t *testing.T) types.Object {
	t.Helper()
	value, diagnostics := types.ObjectValue(map[string]attr.Type{
		"provider": types.StringType, "provider_id": types.StringType, "mount": types.StringType,
		"path": types.StringType, "version": types.Int64Type, "field": types.StringType,
	}, map[string]attr.Value{
		"provider": types.StringValue("openbao"), "provider_id": types.StringValue("primary"), "mount": types.StringValue("secret"),
		"path": types.StringValue("deploy/token"), "version": types.Int64Value(3), "field": types.StringValue("value"),
	})
	if diagnostics.HasError() {
		t.Fatal(diagnostics.Errors())
	}
	return value
}

func TestEXGlobalCredentialMaterialUsesWriteOnlyValueAndExternalReference(t *testing.T) {
	ctx := context.Background()
	material, err := exGlobalCredentialMaterial(ctx, exGlobalCredentialModel{ValueWO: types.StringValue("write-only")}, true)
	if err != nil || material["string_value"] != "write-only" {
		t.Fatalf("write-only material = %#v, %v", material, err)
	}
	material, err = exGlobalCredentialMaterial(ctx, exGlobalCredentialModel{ExternalReference: globalCredentialReference(t)}, false)
	if err != nil {
		t.Fatal(err)
	}
	reference, ok := material["external_reference"].(map[string]any)
	if !ok || reference["path"] != "deploy/token" || reference["version"] != int64(3) {
		t.Fatalf("external material = %#v", material)
	}
}

func TestEXGlobalCredentialResponsePreservesRedactedLocalValue(t *testing.T) {
	old := exGlobalCredentialModel{Value: types.StringValue("configured-secret"), ExternalReference: types.ObjectNull(map[string]attr.Type{"provider": types.StringType, "provider_id": types.StringType, "mount": types.StringType, "path": types.StringType, "version": types.Int64Type, "field": types.StringType})}
	next, err := exGlobalCredentialModelFromResponse(context.Background(), old, exGlobalCredentialResponse{ID: 7, Type: "string", DisplayName: "Deploy", Enabled: true, Revision: 4, CurrentVersion: 2, Fingerprint: "fingerprint", MaterialKind: "local_encrypted"})
	if err != nil {
		t.Fatal(err)
	}
	if next.Value.ValueString() != "configured-secret" || next.ID.ValueInt64() != 7 || next.Revision.ValueInt64() != 4 {
		t.Fatalf("state = %#v", next)
	}
}

func TestEXGlobalCredentialReadUsesDetailedEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/global-credentials/7" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(exGlobalCredentialResponse{ID: 7, Type: "string", DisplayName: "Deploy", Enabled: true, Revision: 2, CurrentVersion: 1, Fingerprint: "fingerprint", MaterialKind: "external_reference", ExternalReference: map[string]any{"provider": "openbao", "provider_id": "primary", "mount": "secret", "path": "deploy/token", "version": 3, "field": "value"}})
	}))
	defer server.Close()
	next, err := exGlobalCredentialRead(context.Background(), newEXTestClient(t, server.URL), exGlobalCredentialModel{ID: types.Int64Value(7)})
	if err != nil {
		t.Fatal(err)
	}
	if next.DisplayName.ValueString() != "Deploy" || next.ExternalReference.IsNull() {
		t.Fatalf("state = %#v", next)
	}
}

func TestAcc_EXGlobalCredential(t *testing.T) {
	name := "acceptance-credential-" + acctest.RandString(8)
	config := `
resource "semaphore_ex_global_credential" "test" {
  type             = "string"
  display_name     = "` + name + `"
  value_wo         = "acceptance-write-only-value"
  value_wo_version = 1
}
data "semaphore_ex_global_credential" "test" {
  id = semaphore_ex_global_credential.test.id
}`
	updated := `
resource "semaphore_ex_global_credential" "test" {
  type             = "string"
  display_name     = "` + name + ` updated"
  enabled          = false
  value_wo         = "acceptance-write-only-value"
  value_wo_version = 1
}
data "semaphore_ex_global_credential" "test" {
  id = semaphore_ex_global_credential.test.id
}`
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttrSet("semaphore_ex_global_credential.test", "id"), resource.TestCheckResourceAttr("semaphore_ex_global_credential.test", "type", "string"), resource.TestCheckResourceAttr("data.semaphore_ex_global_credential.test", "display_name", name), resource.TestCheckResourceAttrSet("semaphore_ex_global_credential.test", "revision"))},
		{ResourceName: "semaphore_ex_global_credential.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"value_wo_version"}, ImportStateIdFunc: func(s *terraform.State) (string, error) {
			return s.RootModule().Resources["semaphore_ex_global_credential.test"].Primary.ID, nil
		}},
		{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_global_credential.test", "display_name", name+" updated"), resource.TestCheckResourceAttr("semaphore_ex_global_credential.test", "enabled", "false"))},
	}})
}
