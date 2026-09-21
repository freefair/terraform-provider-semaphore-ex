package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client/project"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Terraform owns address moves and forgetting. Exercise its real CLI rather
// than emulating those operations inside provider methods.
func TestAcc_ProviderLifecycleCLI(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance test requires TF_ACC")
	}
	testAccPreCheck(t)
	ctx := context.Background()
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	require.NoError(t, os.Mkdir(bin, 0700))
	build := exec.CommandContext(ctx, "go", "build", "-o", filepath.Join(bin, "terraform-provider-semaphore-ex"), "../..")
	output, err := build.CombinedOutput()
	require.NoError(t, err, "provider build: %s", output)
	cliConfig := filepath.Join(dir, "terraform.rc")
	require.NoError(t, os.WriteFile(cliConfig, []byte(fmt.Sprintf("provider_installation {\n dev_overrides {\n  \"freefair/semaphore-ex\" = %q\n }\n direct {}\n}\n", bin)), 0600))
	run := func(args ...string) []byte {
		t.Helper()
		c := exec.CommandContext(ctx, "terraform", args...)
		c.Dir = dir
		for _, entry := range os.Environ() {
			if !strings.HasPrefix(entry, "TF_CLI_CONFIG_FILE=") && !strings.HasPrefix(entry, "TF_REATTACH_PROVIDERS=") && !strings.HasPrefix(entry, "TF_CLI_ARGS") && !strings.HasPrefix(entry, "TF_DATA_DIR=") {
				c.Env = append(c.Env, entry)
			}
		}
		c.Env = append(c.Env, "TF_CLI_CONFIG_FILE="+cliConfig, "TF_INPUT=0", "CHECKPOINT_DISABLE=1")
		output, err := c.CombinedOutput()
		require.NoError(t, err, "terraform %v: %s", args, output)
		return output
	}
	write := func(body string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.tf"), []byte("terraform {\n required_providers {\n semaphore = { source = \"freefair/semaphore-ex\" }\n }\n}\nprovider \"semaphore\" {}\n"+body), 0600))
	}
	apply := func(extra ...string) {
		t.Helper()
		args := append([]string{"plan", "-no-color", "-out=change.tfplan"}, extra...)
		run(args...)
		run("apply", "-no-color", "change.tfplan")
	}
	id := func(address string) int64 {
		t.Helper()
		var state struct {
			Values struct {
				RootModule struct {
					Resources []struct {
						Address string
						Values  struct{ ID int64 }
					} `json:"resources"`
				} `json:"root_module"`
			} `json:"values"`
		}
		require.NoError(t, json.Unmarshal(run("show", "-json"), &state))
		for _, r := range state.Values.RootModule.Resources {
			if r.Address == address {
				return r.Values.ID
			}
		}
		t.Fatalf("missing resource %s", address)
		return 0
	}
	exists := func(objectID int64) {
		t.Helper()
		_, err := testClient().Project.GetProjectProjectID(&project.GetProjectProjectIDParams{ProjectID: objectID}, nil)
		require.NoError(t, err)
	}
	absent := func(objectID int64) {
		t.Helper()
		_, err := testClient().Project.GetProjectProjectID(&project.GetProjectProjectIDParams{ProjectID: objectID}, nil)
		require.True(t, resourceNotFound(err), "expected absent object: %v", err)
	}
	name := "lifecycle-" + acctest.RandString(8)
	config := func(label, title string) string {
		return fmt.Sprintf("resource \"semaphore_ex_project\" %q {\n name = %q\n}\n", label, title)
	}
	write(config("original", name))
	run("validate", "-no-color")
	apply()
	original := id("semaphore_ex_project.original")
	apply("-refresh-only")
	assert.Equal(t, original, id("semaphore_ex_project.original"))
	write(config("renamed", name) + "moved {\n from = semaphore_ex_project.original\n to = semaphore_ex_project.renamed\n}\n")
	apply()
	assert.Equal(t, original, id("semaphore_ex_project.renamed"))
	run("state", "mv", "semaphore_ex_project.renamed", "semaphore_ex_project.final")
	write(config("final", name))
	apply()
	assert.Equal(t, original, id("semaphore_ex_project.final"))
	run("state", "rm", "semaphore_ex_project.final")
	exists(original)
	run("import", "-no-color", "semaphore_ex_project.final", fmt.Sprint(original))
	assert.Equal(t, original, id("semaphore_ex_project.final"))
	apply()
	write("removed {\n from = semaphore_ex_project.final\n lifecycle { destroy = false }\n}\n")
	apply()
	exists(original)
	write(config("final", name) + fmt.Sprintf("import {\n to = semaphore_ex_project.final\n id = \"project/%d\"\n}\n", original))
	apply()
	assert.Equal(t, original, id("semaphore_ex_project.final"))
	write(config("final", name+"-updated"))
	apply("-target=semaphore_ex_project.final")
	assert.Equal(t, original, id("semaphore_ex_project.final"))
	apply("-replace=semaphore_ex_project.final")
	replacement := id("semaphore_ex_project.final")
	assert.NotEqual(t, original, replacement)
	absent(original)
	_, err = testClient().Project.DeleteProjectProjectID(&project.DeleteProjectProjectIDParams{ProjectID: replacement}, nil)
	require.NoError(t, err)
	apply()
	recreated := id("semaphore_ex_project.final")
	assert.NotEqual(t, replacement, recreated)
	apply("-destroy")
	absent(recreated)
}

func TestAcc_LegacyProviderMovedGraph(t *testing.T) {
	suffix := acctest.RandString(8)
	testPassword := acctest.RandString(32)
	t.Setenv("TF_VAR_migration_password", testPassword)
	config := testAccIntegrationAliasDependencyConfig(suffix) + fmt.Sprintf(`
variable "migration_password" {
 type = string
 sensitive = true
}
resource "semaphore_ex_integration_alias" "test" {
 project_id = semaphore_ex_project.test.id
 integration_id = semaphore_ex_project_integration.test.id
}
resource "semaphore_ex_project_view" "test" {
 project_id = semaphore_ex_project.test.id
 title = "Migration"
 position = 1
}
resource "semaphore_ex_project_schedule" "test" {
 project_id = semaphore_ex_project.test.id
 template_id = semaphore_ex_project_template.test.id
 name = "Migration"
 cron_format = "0 0 * * *"
 enabled = false
}
resource "semaphore_ex_user" "test" {
 username = "migration-%s"
 name = "Migration User"
 email = "migration@example.test"
 password = var.migration_password
}
resource "semaphore_ex_project_user" "test" {
 project_id = semaphore_ex_project.test.id
 user_id = semaphore_ex_user.test.id
 role = "guest"
}
resource "semaphore_ex_runner" "test" {
 name = "migration-global-%s"
 active = false
}
resource "semaphore_ex_project_runner" "test" {
 project_id = semaphore_ex_project.test.id
 name = "migration-project-%s"
 active = false
}
resource "semaphore_ex_runner_registration_token" "test" {
 runner_id = semaphore_ex_runner.test.id
}
`, suffix, suffix, suffix)
	old := strings.ReplaceAll(config, "semaphore_ex_", "semaphoreui_")
	kinds := []string{"integration_alias", "project", "project_environment", "project_integration", "project_inventory", "project_key", "project_repository", "project_runner", "project_schedule", "project_template", "project_user", "project_view", "runner", "runner_registration_token", "user"}
	moves := ""
	checks := []plancheck.PlanCheck{}
	for _, kind := range kinds {
		moves += fmt.Sprintf("\nmoved {\n from = semaphoreui_%s.test\n to = semaphore_ex_%s.test\n}\n", kind, kind)
		checks = append(checks, plancheck.ExpectResourceAction("semaphore_ex_"+kind+".test", plancheck.ResourceActionNoop))
	}
	oldIDs := map[string]string{}
	capture := func(s *terraform.State) error {
		for _, kind := range kinds {
			r := s.RootModule().Resources["semaphoreui_"+kind+".test"]
			if r == nil {
				return fmt.Errorf("missing legacy %s", kind)
			}
			oldIDs[kind] = r.Primary.Attributes["id"]
		}
		return nil
	}
	verify := func(s *terraform.State) error {
		for kind, want := range oldIDs {
			r := s.RootModule().Resources["semaphore_ex_"+kind+".test"]
			if r == nil || r.Primary.Attributes["id"] != want {
				return fmt.Errorf("identity changed for %s", kind)
			}
		}
		if s.RootModule().Resources["semaphore_ex_user.test"].Primary.Attributes["password"] != testPassword {
			return fmt.Errorf("password was not preserved")
		}
		return nil
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, ExternalProviders: map[string]resource.ExternalProvider{"semaphoreui": {Source: "semaphoreui/semaphore", VersionConstraint: "= 0.3.9"}}, Steps: []resource.TestStep{
		{Config: old, Check: capture},
		{Config: config + moves, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: checks}, Check: verify},
	}})
}
