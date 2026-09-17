package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func templatePublishConfig(t *testing.T, projectID, templateID int64) tfsdk.Config {
	t.Helper()
	var schemaResponse action.SchemaResponse
	(&projectTemplatePublishAction{}).Schema(context.Background(), action.SchemaRequest{}, &schemaResponse)
	typeOf := schemaResponse.Schema.Type().TerraformType(context.Background())
	return tfsdk.Config{Schema: schemaResponse.Schema, Raw: tftypes.NewValue(typeOf, map[string]tftypes.Value{
		"project_id": tftypes.NewValue(tftypes.Number, projectID), "template_id": tftypes.NewValue(tftypes.Number, templateID),
	})}
}

func TestTemplatePublishOnlyInvokesOnAction(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/project/1/templates/2/versions", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"id":3,"version_number":1,"content_fingerprint":"sha256:test"}`))
		assert.NoError(t, err)
	}))
	defer server.Close()
	a := &projectTemplatePublishAction{client: newEXTestClient(t, server.URL)}
	var schemaResponse action.SchemaResponse
	a.Schema(context.Background(), action.SchemaRequest{}, &schemaResponse)
	assert.Zero(t, requests)
	resp := action.InvokeResponse{}
	a.Invoke(context.Background(), action.InvokeRequest{Config: templatePublishConfig(t, 1, 2)}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.Equal(t, 1, requests)
}

func TestTemplateVersionPaginationFindsOlderVersion(t *testing.T) {
	pages := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages++
		w.Header().Set("Content-Type", "application/json")
		var values []templateVersionResponse
		switch r.URL.Query().Get("before") {
		case "":
			values = []templateVersionResponse{{ID: 20, VersionNumber: 2}}
		case "20":
			values = []templateVersionResponse{{ID: 10, VersionNumber: 1}}
		default:
			t.Errorf("unexpected cursor")
			w.WriteHeader(400)
			return
		}
		assert.NoError(t, json.NewEncoder(w).Encode(values))
	}))
	defer server.Close()
	version, err := findTemplateVersion(context.Background(), newEXTestClient(t, server.URL), 1, 2, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(10), version.ID)
	assert.Equal(t, 2, pages)
}

func TestAcc_ProjectTemplateVersion(t *testing.T) {
	suffix := acctest.RandString(8)
	config := testAccProjectTemplateConfig(suffix, "")
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config, Check: func(state *terraform.State) error {
				template, ok := state.RootModule().Resources["semaphore_ex_project_template.test"]
				if !ok {
					return fmt.Errorf("template missing")
				}
				projectID, err := strconv.ParseInt(template.Primary.Attributes["project_id"], 10, 64)
				if err != nil {
					return err
				}
				templateID, err := strconv.ParseInt(template.Primary.ID, 10, 64)
				if err != nil {
					return err
				}
				a := &projectTemplatePublishAction{client: testClient()}
				resp := action.InvokeResponse{}
				a.Invoke(context.Background(), action.InvokeRequest{Config: templatePublishConfig(t, projectID, templateID)}, &resp)
				if resp.Diagnostics.HasError() {
					return fmt.Errorf("publish: %v", resp.Diagnostics)
				}
				return nil
			}},
			{Config: config + `
data "semaphore_ex_project_template_version" "test" {
  project_id = semaphore_ex_project.test.id
  template_id = semaphore_ex_project_template.test.id
  version_number = 1
}
`, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("data.semaphore_ex_project_template_version.test", "version_number", "1"), resource.TestCheckResourceAttrSet("data.semaphore_ex_project_template_version.test", "content_fingerprint"))},
		},
	})
}
