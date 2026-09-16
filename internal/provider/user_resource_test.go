package provider

import (
	"fmt"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client/user"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func testAccUserExists(resourceName string) resource.TestCheckFunc {
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

		_, err = testClient().User.GetUsersUserID(&user.GetUsersUserIDParams{UserID: id}, nil)
		return err
	}
}

func testAccUserConfig(userNameSuffix string, userExtras string) string {
	return fmt.Sprintf(`
resource "semaphore_ex_user" "test" {
  username = "test-%[1]s"
  name     = "Test User"
  email    = "test@example.com"
  %[2]s
}`, userNameSuffix, userExtras)
}

func testAccUserConfig_Exists(userNameSuffix string) string {
	return fmt.Sprintf(`
resource "semaphore_ex_user" "existing" {
  username = "test-%[1]s"
  name	   = "Test User"
  email	   = "test@example.com"
}

resource "semaphore_ex_user" "test" {
  username       = "test-%[1]s"
  name           = "Test User"
  email          = "test@example.com"
  depends_on = [semaphore_ex_user.existing]
}`, userNameSuffix)
}

func testAccUserImportID(n string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return "", fmt.Errorf("not found: %s", n)
		}

		return fmt.Sprintf("user/%s", rs.Primary.Attributes["id"]), nil
	}
}

func TestAcc_UserResource_basic(t *testing.T) {
	userNameSuffix := acctest.RandString(8)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccUserConfig(userNameSuffix, `  admin = true
password = "password!"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccUserExists("semaphore_ex_user.test"),
					resource.TestCheckResourceAttr("semaphore_ex_user.test", "username", fmt.Sprintf("test-%s", userNameSuffix)),
					resource.TestCheckResourceAttr("semaphore_ex_user.test", "external", "false"),
					resource.TestCheckResourceAttr("semaphore_ex_user.test", "name", "Test User"),
					resource.TestCheckResourceAttr("semaphore_ex_user.test", "password", "password!"),
					resource.TestCheckResourceAttr("semaphore_ex_user.test", "admin", "true"),
					resource.TestCheckResourceAttr("semaphore_ex_user.test", "alert", "false"),
					resource.TestCheckResourceAttr("semaphore_ex_user.test", "email", "test@example.com"),
					resource.TestCheckResourceAttrSet("semaphore_ex_user.test", "id"),
					resource.TestCheckResourceAttrSet("semaphore_ex_user.test", "created"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "semaphore_ex_user.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: testAccUserImportID("semaphore_ex_user.test"),
				// Password is encrypted and not returned by the API on import
				ImportStateVerifyIgnore: []string{"password"},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("semaphore_ex_user.test", "password", ""),
				),
			},
			// Update and Read testing
			{
				Config: testAccUserConfig(userNameSuffix, `  admin = false
password = "something"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("semaphore_ex_user.test", "username", fmt.Sprintf("test-%s", userNameSuffix)),
					resource.TestCheckResourceAttr("semaphore_ex_user.test", "email", "test@example.com"),
					resource.TestCheckResourceAttr("semaphore_ex_user.test", "name", "Test User"),
					resource.TestCheckResourceAttr("semaphore_ex_user.test", "password", "something"),
					resource.TestCheckResourceAttr("semaphore_ex_user.test", "admin", "false"),
					resource.TestCheckResourceAttr("semaphore_ex_user.test", "alert", "false"),
					resource.TestCheckResourceAttr("semaphore_ex_user.test", "external", "false"),
				),
			},
		},
	})
}

func TestAcc_UserResource_errorOnExists(t *testing.T) {
	userNameSuffix := acctest.RandString(8)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,

		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config:      testAccUserConfig_Exists(userNameSuffix),
				ExpectError: regexp.MustCompile("Could not create user, unexpected error"),
			},
		},
	})
}
