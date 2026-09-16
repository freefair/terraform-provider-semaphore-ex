package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func testAccProjectScheduleDataSourceConfigByID() string {
	return `
resource "semaphore_ex_project" "test" {
  name = "Project"
}

resource "semaphore_ex_project_key" "test" {
  project_id = semaphore_ex_project.test.id
  name       = "None"
  none       = {}
}

resource "semaphore_ex_project_repository" "test" {
  project_id = semaphore_ex_project.test.id
  name       = "Repo"
  url        = "git@github.com:example/test.git"
  branch     = "main"
  ssh_key_id = semaphore_ex_project_key.test.id
}

resource "semaphore_ex_project_inventory" "test" {
  project_id = semaphore_ex_project.test.id
  name       = "Inventory"
  ssh_key_id = semaphore_ex_project_key.test.id
  file = {
    path          = "path/to/inventory"
    repository_id = semaphore_ex_project_repository.test.id
  }
}

resource "semaphore_ex_project_environment" "test" {
  project_id = semaphore_ex_project.test.id
  name       = "Environment"
  secrets = [{
    name  = "SECRET_ONE"
    type  = "var"
    value = "VALUE_ONE"
  }]
}

# Task Template
resource "semaphore_ex_project_template" "test" {
  project_id     = semaphore_ex_project.test.id
  environment_id = semaphore_ex_project_environment.test.id
  inventory_id   = semaphore_ex_project_inventory.test.id
  repository_id  = semaphore_ex_project_repository.test.id
  name           = "Template"
  playbook       = "playbook.yml"
  description    = "Description"
}

resource "semaphore_ex_project_schedule" "test" {
  project_id  = semaphore_ex_project.test.id
  template_id = semaphore_ex_project_template.test.id
  name        = "Test Schedule"
  cron_format = "0 0 * * *"
  enabled     = true
}

data "semaphore_ex_project_schedule" "test" {
  project_id = semaphore_ex_project.test.id
  id         = semaphore_ex_project_schedule.test.id
}`
}

func TestAcc_ProjectScheduleDataSource_basicID(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectScheduleDataSourceConfigByID(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.semaphore_ex_project_schedule.test", "name", "Test Schedule"),
					resource.TestCheckResourceAttr("data.semaphore_ex_project_schedule.test", "cron_format", "0 0 * * *"),
					resource.TestCheckResourceAttr("data.semaphore_ex_project_schedule.test", "enabled", "true"),
					resource.TestCheckResourceAttrSet("data.semaphore_ex_project_schedule.test", "id"),
					resource.TestCheckResourceAttrSet("data.semaphore_ex_project_schedule.test", "project_id"),
				),
			},
		},
	})
}
