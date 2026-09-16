package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func testAccProjectViewDataSourceConfigByID() string {
	return `
resource "semaphore_ex_project" "test" {
  name = "Test Project"
}

resource "semaphore_ex_project_view" "test" {
  project_id = semaphore_ex_project.test.id
  title      = "Test"
  position   = 1
}

data "semaphore_ex_project_view" "test" {
  project_id = semaphore_ex_project.test.id
  id         = semaphore_ex_project_view.test.id
}`
}

func testAccProjectViewDataSourceConfigByName() string {
	return `
resource "semaphore_ex_project" "test" {
  name = "Test Project"
}

resource "semaphore_ex_project_view" "test" {
  project_id = semaphore_ex_project.test.id
  title      = "Title"
  position   = 3
}

data "semaphore_ex_project_view" "test" {
  project_id = semaphore_ex_project.test.id
  title      = "Title"
  depends_on = [semaphore_ex_project_view.test]
}`
}

func TestAcc_ProjectViewDataSource_basicID(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectViewDataSourceConfigByID(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.semaphore_ex_project_view.test", "title", "Test"),
					resource.TestCheckResourceAttr("data.semaphore_ex_project_view.test", "position", "1"),
					resource.TestCheckResourceAttrSet("data.semaphore_ex_project_view.test", "id"),
					resource.TestCheckResourceAttrSet("data.semaphore_ex_project_view.test", "project_id"),
				),
			},
		},
	})
}

func TestAcc_ProjectViewDataSource_basicName(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectViewDataSourceConfigByName(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.semaphore_ex_project_view.test", "title", "Title"),
					resource.TestCheckResourceAttr("data.semaphore_ex_project_view.test", "position", "3"),
					resource.TestCheckResourceAttrSet("data.semaphore_ex_project_view.test", "id"),
					resource.TestCheckResourceAttrSet("data.semaphore_ex_project_view.test", "project_id"),
				),
			},
		},
	})
}
