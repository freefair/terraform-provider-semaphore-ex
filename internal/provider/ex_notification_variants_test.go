package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_EXNotificationDestinationVariants(t *testing.T) {
	suffix := acctest.RandString(8)
	credential := acctest.RandString(32)
	config := fmt.Sprintf(`
resource "semaphore_ex_global_notification_destination" "generic" {
  name = "generic-%s"
  destination_type = "generic"
  environment = "test"
  enabled = false
}
resource "semaphore_ex_global_notification_destination" "servicenow" {
  name = "servicenow-%s"
  destination_type = "servicenow"
  environment = "test"
  enabled = false
  credential_wo = %q
  credential_wo_version = 1
  servicenow = {
    instance_origin = "https://example.service-now.com"
    auth_mode = "basic"
    basic_username = "fixture"
    field_mappings = [{ incident_field = "short_description", source_field = "summary" }]
  }
}
resource "semaphore_ex_global_notification_destination" "opsgenie" {
  name = "opsgenie-%s"
  destination_type = "opsgenie"
  environment = "test"
  region = "us"
  enabled = false
  credential_wo = %q
  credential_wo_version = 1
  opsgenie = { priority = "P3", responders = [] }
}
`, suffix, suffix, credential, suffix, credential)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttrSet("semaphore_ex_global_notification_destination.generic", "id"),
			resource.TestCheckResourceAttrSet("semaphore_ex_global_notification_destination.servicenow", "id"),
			resource.TestCheckResourceAttr("semaphore_ex_global_notification_destination.opsgenie", "opsgenie.responders.#", "0"),
		)},
		{Config: strings.ReplaceAll(config, `environment = "test"`, `environment = "test-updated"`)},
	}})
}
