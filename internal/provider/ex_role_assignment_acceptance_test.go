package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func testAccGlobalRoleAssignmentImportID(name string) resource.ImportStateIdFunc {
	return func(state *terraform.State) (string, error) {
		value, ok := state.RootModule().Resources[name]
		if !ok {
			return "", fmt.Errorf("resource %s not found", name)
		}
		return fmt.Sprintf("user/%s/assignment/%s", value.Primary.Attributes["user_id"], value.Primary.Attributes["id"]), nil
	}
}

func TestAcc_EXGlobalRoleAssignment(t *testing.T) {
	suffix := acctest.RandString(8)
	config := fmt.Sprintf(`
resource "semaphore_ex_global_role" "test" {
  name = "assignment-%[1]s"
  project_permissions = []
  global_permissions = ["global.audit.read"]
}
resource "semaphore_ex_user" "test" {
  username = "assignment-%[1]s"
  name = "Assignment %[1]s"
  email = "assignment-%[1]s@example.test"
}
resource "semaphore_ex_global_role_assignment" "test" {
  user_id = semaphore_ex_user.test.id
  role_id = semaphore_ex_global_role.test.id
}`, suffix)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttrPair("semaphore_ex_global_role_assignment.test", "user_id", "semaphore_ex_user.test", "id"), resource.TestCheckResourceAttrPair("semaphore_ex_global_role_assignment.test", "role_id", "semaphore_ex_global_role.test", "id"), resource.TestCheckResourceAttrSet("semaphore_ex_global_role_assignment.test", "id"), resource.TestCheckResourceAttrSet("semaphore_ex_global_role_assignment.test", "revision"))},
		{ResourceName: "semaphore_ex_global_role_assignment.test", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: testAccGlobalRoleAssignmentImportID("semaphore_ex_global_role_assignment.test")},
	}})
}
