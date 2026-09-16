package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func testAccProjectUserDataSourceConfig() string {
	return `
resource "semaphore_ex_project" "test" {
  name = "Project 1"
}

resource "semaphore_ex_user" "test" {
  username = "test"
  name = "test name"
  email = "test@example.com"
}

resource "semaphore_ex_project_user" "test" {
  project_id = semaphore_ex_project.test.id
  user_id = semaphore_ex_user.test.id
  role = "task_runner"
}

data "semaphore_ex_project_user" "test" {
  project_id = semaphore_ex_project.test.id
  user_id = semaphore_ex_user.test.id
  depends_on = [semaphore_ex_project_user.test]
}`
}

func TestAcc_ProjectUserDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccProjectUserDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.semaphore_ex_project_user.test", "username", "test"),
					resource.TestCheckResourceAttr("data.semaphore_ex_project_user.test", "role", "task_runner"),
					resource.TestCheckResourceAttr("data.semaphore_ex_project_user.test", "name", "test name"),
				),
			},
		},
	})
}
