package provider

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func testAccTerraformBackendAliasConfig(suffix string, useSecondKey bool) string {
	selected := "first.id"
	if useSecondKey {
		selected = "second.id"
	}
	return fmt.Sprintf(`
resource "semaphore_ex_project" "test" {
  name = "terraform-backend-%[1]s"
}
resource "semaphore_ex_project_key" "none" {
  project_id = semaphore_ex_project.test.id
  name = "none-%[1]s"
  none = {}
}
resource "semaphore_ex_project_key" "first" {
  project_id = semaphore_ex_project.test.id
  name = "backend-first-%[1]s"
  login_password = { login = "backend-user-%[1]s", password = "backend-password-%[1]s" }
}
resource "semaphore_ex_project_key" "second" {
  project_id = semaphore_ex_project.test.id
  name = "backend-second-%[1]s"
  login_password = { login = "backend-user-%[1]s", password = "backend-password-second-%[1]s" }
}
resource "semaphore_ex_project_inventory" "workspace" {
  project_id = semaphore_ex_project.test.id
  name = "workspace-%[1]s"
  ssh_key_id = semaphore_ex_project_key.none.id
  terraform_workspace = { workspace = "acceptance-%[1]s" }
}
resource "semaphore_ex_project_terraform_backend_alias" "test" {
  project_id = semaphore_ex_project.test.id
  inventory_id = semaphore_ex_project_inventory.workspace.id
  auth_key_id = semaphore_ex_project_key.%[2]s
}
data "semaphore_ex_project_terraform_backend_alias" "test" {
  project_id = semaphore_ex_project.test.id
  inventory_id = semaphore_ex_project_inventory.workspace.id
  id = semaphore_ex_project_terraform_backend_alias.test.id
}
`, suffix, selected)
}

func testAccTerraformBackendAliasImportID(name string) resource.ImportStateIdFunc {
	return func(state *terraform.State) (string, error) {
		alias := state.RootModule().Resources[name].Primary.Attributes
		return fmt.Sprintf("project/%s/inventory/%s/alias/%s", alias["project_id"], alias["inventory_id"], alias["id"]), nil
	}
}

func testAccTerraformHTTPBackendState(t *testing.T, username, password string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		alias := state.RootModule().Resources["semaphore_ex_project_terraform_backend_alias.test"].Primary.Attributes["id"]
		dir := t.TempDir()
		config := []byte("terraform {\n  backend \"http\" {}\n}\nresource \"terraform_data\" \"acceptance\" {\n  input = \"ok\"\n}\n")
		if err := os.WriteFile(filepath.Join(dir, "main.tf"), config, 0o600); err != nil {
			return err
		}
		address := os.Getenv("SEMAPHOREUI_API_BASE_URL") + "/terraform/" + alias
		denied, err := http.NewRequest(http.MethodGet, address, nil)
		if err != nil {
			return err
		}
		denied.SetBasicAuth(username, "wrong-password")
		response, err := http.DefaultClient.Do(denied)
		if err != nil {
			return err
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			return fmt.Errorf("wrong backend credential returned %d", response.StatusCode)
		}
		request := func(method, id string) (*http.Response, error) {
			body := bytes.NewBufferString(`{"ID":"` + id + `"}`)
			req, err := http.NewRequest(method, address, body)
			if err != nil {
				return nil, err
			}
			req.SetBasicAuth(username, password)
			return http.DefaultClient.Do(req)
		}
		locked, err := request("LOCK", "acceptance-lock")
		if err != nil {
			return err
		}
		_ = locked.Body.Close()
		if locked.StatusCode != http.StatusOK {
			return fmt.Errorf("initial lock returned %d", locked.StatusCode)
		}
		contended, err := request("LOCK", "competing-lock")
		if err != nil {
			return err
		}
		_ = contended.Body.Close()
		if contended.StatusCode != http.StatusConflict {
			return fmt.Errorf("competing lock returned %d", contended.StatusCode)
		}
		wrongPost, err := http.NewRequest(http.MethodPost, address+"?ID=competing-lock", bytes.NewBufferString(`{"version":4}`))
		if err != nil {
			return err
		}
		wrongPost.SetBasicAuth(username, password)
		response, err = http.DefaultClient.Do(wrongPost)
		if err != nil {
			return err
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusConflict {
			return fmt.Errorf("wrong lock state write returned %d", response.StatusCode)
		}
		wrongUnlock, err := request("UNLOCK", "competing-lock")
		if err != nil {
			return err
		}
		_ = wrongUnlock.Body.Close()
		if wrongUnlock.StatusCode != http.StatusConflict {
			return fmt.Errorf("wrong unlock returned %d", wrongUnlock.StatusCode)
		}
		unlocked, err := request("UNLOCK", "acceptance-lock")
		if err != nil {
			return err
		}
		_ = unlocked.Body.Close()
		if unlocked.StatusCode != http.StatusOK {
			return fmt.Errorf("owner unlock returned %d", unlocked.StatusCode)
		}
		env := append(os.Environ(), "TF_HTTP_ADDRESS="+address, "TF_HTTP_LOCK_ADDRESS="+address, "TF_HTTP_UNLOCK_ADDRESS="+address, "TF_HTTP_USERNAME="+username, "TF_HTTP_PASSWORD="+password, "TF_HTTP_LOCK_METHOD=LOCK", "TF_HTTP_UNLOCK_METHOD=UNLOCK")
		for _, args := range [][]string{{"init", "-input=false"}, {"apply", "-auto-approve", "-input=false"}, {"refresh", "-input=false"}, {"destroy", "-auto-approve", "-input=false"}} {
			command := exec.Command("terraform", args...)
			command.Dir, command.Env = dir, env
			if output, err := command.CombinedOutput(); err != nil {
				return fmt.Errorf("terraform %v: %w: %s", args, err, output)
			}
		}
		return nil
	}
}

func testAccTerraformBackendAliasURL(state *terraform.State) error {
	resource := state.RootModule().Resources["semaphore_ex_project_terraform_backend_alias.test"].Primary.Attributes
	if !strings.HasSuffix(resource["url"], "/api/terraform/"+resource["id"]) {
		return fmt.Errorf("backend URL does not end in the alias identifier")
	}
	data := state.RootModule().Resources["data.semaphore_ex_project_terraform_backend_alias.test"].Primary.Attributes
	if data["url"] != resource["url"] {
		return fmt.Errorf("backend alias data source URL differs from resource")
	}
	return nil
}

func TestAcc_ProjectTerraformBackendAlias(t *testing.T) {
	suffix := acctest.RandString(8)
	firstUser, firstPassword := "backend-user-"+suffix, "backend-password-"+suffix
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: testAccTerraformBackendAliasConfig(suffix, false), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttrSet("semaphore_ex_project_terraform_backend_alias.test", "id"), resource.TestCheckResourceAttrPair("data.semaphore_ex_project_terraform_backend_alias.test", "id", "semaphore_ex_project_terraform_backend_alias.test", "id"), testAccTerraformBackendAliasURL, testAccTerraformHTTPBackendState(t, firstUser, firstPassword))},
		{ResourceName: "semaphore_ex_project_terraform_backend_alias.test", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: testAccTerraformBackendAliasImportID("semaphore_ex_project_terraform_backend_alias.test")},
		{Config: testAccTerraformBackendAliasConfig(suffix, true), Check: resource.TestCheckResourceAttrPair("semaphore_ex_project_terraform_backend_alias.test", "auth_key_id", "semaphore_ex_project_key.second", "id")},
		{Config: `resource "semaphore_ex_project" "test" {
  name = "terraform-backend-removed-` + suffix + `"
}`, Check: testAccResourceNotExists("semaphore_ex_project_terraform_backend_alias.test")},
	}})
}
