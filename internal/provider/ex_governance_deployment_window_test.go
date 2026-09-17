package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func deploymentWindowRules(t *testing.T) types.List {
	t.Helper()
	rule, diagnostics := types.ObjectValue(exDeploymentWindowRuleTypes(), map[string]attr.Value{
		"name": types.StringValue("maintenance"), "active": types.BoolValue(true), "kind": types.StringValue("freeze"), "scope": types.StringValue("project"),
		"template_id": types.Int64Null(), "workflow_id": types.Int64Null(), "recurrence": types.StringValue("0 2 * * 1"), "duration_minutes": types.Int64Value(60),
		"effective_from": types.StringNull(), "effective_until": types.StringNull(),
	})
	require.False(t, diagnostics.HasError(), diagnostics.Errors())
	rules, diagnostics := types.ListValue(types.ObjectType{AttrTypes: exDeploymentWindowRuleTypes()}, []attr.Value{rule})
	require.False(t, diagnostics.HasError(), diagnostics.Errors())
	return rules
}

func TestEXDeploymentWindowUsesRevisionFencedRoutes(t *testing.T) {
	model := exProjectDeploymentWindowModel{ProjectID: types.Int64Value(7), Revision: types.Int64Value(3), Timezone: types.StringValue("UTC"), Default: types.StringValue("allow"), Rules: deploymentWindowRules(t)}
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		assert.Equal(t, "/api/project/7/deployment-windows", r.URL.Path)
		switch requestCount {
		case 1:
			assert.Equal(t, http.MethodGet, r.Method)
		case 2:
			assert.Equal(t, http.MethodPut, r.Method)
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, float64(3), body["revision"])
			assert.NotContains(t, body["rules"].([]any)[0].(map[string]any), "id")
		case 3:
			assert.Equal(t, http.MethodDelete, r.Method)
			assert.Equal(t, "3", r.URL.Query().Get("expected_revision"))
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"project_id":7,"revision":4,"timezone":"UTC","default":"allow","rules":[{"id":9,"revision":4,"name":"maintenance","active":true,"kind":"freeze","scope":"project","recurrence":"0 2 * * 1","duration_minutes":60}]}`))
	}))
	defer server.Close()

	client := newEXTestClient(t, server.URL)
	current, err := exReadProjectDeploymentWindow(context.Background(), client, model)
	require.NoError(t, err)
	assert.Equal(t, int64(4), current.Revision.ValueInt64())

	body, err := exDeploymentWindowBody(context.Background(), model, true)
	require.NoError(t, err)
	params, err := exDeploymentWindowParams(model.ProjectID)
	require.NoError(t, err)
	var raw map[string]any
	require.NoError(t, exRequest(context.Background(), client, http.MethodPut, exDeploymentWindowRoute(), params, body, &raw))
	next, err := exDeploymentWindowFromResponse(context.Background(), model, raw)
	require.NoError(t, err)
	assert.Equal(t, int64(4), next.Revision.ValueInt64())

	require.NoError(t, exRequestWithOptions(context.Background(), client, http.MethodDelete, exDeploymentWindowRoute(), exRequestOptions{PathParams: params, Query: map[string]string{"expected_revision": "3"}}, nil, nil))
}

func TestEXDeploymentWindowRejectsMissingRevision(t *testing.T) {
	_, err := exDeploymentWindowBody(context.Background(), exProjectDeploymentWindowModel{Revision: types.Int64Null()}, true)
	assert.ErrorContains(t, err, "refresh the deployment-window policy")
}

func TestAcc_EXProjectDeploymentWindow(t *testing.T) {
	name := "acceptance-deployment-window-" + acctest.RandString(8)
	config := `
resource "semaphore_ex_project" "test" { name = "` + name + `" }
resource "semaphore_ex_project_deployment_window" "test" {
  project_id = semaphore_ex_project.test.id
  timezone = "UTC"
  default = "allow"
  rules = [{
    name = "maintenance"
    active = true
    kind = "freeze"
    scope = "project"
    recurrence = "0 2 * * 1"
    duration_minutes = 60
  }]
}
data "semaphore_ex_project_deployment_window" "test" { project_id = semaphore_ex_project.test.id }
`
	updated := strings.Replace(config, `default = "allow"`, `default = "deny"`, 1)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttrSet("semaphore_ex_project_deployment_window.test", "revision"), resource.TestCheckResourceAttr("data.semaphore_ex_project_deployment_window.test", "timezone", "UTC"))},
		{ResourceName: "semaphore_ex_project_deployment_window.test", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: func(state *terraform.State) (string, error) {
			return state.RootModule().Resources["semaphore_ex_project.test"].Primary.ID, nil
		}},
		{Config: updated, Check: resource.TestCheckResourceAttr("semaphore_ex_project_deployment_window.test", "default", "deny")},
	}})
}
