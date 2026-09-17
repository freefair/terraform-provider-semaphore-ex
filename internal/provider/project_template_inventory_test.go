package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateInventoryDetectsExternalReassignment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"id":3,"project_id":1,"template_id":99}`))
		assert.NoError(t, err)
	}))
	defer server.Close()
	attached, err := templateInventoryAttached(context.Background(), newEXTestClient(t, server.URL), projectTemplateInventoryModel{ProjectID: types.Int64Value(1), TemplateID: types.Int64Value(2), InventoryID: types.Int64Value(3)})
	require.NoError(t, err)
	assert.False(t, attached)
}

func TestAcc_ProjectTemplateInventory(t *testing.T) {
	suffix := acctest.RandString(8)
	config := func(name string) string {
		return testAccProjectTemplateConfig(suffix, "") + fmt.Sprintf(`
resource "semaphore_ex_project_inventory" "extra" {
  project_id = semaphore_ex_project.test.id
  name = %q
  ssh_key_id = semaphore_ex_project_key.test.id
  file = { path = "inventories/extra" }
}
resource "semaphore_ex_project_template_inventory" "test" {
  project_id = semaphore_ex_project.test.id
  template_id = semaphore_ex_project_template.test.id
  inventory_id = semaphore_ex_project_inventory.extra.id
}
data "semaphore_ex_project_template_inventory" "test" {
  project_id = semaphore_ex_project_template_inventory.test.project_id
  template_id = semaphore_ex_project_template_inventory.test.template_id
  inventory_id = semaphore_ex_project_template_inventory.test.inventory_id
}
`, name)
	}
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("Extra"), Check: resource.TestCheckResourceAttrSet("data.semaphore_ex_project_template_inventory.test", "id")},
		{ResourceName: "semaphore_ex_project_template_inventory.test", ImportState: true, ImportStateVerify: true},
		{Config: config("Updated extra"), Check: func(state *terraform.State) error {
			inventory := state.RootModule().Resources["semaphore_ex_project_inventory.extra"]
			template := state.RootModule().Resources["semaphore_ex_project_template.test"]
			if inventory.Primary.Attributes["template_id"] != template.Primary.ID {
				return fmt.Errorf("inventory update lost attachment")
			}
			return nil
		}},
	}})
}
