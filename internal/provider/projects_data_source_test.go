package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func testAccProjectsDataSourceConfig() string {
	return `
resource "semaphore_ex_project" "project1" {
  name = "Project 1"
}
resource "semaphore_ex_project" "project2" {
  name  = "Project 2"
  alert = true
}
data "semaphore_ex_projects" "test" {
  depends_on = [semaphore_ex_project.project1]
}`
}

func TestAcc_ProjectsDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccProjectsDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.semaphore_ex_projects.test", "projects.#", "2"),
					resource.TestCheckResourceAttr("data.semaphore_ex_projects.test", "projects.0.name", "Project 1"),
					resource.TestCheckResourceAttr("data.semaphore_ex_projects.test", "projects.1.name", "Project 2"),
					resource.TestCheckResourceAttr("data.semaphore_ex_projects.test", "projects.1.alert", "true"),
				),
			},
		},
	})
}
