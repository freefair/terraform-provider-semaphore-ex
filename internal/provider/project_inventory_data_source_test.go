package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func testAccProjectInventoryDataSourceConfigByID() string {
	return `
resource "semaphore_ex_project" "test" {
  name = "Project 1"
}

resource "semaphore_ex_project_key" "test" {
  project_id = semaphore_ex_project.test.id
  name       = "None"
  none       = {}
}

resource "semaphore_ex_project_inventory" "test" {
  project_id = semaphore_ex_project.test.id
  name       = "Test Inventory"
  ssh_key_id = semaphore_ex_project_key.test.id
  static = {
	inventory = <<-EOT
      [all]
      hostname
    EOT
  }
}

data "semaphore_ex_project_inventory" "test" {
  project_id = semaphore_ex_project.test.id
  id         = semaphore_ex_project_inventory.test.id
  depends_on = [semaphore_ex_project_inventory.test]
}`
}

func testAccProjectInventoryDataSourceConfigByName() string {
	return `
resource "semaphore_ex_project" "test" {
  name = "Project 1"
}

resource "semaphore_ex_project_key" "test" {
  project_id = semaphore_ex_project.test.id
  name       = "None"
  none       = {}
}

resource "semaphore_ex_project_inventory" "test" {
  project_id = semaphore_ex_project.test.id
  name       = "Test Inventory"
  ssh_key_id = semaphore_ex_project_key.test.id
  file = {
    path = "inventory.yml"
  }
}

data "semaphore_ex_project_inventory" "test" {
  project_id = semaphore_ex_project.test.id
  name       = "Test Inventory"
  depends_on = [semaphore_ex_project_inventory.test]
}`
}

func TestAcc_ProjectInventoryDataSource_basicID(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectInventoryDataSourceConfigByID(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.semaphore_ex_project_inventory.test", "name", "Test Inventory"),
					resource.TestCheckResourceAttrSet("data.semaphore_ex_project_inventory.test", "ssh_key_id"),
					resource.TestCheckResourceAttr("data.semaphore_ex_project_inventory.test", "static.%", "2"),
					resource.TestCheckResourceAttr("data.semaphore_ex_project_inventory.test", "static.inventory", "[all]\nhostname\n"),
					resource.TestCheckNoResourceAttr("data.semaphore_ex_project_inventory.test", "file"),
					resource.TestCheckNoResourceAttr("data.semaphore_ex_project_inventory.test", "static_yaml"),
					resource.TestCheckNoResourceAttr("data.semaphore_ex_project_inventory.test", "terraform_workspace"),
				),
			},
		},
	})
}

func TestAcc_ProjectInventoryDataSource_basicName(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectInventoryDataSourceConfigByName(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.semaphore_ex_project_inventory.test", "name", "Test Inventory"),
					resource.TestCheckResourceAttrSet("data.semaphore_ex_project_inventory.test", "ssh_key_id"),
					resource.TestCheckResourceAttr("data.semaphore_ex_project_inventory.test", "file.%", "3"),
					resource.TestCheckResourceAttr("data.semaphore_ex_project_inventory.test", "file.path", "inventory.yml"),
					resource.TestCheckNoResourceAttr("data.semaphore_ex_project_inventory.test", "static"),
					resource.TestCheckNoResourceAttr("data.semaphore_ex_project_inventory.test", "static_yaml"),
					resource.TestCheckNoResourceAttr("data.semaphore_ex_project_inventory.test", "terraform_workspace"),
				),
			},
		},
	})
}
