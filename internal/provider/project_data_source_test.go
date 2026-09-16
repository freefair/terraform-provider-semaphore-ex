package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func testAccProjectDataSourceConfigByID() string {
	return `
resource "semaphore_ex_project" "test" {
  name       = "Project 1"
  alert      = true
  alert_chat = "slack"
}

data "semaphore_ex_project" "test" {
  id = semaphore_ex_project.test.id
}`
}

func testAccProjectDataSourceConfigByName() string {
	return `
resource "semaphore_ex_project" "test" {
  name = "Test Project"
}

data "semaphore_ex_project" "test" {
  name = semaphore_ex_project.test.name
}`
}

func TestAcc_ProjectDataSource_basicID(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccProjectDataSourceConfigByID(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.semaphore_ex_project.test", "name", "Project 1"),
					resource.TestCheckResourceAttr("data.semaphore_ex_project.test", "alert", "true"),
					resource.TestCheckResourceAttr("data.semaphore_ex_project.test", "alert_chat", "slack"),
					resource.TestCheckResourceAttr("data.semaphore_ex_project.test", "max_parallel_tasks", "0"),
					resource.TestCheckResourceAttrSet("data.semaphore_ex_project.test", "created"),
					resource.TestCheckResourceAttrSet("data.semaphore_ex_project.test", "id"),
				),
			},
		},
	})
}

func TestAcc_ProjectDataSource_basicName(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccProjectDataSourceConfigByName(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.semaphore_ex_project.test", "name", "Test Project"),
					resource.TestCheckResourceAttr("data.semaphore_ex_project.test", "alert", "false"),
					resource.TestCheckResourceAttr("data.semaphore_ex_project.test", "max_parallel_tasks", "0"),
					resource.TestCheckResourceAttrSet("data.semaphore_ex_project.test", "created"),
					resource.TestCheckResourceAttrSet("data.semaphore_ex_project.test", "id"),
				),
			},
		},
	})
}
