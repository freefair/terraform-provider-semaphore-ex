package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func testAccProjectRunnerDataSourceConfigByID() string {
	return `
resource "semaphore_ex_project" "test" {
  name = "Project 1"
}

resource "semaphore_ex_project_runner" "test" {
  project_id         = semaphore_ex_project.test.id
  name               = "Test Runner"
  max_parallel_tasks = 2
  tags               = ["linux"]
}

data "semaphore_ex_project_runner" "test" {
  project_id = semaphore_ex_project.test.id
  id         = semaphore_ex_project_runner.test.id
  depends_on = [semaphore_ex_project_runner.test]
}`
}

func testAccProjectRunnerDataSourceConfigByName() string {
	return `
resource "semaphore_ex_project" "test" {
  name = "Project 1"
}

resource "semaphore_ex_project_runner" "test" {
  project_id         = semaphore_ex_project.test.id
  name               = "Test Runner"
  max_parallel_tasks = 2
  tags               = ["linux"]
}

data "semaphore_ex_project_runner" "test" {
  project_id = semaphore_ex_project.test.id
  name       = "Test Runner"
  depends_on = [semaphore_ex_project_runner.test]
}`
}

func TestAcc_ProjectRunnerDataSource_basicID(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckProjectRunner(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectRunnerDataSourceConfigByID(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.semaphore_ex_project_runner.test", "name", "Test Runner"),
					resource.TestCheckResourceAttr("data.semaphore_ex_project_runner.test", "max_parallel_tasks", "2"),
					resource.TestCheckResourceAttr("data.semaphore_ex_project_runner.test", "tags.#", "1"),
					resource.TestCheckTypeSetElemAttr("data.semaphore_ex_project_runner.test", "tags.*", "linux"),
					resource.TestCheckResourceAttrSet("data.semaphore_ex_project_runner.test", "id"),
				),
			},
		},
	})
}

func TestAcc_ProjectRunnerDataSource_basicName(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckProjectRunner(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProjectRunnerDataSourceConfigByName(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.semaphore_ex_project_runner.test", "name", "Test Runner"),
					resource.TestCheckResourceAttr("data.semaphore_ex_project_runner.test", "max_parallel_tasks", "2"),
					resource.TestCheckResourceAttr("data.semaphore_ex_project_runner.test", "tags.#", "1"),
					resource.TestCheckTypeSetElemAttr("data.semaphore_ex_project_runner.test", "tags.*", "linux"),
					resource.TestCheckResourceAttrSet("data.semaphore_ex_project_runner.test", "id"),
				),
			},
		},
	})
}
