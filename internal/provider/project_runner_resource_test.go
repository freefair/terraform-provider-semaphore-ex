package provider

import (
	"fmt"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client/project"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client/runner"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// testAccPreCheckProjectRunner skips the test when the SemaphoreUI server under
// test does not allow project-scoped runners. They are a paid-plan feature: the
// community edition (used by CI) answers project runner creation with a 403
// "Your plan does not allow adding more runners." Global runners are unaffected.
func testAccPreCheckProjectRunner(t *testing.T) {
	testAccPreCheck(t)

	client := testClient()
	proj, err := client.Project.PostProjects(&project.PostProjectsParams{
		Project: &models.ProjectRequest{Name: "runner-precheck-" + acctest.RandString(8)},
	}, nil)
	if err != nil {
		// Could not probe; let the test run and surface any real error.
		return
	}
	projectID := proj.Payload.ID
	defer func() {
		_, _ = client.Project.DeleteProjectProjectID(&project.DeleteProjectProjectIDParams{
			ProjectID: projectID,
		}, nil)
	}()

	created, err := client.Runner.PostProjectProjectIDRunners(&runner.PostProjectProjectIDRunnersParams{
		ProjectID: projectID,
		Runner:    &models.RunnerRequest{ProjectID: projectID, Name: "precheck"},
	}, nil)
	if err != nil {
		if strings.Contains(err.Error(), "403") {
			t.Skip("skipping: SemaphoreUI plan does not allow project runners")
		}
		return
	}
	_, _ = client.Runner.DeleteProjectProjectIDRunnersRunnerID(&runner.DeleteProjectProjectIDRunnersRunnerIDParams{
		ProjectID: projectID,
		RunnerID:  created.Payload.ID,
	}, nil)
}

func testAccProjectRunnerExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		if rs.Primary.Attributes["id"] == "" {
			return fmt.Errorf("no ID is set")
		}
		if rs.Primary.Attributes["project_id"] == "" {
			return fmt.Errorf("no ProjectID is set")
		}

		id, _ := strconv.ParseInt(rs.Primary.Attributes["id"], 10, 64)
		projectId, _ := strconv.ParseInt(rs.Primary.Attributes["project_id"], 10, 64)

		_, err := testClient().Runner.GetProjectProjectIDRunnersRunnerID(&runner.GetProjectProjectIDRunnersRunnerIDParams{
			ProjectID: projectId,
			RunnerID:  id,
		}, nil)
		if err != nil {
			return fmt.Errorf("error reading project runner: %s", err.Error())
		}

		return nil
	}
}

func testAccProjectRunnerProjectConfig(nameSuffix string) string {
	return fmt.Sprintf(`
resource "semaphore_ex_project" "test" {
  name = "test-%[1]s"
}
`, nameSuffix)
}

func testAccProjectRunnerConfig(nameSuffix string, maxParallelTasks int, active bool, isDefault bool, tags string) string {
	return fmt.Sprintf(`
%[1]s
resource "semaphore_ex_project_runner" "test" {
  project_id         = semaphore_ex_project.test.id
  name               = "Test %[2]s"
  max_parallel_tasks = %[3]d
  active             = %[4]t
  is_default         = %[5]t
  tags               = %[6]s
}`, testAccProjectRunnerProjectConfig(nameSuffix), nameSuffix, maxParallelTasks, active, isDefault, tags)
}

func testAccProjectRunnerImportID(n string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return "", fmt.Errorf("not found: %s", n)
		}

		return fmt.Sprintf("project/%[1]s/runner/%[2]s", rs.Primary.Attributes["project_id"], rs.Primary.Attributes["id"]), nil
	}
}

func TestAcc_ProjectRunnerResource_basic(t *testing.T) {
	nameSuffix := acctest.RandString(8)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckProjectRunner(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccProjectRunnerConfig(nameSuffix, 1, false, false, `["linux", "production"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccProjectRunnerExists("semaphore_ex_project_runner.test"),
					resource.TestCheckResourceAttr("semaphore_ex_project_runner.test", "name", fmt.Sprintf("Test %s", nameSuffix)),
					resource.TestCheckResourceAttr("semaphore_ex_project_runner.test", "max_parallel_tasks", "1"),
					resource.TestCheckResourceAttr("semaphore_ex_project_runner.test", "active", "false"),
					resource.TestCheckResourceAttr("semaphore_ex_project_runner.test", "is_default", "false"),
					resource.TestCheckResourceAttr("semaphore_ex_project_runner.test", "tags.#", "2"),
					resource.TestCheckTypeSetElemAttr("semaphore_ex_project_runner.test", "tags.*", "linux"),
					resource.TestCheckTypeSetElemAttr("semaphore_ex_project_runner.test", "tags.*", "production"),
					resource.TestCheckResourceAttrSet("semaphore_ex_project_runner.test", "id"),
					resource.TestCheckResourceAttrSet("semaphore_ex_project_runner.test", "project_id"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "semaphore_ex_project_runner.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testAccProjectRunnerImportID("semaphore_ex_project_runner.test"),
			},
			// Update and Read testing
			{
				Config: testAccProjectRunnerConfig(nameSuffix, 4, false, true, `["windows"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccProjectRunnerExists("semaphore_ex_project_runner.test"),
					resource.TestCheckResourceAttr("semaphore_ex_project_runner.test", "name", fmt.Sprintf("Test %s", nameSuffix)),
					resource.TestCheckResourceAttr("semaphore_ex_project_runner.test", "max_parallel_tasks", "4"),
					resource.TestCheckResourceAttr("semaphore_ex_project_runner.test", "active", "false"),
					resource.TestCheckResourceAttr("semaphore_ex_project_runner.test", "is_default", "true"),
					resource.TestCheckResourceAttr("semaphore_ex_project_runner.test", "tags.#", "1"),
					resource.TestCheckTypeSetElemAttr("semaphore_ex_project_runner.test", "tags.*", "windows"),
				),
			},
			// Delete testing
			{
				Config: testAccProjectRunnerProjectConfig(nameSuffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccResourceNotExists("semaphore_ex_project_runner.test"),
				),
			},
		},
	})
}

func TestAcc_ProjectRunnerResource_importedSecurePolicySurvivesUpdate(t *testing.T) {
	nameSuffix := acctest.RandString(8)
	config := func(parallelism int, policy string) string {
		policyLine := ""
		if policy != "" {
			policyLine = "  registration_policy = \"" + policy + "\"\n"
		}
		return fmt.Sprintf(`%sresource "semaphore_ex_project_runner" "secure" {
  project_id         = semaphore_ex_project.test.id
  name               = "Secure %s"
  max_parallel_tasks = %d
  active             = false
%s}`, testAccProjectRunnerProjectConfig(nameSuffix), nameSuffix, parallelism, policyLine)
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheckProjectRunner(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config(1, "secure"), Check: resource.TestCheckResourceAttr("semaphore_ex_project_runner.secure", "registration_policy", "secure")},
			{ResourceName: "semaphore_ex_project_runner.secure", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: testAccProjectRunnerImportID("semaphore_ex_project_runner.secure")},
			{Config: config(2, ""), Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("semaphore_ex_project_runner.secure", "registration_policy", "secure"),
				resource.TestCheckResourceAttr("semaphore_ex_project_runner.secure", "max_parallel_tasks", "2"),
			)},
		},
	})
}
