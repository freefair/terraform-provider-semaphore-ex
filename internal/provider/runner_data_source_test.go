package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func testAccRunnerDataSourceConfigByID() string {
	return `
resource "semaphore_ex_runner" "test" {
  name               = "Test Global Runner"
  max_parallel_tasks = 2
  tags               = ["linux"]
}

data "semaphore_ex_runner" "test" {
  id         = semaphore_ex_runner.test.id
  depends_on = [semaphore_ex_runner.test]
}`
}

func testAccRunnerDataSourceConfigByName() string {
	return `
resource "semaphore_ex_runner" "test" {
  name               = "Test Global Runner"
  max_parallel_tasks = 2
  tags               = ["linux"]
}

data "semaphore_ex_runner" "test" {
  name       = "Test Global Runner"
  depends_on = [semaphore_ex_runner.test]
}`
}

func TestAcc_RunnerDataSource_basicID(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRunnerDataSourceConfigByID(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.semaphore_ex_runner.test", "name", "Test Global Runner"),
					resource.TestCheckResourceAttr("data.semaphore_ex_runner.test", "max_parallel_tasks", "2"),
					resource.TestCheckResourceAttr("data.semaphore_ex_runner.test", "tags.#", "1"),
					resource.TestCheckTypeSetElemAttr("data.semaphore_ex_runner.test", "tags.*", "linux"),
					resource.TestCheckResourceAttrSet("data.semaphore_ex_runner.test", "id"),
				),
			},
		},
	})
}

func TestAcc_RunnerDataSource_basicName(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRunnerDataSourceConfigByName(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.semaphore_ex_runner.test", "name", "Test Global Runner"),
					resource.TestCheckResourceAttr("data.semaphore_ex_runner.test", "max_parallel_tasks", "2"),
					resource.TestCheckResourceAttr("data.semaphore_ex_runner.test", "tags.#", "1"),
					resource.TestCheckTypeSetElemAttr("data.semaphore_ex_runner.test", "tags.*", "linux"),
					resource.TestCheckResourceAttrSet("data.semaphore_ex_runner.test", "id"),
				),
			},
		},
	})
}
