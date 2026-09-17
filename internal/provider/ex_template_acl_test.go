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
)

func TestTemplateACLNamedPermissionsRoundTripAndRevision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/project/1/templates/2/perms/catalog" {
			t.Fatalf("catalog path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"template.read","permission":1},{"id":"template.run","permission":2}]`))
	}))
	defer server.Close()
	allowed, _ := types.SetValueFrom(context.Background(), types.StringType, []string{"template.read"})
	denied, _ := types.SetValueFrom(context.Background(), types.StringType, []string{"template.run"})
	model := exTemplateACLModel{ID: types.Int64Value(3), ProjectID: types.Int64Value(1), TemplateID: types.Int64Value(2), RoleSlug: types.StringValue("project_manager"), AllowedPermissions: allowed, DeniedPermissions: denied, Revision: types.Int64Value(7)}
	catalog, err := aclCatalog(context.Background(), newEXTestClient(t, server.URL), model)
	if err != nil {
		t.Fatal(err)
	}
	body, err := aclBody(context.Background(), model, catalog, model.Revision.ValueInt64())
	if err != nil || body["allowed_permissions"] != int64(1) || body["denied_permissions"] != int64(2) || body["revision"] != int64(7) {
		t.Fatalf("body = %#v, err = %v", body, err)
	}
	state, err := aclState(context.Background(), newEXTestClient(t, server.URL), model, exTemplateACLResponse{ID: 3, ProjectID: 1, TemplateID: 2, RoleSlug: "project_manager", AllowedPermissions: 1, DeniedPermissions: 2, Revision: 8})
	if err != nil || state.Revision.ValueInt64() != 8 {
		t.Fatalf("state = %#v, err = %v", state, err)
	}
}

func TestAcc_EXTemplateACL(t *testing.T) {
	suffix := acctest.RandString(8)
	base := testAccProjectTemplateConfig(suffix, "")
	config := base + `
resource "semaphore_ex_template_acl" "test" {
  project_id  = semaphore_ex_project.test.id
  template_id = semaphore_ex_project_template.test.id
  role_slug   = "manager"
  allowed_permissions = ["template.read"]
  denied_permissions  = ["template.delete"]
}
data "semaphore_ex_template_acl" "test" {
  project_id  = semaphore_ex_project.test.id
  template_id = semaphore_ex_project_template.test.id
  id          = semaphore_ex_template_acl.test.id
}`
	updated := base + `
resource "semaphore_ex_template_acl" "test" {
  project_id  = semaphore_ex_project.test.id
  template_id = semaphore_ex_project_template.test.id
  role_slug   = "manager"
  allowed_permissions = ["template.read", "template.run"]
  denied_permissions  = []
}
data "semaphore_ex_template_acl" "test" {
  project_id  = semaphore_ex_project.test.id
  template_id = semaphore_ex_project_template.test.id
  id          = semaphore_ex_template_acl.test.id
}`
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttrSet("semaphore_ex_template_acl.test", "id"), resource.TestCheckResourceAttr("data.semaphore_ex_template_acl.test", "allowed_permissions.#", "1"), resource.TestCheckResourceAttrSet("semaphore_ex_template_acl.test", "revision"))},
		{ResourceName: "semaphore_ex_template_acl.test", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: func(s *terraform.State) (string, error) {
			r := s.RootModule().Resources["semaphore_ex_template_acl.test"]
			return fmt.Sprintf("project/%s/template/%s/acl/%s", r.Primary.Attributes["project_id"], r.Primary.Attributes["template_id"], r.Primary.ID), nil
		}},
		{Config: updated, Check: resource.TestCheckResourceAttr("semaphore_ex_template_acl.test", "allowed_permissions.#", "2")},
	}})
}
