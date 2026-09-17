package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/require"
)

func TestGeneratedSSHKeyFromAPIRejectsNonGeneratedKey(t *testing.T) {
	_, err := generatedSSHKeyFromAPI(map[string]any{
		"id": int64(3), "project_id": int64(2), "name": "ordinary", "type": "ssh",
	}, generatedSSHKeyModel{})
	require.ErrorContains(t, err, "not server-generated")
}

func testAccGeneratedSSHKeyConfig(suffix, name, algorithm string, loginVersion int) string {
	loginConfig := ""
	if loginVersion > 0 {
		loginConfig = fmt.Sprintf("  login            = \"deploy\"\n  login_wo_version = %d\n", loginVersion)
	}
	return fmt.Sprintf(`
resource "semaphore_ex_project" "generated_key" {
  name = "generated-key-%[1]s"
}

resource "semaphore_ex_project_generated_ssh_key" "test" {
  project_id = semaphore_ex_project.generated_key.id
  name       = %q
%[3]s
  algorithm  = %q
}

data "semaphore_ex_project_generated_ssh_key" "test" {
  project_id = semaphore_ex_project.generated_key.id
  id         = semaphore_ex_project_generated_ssh_key.test.id
}
`, suffix, name, loginConfig, algorithm)
}

func testAccGeneratedSSHKeyProjectOnlyConfig(suffix string) string {
	return fmt.Sprintf(`resource "semaphore_ex_project" "generated_key" { name = "generated-key-%s" }`, suffix)
}

func generatedSSHKeyImportID(resourceName string) resource.ImportStateIdFunc {
	return func(state *terraform.State) (string, error) {
		resourceState, found := state.RootModule().Resources[resourceName]
		if !found {
			return "", fmt.Errorf("generated SSH key resource %q is missing", resourceName)
		}
		return fmt.Sprintf("project/%s/generated-ssh-key/%s", resourceState.Primary.Attributes["project_id"], resourceState.Primary.ID), nil
	}
}

func TestAcc_EXProjectGeneratedSSHKey(t *testing.T) {
	suffix := acctest.RandString(8)
	firstName := "generated " + suffix
	secondName := firstName + " renamed"
	var projectID, keyID int64
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccGeneratedSSHKeyConfig(suffix, firstName, "ed25519", 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("semaphore_ex_project_generated_ssh_key.test", "name", firstName),
					resource.TestCheckResourceAttr("semaphore_ex_project_generated_ssh_key.test", "algorithm", "ed25519"),
					resource.TestCheckResourceAttrSet("semaphore_ex_project_generated_ssh_key.test", "public_key"),
					resource.TestCheckResourceAttrSet("semaphore_ex_project_generated_ssh_key.test", "fingerprint"),
					resource.TestCheckNoResourceAttr("semaphore_ex_project_generated_ssh_key.test", "login"),
					resource.TestCheckResourceAttrSet("data.semaphore_ex_project_generated_ssh_key.test", "public_key"),
					func(state *terraform.State) error {
						resourceState := state.RootModule().Resources["semaphore_ex_project_generated_ssh_key.test"]
						var err error
						projectID, err = strconv.ParseInt(resourceState.Primary.Attributes["project_id"], 10, 64)
						if err != nil {
							return err
						}
						keyID, err = strconv.ParseInt(resourceState.Primary.ID, 10, 64)
						return err
					},
				),
			},
			{
				Config: testAccGeneratedSSHKeyConfig(suffix, secondName, "ed25519", 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("semaphore_ex_project_generated_ssh_key.test", "name", secondName),
					resource.TestCheckResourceAttrSet("semaphore_ex_project_generated_ssh_key.test", "public_key"),
				),
			},
			{
				Config: testAccGeneratedSSHKeyConfig(suffix, secondName, "ed25519", 2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("semaphore_ex_project_generated_ssh_key.test", "login_wo_version", "2"),
					func(state *terraform.State) error {
						resourceState := state.RootModule().Resources["semaphore_ex_project_generated_ssh_key.test"]
						replacementID, err := strconv.ParseInt(resourceState.Primary.ID, 10, 64)
						if err != nil {
							return err
						}
						if replacementID == keyID {
							return fmt.Errorf("login_wo_version change did not replace generated SSH key")
						}
						keyID = replacementID
						return nil
					},
				),
			},
			{
				Config: testAccGeneratedSSHKeyConfig(suffix, secondName, "rsa-3072", 2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("semaphore_ex_project_generated_ssh_key.test", "algorithm", "rsa-3072"),
					func(state *terraform.State) error {
						resourceState := state.RootModule().Resources["semaphore_ex_project_generated_ssh_key.test"]
						replacementID, err := strconv.ParseInt(resourceState.Primary.ID, 10, 64)
						if err != nil {
							return err
						}
						if replacementID == keyID {
							return fmt.Errorf("algorithm change did not replace generated SSH key")
						}
						keyID = replacementID
						return nil
					},
				),
			},
			{
				ResourceName:            "semaphore_ex_project_generated_ssh_key.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"login", "login_wo_version"},
				ImportStateIdFunc:       generatedSSHKeyImportID("semaphore_ex_project_generated_ssh_key.test"),
			},
			{
				Config: testAccGeneratedSSHKeyProjectOnlyConfig(suffix),
				Check: func(_ *terraform.State) error {
					err := exRequest(context.Background(), testClient(), http.MethodGet, "/project/{project_id}/keys/{key_id}", map[string]string{"project_id": strconv.FormatInt(projectID, 10), "key_id": strconv.FormatInt(keyID, 10)}, nil, &map[string]any{})
					if !exNotFound(err) {
						return fmt.Errorf("generated SSH key still exists after deletion: %v", err)
					}
					return nil
				},
			},
		},
	})
}
