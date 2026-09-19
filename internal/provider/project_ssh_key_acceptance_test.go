package provider

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func testAccSSHKeyPolicyConfig(suffix, policy, selection string) string {
	dependencies := testAccProjectTemplateDependencyConfig(suffix)
	project := fmt.Sprintf(`resource "semaphore_ex_project" "test" {
  name = "test-%s"
}`, suffix)
	configuredProject := fmt.Sprintf(`resource "semaphore_ex_project" "test" {
  name               = "test-%[1]s"
  alert              = true
  max_parallel_tasks = 4
}`, suffix)
	dependencies = strings.Replace(dependencies, project, configuredProject, 1)
	return fmt.Sprintf(`
%[1]s

resource "semaphore_ex_project_generated_ssh_key" "one" {
  project_id = semaphore_ex_project.test.id
  name       = "key-one-%[2]s"
  login      = "deploy-one"
  login_wo_version = 1
  algorithm  = "ed25519"
}

resource "semaphore_ex_project_generated_ssh_key" "two" {
  project_id = semaphore_ex_project.test.id
  name       = "key-two-%[2]s"
  login      = "deploy-two"
  login_wo_version = 1
  algorithm  = "ed25519"
}

%[3]s

resource "semaphore_ex_project_template" "test" {
  project_id      = semaphore_ex_project.test.id
  environment_ids = [semaphore_ex_project_environment.test.id]
  inventory_id    = semaphore_ex_project_inventory.test.id
  repository_id   = semaphore_ex_project_repository.test.id
  name            = "SSH key template %[2]s"
  playbook        = "playbook.yml"
  depends_on      = [semaphore_ex_project_generated_ssh_key.one, semaphore_ex_project_generated_ssh_key.two]
%[4]s
}

data "semaphore_ex_project_template" "test" {
  project_id = semaphore_ex_project.test.id
  id         = semaphore_ex_project_template.test.id
}
`, dependencies, suffix, policy, selection)
}

func testAccSSHKeyPolicyResource(defaults, always string) string {
	return fmt.Sprintf(`
resource "semaphore_ex_project_ssh_key_policy" "test" {
  project_id = semaphore_ex_project.test.id
  depends_on = [semaphore_ex_project_generated_ssh_key.one, semaphore_ex_project_generated_ssh_key.two]
  %[1]s
  %[2]s
}

data "semaphore_ex_project_ssh_key_policy" "test" {
  project_id = semaphore_ex_project.test.id
  depends_on = [semaphore_ex_project_ssh_key_policy.test]
}
`, defaults, always)
}

func testAccTemplateSSHKeySelection(inherit bool, bindings string) string {
	return fmt.Sprintf(`
  ssh_keys = {
    inherit = %t
    %s
  }
`, inherit, bindings)
}

func testAccProjectSSHKeyPolicyImportID(resourceName string) resource.ImportStateIdFunc {
	return func(state *terraform.State) (string, error) {
		policy, found := state.RootModule().Resources[resourceName]
		if !found {
			return "", fmt.Errorf("project SSH key policy %q is missing", resourceName)
		}
		return policy.Primary.Attributes["project_id"], nil
	}
}

func testAccTemplateSSHKeyImportID(resourceName string) resource.ImportStateIdFunc {
	return func(state *terraform.State) (string, error) {
		template, found := state.RootModule().Resources[resourceName]
		if !found {
			return "", fmt.Errorf("project template %q is missing", resourceName)
		}
		return fmt.Sprintf("project/%s/template/%s", template.Primary.Attributes["project_id"], template.Primary.ID), nil
	}
}

func testAccProjectPolicyKeepsProjectSettings() resource.TestCheckFunc {
	return func(state *terraform.State) error {
		project, found := state.RootModule().Resources["semaphore_ex_project.test"]
		if !found {
			return fmt.Errorf("acceptance project is missing")
		}
		var body map[string]any
		if err := exRequest(context.Background(), testClient(), http.MethodGet, "/project/{project_id}", map[string]string{"project_id": project.Primary.ID}, nil, &body); err != nil {
			return fmt.Errorf("read project after SSH key policy update: %w", err)
		}
		if body["name"] != project.Primary.Attributes["name"] || body["alert"] != true {
			return fmt.Errorf("SSH key policy update changed unrelated project settings: %#v", body)
		}
		maxParallel, err := identityNumber(body["max_parallel_tasks"])
		if err != nil || maxParallel != 4 {
			return fmt.Errorf("SSH key policy update changed max_parallel_tasks: %#v", body["max_parallel_tasks"])
		}
		return nil
	}
}

func testAccProjectPolicySelectionCleared() resource.TestCheckFunc {
	return testAccProjectPolicySelectionClearedForProject("semaphore_ex_project.test")
}

func testAccProjectPolicySelectionClearedForProject(resourceName string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		project, found := state.RootModule().Resources[resourceName]
		if !found {
			return fmt.Errorf("acceptance project %q is missing", resourceName)
		}
		var body map[string]any
		if err := exRequest(context.Background(), testClient(), http.MethodGet, "/project/{project_id}", map[string]string{"project_id": project.Primary.ID}, nil, &body); err != nil {
			return fmt.Errorf("read project after policy deletion: %w", err)
		}
		if body["default_ssh_keys"] != nil || body["always_ssh_keys"] != nil {
			return fmt.Errorf("policy deletion did not reset both selections: %#v", body)
		}
		return nil
	}
}

func testAccSSHKeyPolicyReplacementConfig(suffix, policyProject, key string) string {
	return fmt.Sprintf(`
resource "semaphore_ex_project" "first" { name = "ssh-key-policy-first-%[1]s" }
resource "semaphore_ex_project" "second" { name = "ssh-key-policy-second-%[1]s" }

resource "semaphore_ex_project_generated_ssh_key" "first" {
  project_id = semaphore_ex_project.first.id
  name       = "first-%[1]s"
  login      = "first"
  login_wo_version = 1
  algorithm  = "ed25519"
}

resource "semaphore_ex_project_generated_ssh_key" "second" {
  project_id = semaphore_ex_project.second.id
  name       = "second-%[1]s"
  login      = "second"
  login_wo_version = 1
  algorithm  = "ed25519"
}

resource "semaphore_ex_project_ssh_key_policy" "test" {
  project_id = semaphore_ex_project.%[2]s.id
  default_ssh_keys = [{ access_key_id = semaphore_ex_project_generated_ssh_key.%[3]s.id }]
}
`, suffix, policyProject, key)
}

func testAccTemplateSelection(expected string, bindings int) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		template, found := state.RootModule().Resources["semaphore_ex_project_template.test"]
		if !found {
			return fmt.Errorf("acceptance template is missing")
		}
		var body map[string]any
		if err := exRequest(context.Background(), testClient(), http.MethodGet, "/project/{project_id}/templates/{template_id}", map[string]string{"project_id": template.Primary.Attributes["project_id"], "template_id": template.Primary.ID}, nil, &body); err != nil {
			return fmt.Errorf("read template SSH key selection: %w", err)
		}
		selection := body["ssh_keys"]
		switch expected {
		case "inherit":
			if selection != nil {
				return fmt.Errorf("expected inherited SSH keys, got %#v", selection)
			}
		case "explicit":
			items, ok := selection.([]any)
			if !ok || len(items) != bindings {
				return fmt.Errorf("expected %d explicit SSH key bindings, got %#v", bindings, selection)
			}
		default:
			return fmt.Errorf("unknown SSH key expectation %q", expected)
		}
		return nil
	}
}

func TestAcc_ProjectSSHKeyPolicyLifecycle(t *testing.T) {
	suffix := acctest.RandString(8)
	nonEmptyPolicy := testAccSSHKeyPolicyResource(`default_ssh_keys = [{
    access_key_id = semaphore_ex_project_generated_ssh_key.one.id
    hosts = ["git.example.test"]
  }]`, `always_ssh_keys = [{
    access_key_id = semaphore_ex_project_generated_ssh_key.two.id
  }]`)
	emptyPolicy := testAccSSHKeyPolicyResource("default_ssh_keys = []", "always_ssh_keys = []")
	omittedPolicy := testAccSSHKeyPolicyResource("", "")
	nonEmptyTemplate := testAccTemplateSSHKeySelection(false, `bindings = [{
      access_key_id = semaphore_ex_project_generated_ssh_key.one.id
      hosts = ["git.example.test"]
    }]`)
	inheritedTemplate := testAccTemplateSSHKeySelection(true, "")
	emptyTemplate := testAccTemplateSSHKeySelection(false, "bindings = []")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSSHKeyPolicyConfig(suffix, nonEmptyPolicy, nonEmptyTemplate),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("semaphore_ex_project_ssh_key_policy.test", "default_ssh_keys.#", "1"),
					resource.TestCheckResourceAttrPair("semaphore_ex_project_ssh_key_policy.test", "default_ssh_keys.0.access_key_id", "semaphore_ex_project_generated_ssh_key.one", "id"),
					resource.TestCheckResourceAttr("semaphore_ex_project_ssh_key_policy.test", "default_ssh_keys.0.hosts.#", "1"),
					resource.TestCheckResourceAttr("semaphore_ex_project_ssh_key_policy.test", "default_ssh_keys.0.hosts.0", "git.example.test"),
					resource.TestCheckResourceAttr("semaphore_ex_project_ssh_key_policy.test", "always_ssh_keys.#", "1"),
					resource.TestCheckResourceAttrPair("data.semaphore_ex_project_ssh_key_policy.test", "default_ssh_keys.0.access_key_id", "semaphore_ex_project_generated_ssh_key.one", "id"),
					resource.TestCheckResourceAttrPair("data.semaphore_ex_project_ssh_key_policy.test", "always_ssh_keys.0.access_key_id", "semaphore_ex_project_generated_ssh_key.two", "id"),
					resource.TestCheckResourceAttr("data.semaphore_ex_project_template.test", "ssh_keys.inherit", "false"),
					resource.TestCheckResourceAttr("data.semaphore_ex_project_template.test", "ssh_keys.bindings.#", "1"),
					resource.TestCheckResourceAttrPair("data.semaphore_ex_project_template.test", "ssh_keys.bindings.0.access_key_id", "semaphore_ex_project_generated_ssh_key.one", "id"),
					testAccProjectPolicyKeepsProjectSettings(),
					testAccTemplateSelection("explicit", 1),
				),
			},
			{
				ResourceName:      "semaphore_ex_project_ssh_key_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testAccProjectSSHKeyPolicyImportID("semaphore_ex_project_ssh_key_policy.test"),
			},
			{
				ResourceName:      "semaphore_ex_project_template.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testAccTemplateSSHKeyImportID("semaphore_ex_project_template.test"),
			},
			{
				Config: testAccSSHKeyPolicyConfig(suffix, omittedPolicy, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("semaphore_ex_project_ssh_key_policy.test", "default_ssh_keys.#", "1"),
					resource.TestCheckResourceAttr("semaphore_ex_project_ssh_key_policy.test", "always_ssh_keys.#", "1"),
					resource.TestCheckResourceAttr("semaphore_ex_project_template.test", "ssh_keys.inherit", "false"),
					testAccTemplateSelection("explicit", 1),
				),
			},
			{
				Config: testAccSSHKeyPolicyConfig(suffix, strings.Replace(nonEmptyPolicy, `hosts = ["git.example.test"]`, `hosts = []`, 1), strings.Replace(nonEmptyTemplate, `hosts = ["git.example.test"]`, `hosts = []`, 1)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("semaphore_ex_project_ssh_key_policy.test", "default_ssh_keys.0.hosts.#", "0"),
					resource.TestCheckResourceAttr("semaphore_ex_project_template.test", "ssh_keys.bindings.0.hosts.#", "0"),
				),
			},
			{Config: testAccSSHKeyPolicyConfig(suffix, nonEmptyPolicy, inheritedTemplate), Check: testAccTemplateSelection("inherit", 0)},
			{Config: testAccSSHKeyPolicyConfig(suffix, nonEmptyPolicy, emptyTemplate), Check: testAccTemplateSelection("explicit", 0)},
			{Config: testAccSSHKeyPolicyConfig(suffix, nonEmptyPolicy, inheritedTemplate), Check: testAccTemplateSelection("inherit", 0)},
			{Config: testAccSSHKeyPolicyConfig(suffix, nonEmptyPolicy, nonEmptyTemplate), Check: testAccTemplateSelection("explicit", 1)},
			{
				Config: testAccSSHKeyPolicyConfig(suffix, emptyPolicy, nonEmptyTemplate),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("semaphore_ex_project_ssh_key_policy.test", "default_ssh_keys.#", "0"),
					resource.TestCheckResourceAttr("semaphore_ex_project_ssh_key_policy.test", "always_ssh_keys.#", "0"),
					testAccProjectPolicyKeepsProjectSettings(),
				),
			},
			{
				Config: testAccSSHKeyPolicyConfig(suffix, omittedPolicy, nonEmptyTemplate),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("semaphore_ex_project_ssh_key_policy.test", "default_ssh_keys.#", "0"),
					resource.TestCheckResourceAttr("semaphore_ex_project_ssh_key_policy.test", "always_ssh_keys.#", "0"),
				),
			},
			{
				Config: testAccSSHKeyPolicyConfig(suffix, "", nonEmptyTemplate),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccProjectPolicySelectionCleared(),
					testAccProjectPolicyKeepsProjectSettings(),
				),
			},
		},
	})
}

func TestAcc_ProjectSSHKeyPolicyRejectsForeignKey(t *testing.T) {
	suffix := acctest.RandString(8)
	config := fmt.Sprintf(`
resource "semaphore_ex_project" "test" { name = "ssh-key-policy-%[1]s" }
resource "semaphore_ex_project" "foreign" { name = "ssh-key-policy-foreign-%[1]s" }
resource "semaphore_ex_project_generated_ssh_key" "foreign" {
  project_id = semaphore_ex_project.foreign.id
  name = "foreign-%[1]s"
  login = "foreign"
  login_wo_version = 1
  algorithm = "ed25519"
}
resource "semaphore_ex_project_ssh_key_policy" "test" {
  project_id = semaphore_ex_project.test.id
  default_ssh_keys = [{ access_key_id = semaphore_ex_project_generated_ssh_key.foreign.id }]
}
`, suffix)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: config, ExpectError: regexp.MustCompile(`(?s)400`)}}})
}

func TestAcc_ProjectSSHKeyPolicyProjectReplacementClearsOldParent(t *testing.T) {
	suffix := acctest.RandString(8)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSSHKeyPolicyReplacementConfig(suffix, "first", "first"),
				Check:  resource.TestCheckResourceAttrPair("semaphore_ex_project_ssh_key_policy.test", "project_id", "semaphore_ex_project.first", "id"),
			},
			{
				Config: testAccSSHKeyPolicyReplacementConfig(suffix, "second", "second"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("semaphore_ex_project_ssh_key_policy.test", "project_id", "semaphore_ex_project.second", "id"),
					testAccProjectPolicySelectionClearedForProject("semaphore_ex_project.first"),
				),
			},
		},
	})
}

func TestAcc_ProjectSSHKeyPolicyRejectsNonSSHKeyAndInvalidHost(t *testing.T) {
	suffix := acctest.RandString(8)
	nonSSH := fmt.Sprintf(`
resource "semaphore_ex_project" "test" { name = "ssh-key-policy-%[1]s" }
resource "semaphore_ex_project_key" "non_ssh" {
  project_id = semaphore_ex_project.test.id
  name = "none-%[1]s"
  none = {}
}
resource "semaphore_ex_project_ssh_key_policy" "test" {
  project_id = semaphore_ex_project.test.id
  default_ssh_keys = [{ access_key_id = semaphore_ex_project_key.non_ssh.id }]
}
`, suffix)
	invalidHost := fmt.Sprintf(`
resource "semaphore_ex_project" "test" { name = "ssh-key-policy-host-%[1]s" }
resource "semaphore_ex_project_generated_ssh_key" "test" {
  project_id = semaphore_ex_project.test.id
  name = "host-%[1]s"
  login = "host"
  login_wo_version = 1
  algorithm = "ed25519"
}
resource "semaphore_ex_project_ssh_key_policy" "test" {
  project_id = semaphore_ex_project.test.id
  default_ssh_keys = [{ access_key_id = semaphore_ex_project_generated_ssh_key.test.id, hosts = ["not a host"] }]
}
`, suffix)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: nonSSH, ExpectError: regexp.MustCompile(`(?s)400`)},
			{Config: invalidHost, ExpectError: regexp.MustCompile(`(?s)400`)},
		},
	})
}
