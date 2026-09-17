package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func exGlobalRoleConfig(name string, projectPermission string, globalPermission string) string {
	return fmt.Sprintf(`
resource "semaphore_ex_global_role" "test" {
  name = %q
  project_permissions = [%q]
  global_permissions = [%q]
}`, name, projectPermission, globalPermission)
}

func exProjectRoleConfig(name string, permission string) string {
	return fmt.Sprintf(`
resource "semaphore_ex_project" "test" { name = %q }
resource "semaphore_ex_project_role" "test" {
  project_id = semaphore_ex_project.test.id
  name = %q
  project_permissions = [%q]
}`, name, name, permission)
}

func exRoleImportID(resourceName string, project bool) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("resource %s not found", resourceName)
		}
		if project {
			return fmt.Sprintf("project/%s/role/%s", rs.Primary.Attributes["project_id"], rs.Primary.Attributes["id"]), nil
		}
		return rs.Primary.Attributes["id"], nil
	}
}

func TestAcc_EXGlobalRole(t *testing.T) {
	name := "acceptance-global-" + acctest.RandString(8)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: exGlobalRoleConfig(name, "project.resources.view", "global.audit.read"), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_global_role.test", "name", name), resource.TestCheckResourceAttr("semaphore_ex_global_role.test", "project_permissions.#", "1"), resource.TestCheckResourceAttr("semaphore_ex_global_role.test", "global_permissions.#", "1"), resource.TestCheckResourceAttrSet("semaphore_ex_global_role.test", "id"), resource.TestCheckResourceAttrSet("semaphore_ex_global_role.test", "revision"))},
		{ResourceName: "semaphore_ex_global_role.test", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: exRoleImportID("semaphore_ex_global_role.test", false)},
		{Config: exGlobalRoleConfig(name+"-updated", "workflow.view", "global.users.manage"), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_global_role.test", "name", name+"-updated"), resource.TestCheckResourceAttr("semaphore_ex_global_role.test", "project_permissions.0", "workflow.view"), resource.TestCheckResourceAttr("semaphore_ex_global_role.test", "global_permissions.0", "global.users.manage"))},
	}})
}

func TestAcc_EXProjectRole(t *testing.T) {
	name := "acceptance-project-" + acctest.RandString(8)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: exProjectRoleConfig(name, "project.resources.view"), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_project_role.test", "name", name), resource.TestCheckResourceAttr("semaphore_ex_project_role.test", "project_permissions.#", "1"), resource.TestCheckResourceAttrSet("semaphore_ex_project_role.test", "id"))},
		{ResourceName: "semaphore_ex_project_role.test", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: exRoleImportID("semaphore_ex_project_role.test", true)},
		{Config: exProjectRoleConfig(name+"-updated", "workflow.view"), Check: resource.TestCheckResourceAttr("semaphore_ex_project_role.test", "name", name+"-updated")},
	}})
}

func TestAcc_EXProjectRoleMembership(t *testing.T) {
	name := "acceptance-membership-" + acctest.RandString(8)
	config := fmt.Sprintf(`
resource "semaphore_ex_project" "test" { name = %q }
resource "semaphore_ex_project_role" "test" {
  project_id = semaphore_ex_project.test.id
  name = "Custom member"
  project_permissions = ["project.resources.view"]
}
resource "semaphore_ex_user" "test" {
  username = %q
  name = "Custom Member"
  email = "custom-member@example.test"
}
resource "semaphore_ex_project_user" "test" {
  project_id = semaphore_ex_project.test.id
  user_id = semaphore_ex_user.test.id
  role = semaphore_ex_project_role.test.id
}`, name, "custom-member-"+acctest.RandString(8))
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: config, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttrPair("semaphore_ex_project_user.test", "role", "semaphore_ex_project_role.test", "id"), resource.TestCheckResourceAttrPair("semaphore_ex_project_user.test", "role_id", "semaphore_ex_project_role.test", "id"))}}})
}
