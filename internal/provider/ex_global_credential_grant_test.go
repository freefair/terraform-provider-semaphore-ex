package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestEXGlobalCredentialGrantOperationsRoundTrip(t *testing.T) {
	operations, diagnostics := types.SetValueFrom(context.Background(), types.StringType, []string{"reference", "consume"})
	if diagnostics.HasError() {
		t.Fatal(diagnostics.Errors())
	}
	mask, err := exGlobalCredentialGrantOperations(context.Background(), operations)
	if err != nil || mask != 3 {
		t.Fatalf("mask = %d, error = %v", mask, err)
	}
	decoded, err := exGlobalCredentialGrantOperationSet(context.Background(), mask)
	if err != nil {
		t.Fatal(err)
	}
	var values []string
	if diagnostics = decoded.ElementsAs(context.Background(), &values, false); diagnostics.HasError() || len(values) != 2 {
		t.Fatalf("operations = %#v, diagnostics = %#v", values, diagnostics)
	}
}

func TestEXGlobalCredentialGrantRequiresPriorRevision(t *testing.T) {
	if err := exGlobalCredentialGrantRequireRevision(exGlobalCredentialGrantModel{Revision: types.Int64Value(0)}); err == nil {
		t.Fatal("expected missing revision to be rejected")
	}
	if err := exGlobalCredentialGrantRequireRevision(exGlobalCredentialGrantModel{Revision: types.Int64Value(2)}); err != nil {
		t.Fatalf("valid revision rejected: %v", err)
	}
}

func TestAcc_EXGlobalCredentialGrant(t *testing.T) {
	name := "acceptance-grant-" + acctest.RandString(8)
	config := fmt.Sprintf(`
resource "semaphore_ex_project" "test" { name = %q }
resource "semaphore_ex_global_credential" "test" {
  type             = "string"
  display_name     = %q
  value_wo         = "acceptance-grant-value"
  value_wo_version = 1
}
resource "semaphore_ex_global_credential_grant" "test" {
  credential_id = semaphore_ex_global_credential.test.id
  project_id    = semaphore_ex_project.test.id
  operations    = ["reference"]
}
data "semaphore_ex_global_credential_grant" "test" {
  credential_id = semaphore_ex_global_credential.test.id
  id            = semaphore_ex_global_credential_grant.test.id
}`, name, name)
	updated := fmt.Sprintf(`
resource "semaphore_ex_project" "test" { name = %q }
resource "semaphore_ex_global_credential" "test" {
  type             = "string"
  display_name     = %q
  value_wo         = "acceptance-grant-value"
  value_wo_version = 1
}
resource "semaphore_ex_global_credential_grant" "test" {
  credential_id = semaphore_ex_global_credential.test.id
  project_id    = semaphore_ex_project.test.id
  operations    = ["reference", "consume"]
  status        = "revoked"
}
data "semaphore_ex_global_credential_grant" "test" {
  credential_id = semaphore_ex_global_credential.test.id
  id            = semaphore_ex_global_credential_grant.test.id
}`, name, name)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttrSet("semaphore_ex_global_credential_grant.test", "id"), resource.TestCheckResourceAttrSet("data.semaphore_ex_global_credential_grant.test", "project_id"), resource.TestCheckResourceAttr("semaphore_ex_global_credential_grant.test", "operations.#", "1"), resource.TestCheckResourceAttrSet("semaphore_ex_global_credential_grant.test", "revision"))},
		{ResourceName: "semaphore_ex_global_credential_grant.test", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: func(s *terraform.State) (string, error) {
			r := s.RootModule().Resources["semaphore_ex_global_credential_grant.test"]
			return fmt.Sprintf("credential/%s/grant/%s", r.Primary.Attributes["credential_id"], r.Primary.ID), nil
		}},
		{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_global_credential_grant.test", "status", "revoked"), resource.TestCheckResourceAttr("semaphore_ex_global_credential_grant.test", "operations.#", "2"))},
	}})
}
