package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

func TestAcc_App(t *testing.T) {
	id := "provider_test_" + acctest.RandString(8)
	config := func(color, args string) string {
		return fmt.Sprintf(`
resource "semaphore_ex_app" "test" {
  id = %q
  title = "Custom tool"
  path = "/usr/local/bin/custom-tool"
  color = %q
  args = %s
}
data "semaphore_ex_app" "test" { id = semaphore_ex_app.test.id }
`, id, color, args)
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("#123456", `["--verbose"]`), Check: resource.TestCheckResourceAttr("data.semaphore_ex_app.test", "args.0", "--verbose")},
		{ResourceName: "semaphore_ex_app.test", ImportState: true, ImportStateVerify: true},
		{Config: config("", `[]`), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("data.semaphore_ex_app.test", "color", ""), resource.TestCheckResourceAttr("data.semaphore_ex_app.test", "args.#", "0"))},
	}})
}
