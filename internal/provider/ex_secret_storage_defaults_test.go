package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_ProjectSecretStorage_implicitTokenAuth(t *testing.T) {
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	defer endpoint.Close()
	url := strings.Replace(endpoint.URL, "127.0.0.1", "localhost", 1)
	config := fmt.Sprintf(`
resource "semaphore_ex_project" "test" { name = "implicit-auth-%s" }
resource "semaphore_ex_project_secret_storage" "test" {
  project_id = semaphore_ex_project.test.id
  name = "implicit-auth"
  type = "vault"
  params = { url = %q }
  secret_wo = %q
  secret_wo_version = 1
}
`, acctest.RandString(8), url, acctest.RandString(32))
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("semaphore_ex_project_secret_storage.test", "params.url", url),
			resource.TestCheckNoResourceAttr("semaphore_ex_project_secret_storage.test", "params.auth_method"),
		)},
		{Config: strings.Replace(config, `name = "implicit-auth"`, `name = "implicit-auth-renamed"`, 1)},
	}})
}
