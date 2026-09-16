package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func testAccUserDataSourceConfigByID() string {
	return `
resource "semaphore_ex_user" "user" {
  username = "username1"
  name     = "User Name"
  email    = "test@example.com"
}
data "semaphore_ex_user" "test" {
  id = semaphore_ex_user.user.id
}`
}

func testAccUserDataSourceConfigByUsername() string {
	return `
resource "semaphore_ex_user" "user" {
  username = "username1"
  name     = "User Name"
  email    = "test@example.com"
}
data "semaphore_ex_user" "test" {
  username   = "username1"
  depends_on = [semaphore_ex_user.user]
}`
}

func testAccUserDataSourceConfigByEmail() string {
	return `
resource "semaphore_ex_user" "user" {
  username = "username1"
  name     = "User Name"
  email    = "test@example.com"
}
data "semaphore_ex_user" "test" {
  email      = "test@example.com"
  depends_on = [semaphore_ex_user.user]
}`
}

func TestAcc_UserDataSource_id(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccUserDataSourceConfigByID(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.semaphore_ex_user.test", "username", "username1"),
					resource.TestCheckResourceAttr("data.semaphore_ex_user.test", "name", "User Name"),
					resource.TestCheckResourceAttr("data.semaphore_ex_user.test", "email", "test@example.com"),
					resource.TestCheckResourceAttr("data.semaphore_ex_user.test", "password", ""),
					resource.TestCheckResourceAttr("data.semaphore_ex_user.test", "admin", "false"),
					resource.TestCheckResourceAttr("data.semaphore_ex_user.test", "alert", "false"),
					resource.TestCheckResourceAttr("data.semaphore_ex_user.test", "external", "false"),
					resource.TestCheckResourceAttrSet("data.semaphore_ex_user.test", "created"),
				),
			},
		},
	})
}

func TestAcc_UserDataSource_username(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccUserDataSourceConfigByUsername(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.semaphore_ex_user.test", "username", "username1"),
					resource.TestCheckResourceAttr("data.semaphore_ex_user.test", "name", "User Name"),
					resource.TestCheckResourceAttr("data.semaphore_ex_user.test", "email", "test@example.com"),
					resource.TestCheckResourceAttr("data.semaphore_ex_user.test", "password", ""),
					resource.TestCheckResourceAttr("data.semaphore_ex_user.test", "admin", "false"),
					resource.TestCheckResourceAttr("data.semaphore_ex_user.test", "alert", "false"),
					resource.TestCheckResourceAttr("data.semaphore_ex_user.test", "external", "false"),
					resource.TestCheckResourceAttrSet("data.semaphore_ex_user.test", "created"),
				),
			},
		},
	})
}

func TestAcc_UserDataSource_email(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccUserDataSourceConfigByEmail(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.semaphore_ex_user.test", "username", "username1"),
					resource.TestCheckResourceAttr("data.semaphore_ex_user.test", "name", "User Name"),
					resource.TestCheckResourceAttr("data.semaphore_ex_user.test", "email", "test@example.com"),
					resource.TestCheckResourceAttr("data.semaphore_ex_user.test", "password", ""),
					resource.TestCheckResourceAttr("data.semaphore_ex_user.test", "admin", "false"),
					resource.TestCheckResourceAttr("data.semaphore_ex_user.test", "alert", "false"),
					resource.TestCheckResourceAttr("data.semaphore_ex_user.test", "external", "false"),
					resource.TestCheckResourceAttrSet("data.semaphore_ex_user.test", "created"),
				),
			},
		},
	})
}
