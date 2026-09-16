package provider

import (
	"fmt"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client/project"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func testAccProjectExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		if rs.Primary.Attributes["id"] == "" {
			return fmt.Errorf("no ID is set")
		}

		id, err := strconv.ParseInt(rs.Primary.Attributes["id"], 10, 64)
		if err != nil {
			return err
		}

		_, err = testClient().Project.GetProjectProjectID(&project.GetProjectProjectIDParams{ProjectID: id}, nil)
		return err
	}
}

func testAccProjectConfig(projectNameSuffix string, projectExtras string) string {
	return fmt.Sprintf(`
resource "semaphore_ex_project" "test" {
  name = "test-%[1]s"
  %[2]s
}`, projectNameSuffix, projectExtras)
}

func testAccProjectImportID(n string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return "", fmt.Errorf("not found: %s", n)
		}

		return fmt.Sprintf("project/%s", rs.Primary.Attributes["id"]), nil
	}
}

func TestAcc_ProjectResource_basic(t *testing.T) {
	projectNameSuffix := acctest.RandString(8)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccProjectConfig(projectNameSuffix, `alert = false
max_parallel_tasks = 0`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccProjectExists("semaphore_ex_project.test"),
					resource.TestCheckResourceAttr("semaphore_ex_project.test", "name", fmt.Sprintf("test-%s", projectNameSuffix)),
					resource.TestCheckResourceAttr("semaphore_ex_project.test", "alert", "false"),
					resource.TestCheckResourceAttr("semaphore_ex_project.test", "max_parallel_tasks", "0"),
					resource.TestCheckResourceAttrSet("semaphore_ex_project.test", "id"),
					resource.TestCheckResourceAttrSet("semaphore_ex_project.test", "created"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "semaphore_ex_project.test",
				ImportState:       true,
				ImportStateVerify: true,
				// API returns different timestamp format between create and read, so just ignore it
				ImportStateVerifyIgnore: []string{"created"},
				ImportStateIdFunc:       testAccProjectImportID("semaphore_ex_project.test"),
			},
			// Update and Read testing
			{
				Config: testAccProjectConfig(projectNameSuffix, `alert = true
max_parallel_tasks = 2
alert_chat = "testing"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("semaphore_ex_project.test", "name", fmt.Sprintf("test-%s", projectNameSuffix)),
					resource.TestCheckResourceAttr("semaphore_ex_project.test", "alert", "true"),
					resource.TestCheckResourceAttr("semaphore_ex_project.test", "alert_chat", "testing"),
					resource.TestCheckResourceAttr("semaphore_ex_project.test", "max_parallel_tasks", "2"),
				),
			},
		},
	})
}
