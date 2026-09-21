package provider

import (
	"context"
	"fmt"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCoverageEnvironmentLosslessJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":2,"project_id":1,"name":"typed","json":"{\"count\":3,\"enabled\":true,\"nested\":{\"a\":[1,2]},\"label\":\"keep\"}","env":"{\"COUNT\":3,\"ENABLED\":true}"}`))
	}))
	defer server.Close()
	ctx := context.Background()
	d := &projectEnvironmentDataSource{client: newEXTestClient(t, server.URL)}
	var schema datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schema)
	require.Contains(t, schema.Schema.Attributes, "variables_json")
	require.Contains(t, schema.Schema.Attributes, "environment_json")
	config := tfsdk.Config{Schema: schema.Schema, Raw: unknownConfigObject(schema.Schema.Type().TerraformType(ctx), map[string]any{"id": int64(2), "project_id": int64(1)})}
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schema.Schema}}
	d.Read(ctx, datasource.ReadRequest{Config: config}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	var variables, environment types.String
	require.False(t, resp.State.GetAttribute(ctx, path.Root("variables_json"), &variables).HasError())
	require.False(t, resp.State.GetAttribute(ctx, path.Root("environment_json"), &environment).HasError())
	assert.JSONEq(t, `{"count":3,"enabled":true,"nested":{"a":[1,2]},"label":"keep"}`, variables.ValueString())
	assert.JSONEq(t, `{"COUNT":3,"ENABLED":true}`, environment.ValueString())
}

func TestCoverageMalformedEnvironmentJSONFailsRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":2,"project_id":1,"name":"malformed","json":"{invalid","env":"{}"}`))
	}))
	defer server.Close()
	ctx := context.Background()
	d := &projectEnvironmentDataSource{client: newEXTestClient(t, server.URL)}
	var schema datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schema)
	config := tfsdk.Config{Schema: schema.Schema, Raw: unknownConfigObject(schema.Schema.Type().TerraformType(ctx), map[string]any{"id": int64(2), "project_id": int64(1)})}
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schema.Schema}}
	d.Read(ctx, datasource.ReadRequest{Config: config}, &resp)
	assert.True(t, resp.Diagnostics.HasError(), "malformed JSON must not become an empty successful result")
}

func TestCoverageEnvironmentJSONRoundTrip(t *testing.T) {
	for _, raw := range []string{`{"list":[1,true,null],"object":{"enabled":true},"big":9007199254740993}`, `{ "name" : "value" }`, `{}`} {
		t.Run(raw, func(t *testing.T) {
			previous := ProjectEnvironmentModel{VariablesJSON: types.StringValue(raw), EnvironmentJSON: types.StringValue(`{"COUNT":3}`)}
			request, err := convertProjectEnvironmentModelToEnvironmentRequest(context.Background(), previous, &ProjectEnvironmentModel{})
			require.NoError(t, err)
			assert.JSONEq(t, raw, request.JSON)
			current, err := convertEnvironmentResponseToProjectEnvironmentModel(context.Background(), &models.Environment{JSON: request.JSON, Env: request.Env}, &previous)
			require.NoError(t, err)
			assert.Equal(t, previous.VariablesJSON, current.VariablesJSON, "preserve equivalent authored formatting")
			assert.Equal(t, previous.EnvironmentJSON, current.EnvironmentJSON)
		})
	}
}

func TestCoverageEnvironmentJSONRejectsMalformedResponses(t *testing.T) {
	for _, test := range []struct {
		raw    string
		scalar bool
	}{{`{"a":1} trailing`, false}, {`[1,2]`, false}, {`{"":1}`, false}, {`{"ARRAY":[1,2]}`, true}, {`{"OBJECT":{"a":1}}`, true}} {
		_, err := environmentJSONObject(test.raw, test.scalar)
		require.Error(t, err)
	}
}

func TestAcc_CoverageEnvironmentJSON(t *testing.T) {
	suffix := acctest.RandString(8)
	config := func(variables, environment string) string {
		return fmt.Sprintf(`
resource "semaphore_ex_project" "json" { name = "json-%s" }
resource "semaphore_ex_project_environment" "json" {
 project_id = semaphore_ex_project.json.id
 name = "typed"
 variables_json = %q
 environment_json = %q
}
data "semaphore_ex_project_environment" "json" {
 project_id = semaphore_ex_project.json.id
 id = semaphore_ex_project_environment.json.id
}`, suffix, variables, environment)
	}
	first := config(`{"enabled":true,"nested":{"items":[1,2]},"retries":3}`, `{"COUNT":3,"ENABLED":true}`)
	second := config(`{"enabled":false,"items":["next"],"number":9007199254740993}`, `{"COUNT":4}`)
	empty := config(`{}`, `{}`)
	stringsOnly := config(`{"name":"value"}`, `{"LABEL":"text"}`)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: first, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_project_environment.json", "variables_json", `{"enabled":true,"nested":{"items":[1,2]},"retries":3}`), resource.TestCheckResourceAttr("data.semaphore_ex_project_environment.json", "environment_json", `{"COUNT":3,"ENABLED":true}`))},
		{ResourceName: "semaphore_ex_project_environment.json", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: func(s *terraform.State) (string, error) {
			r := s.RootModule().Resources["semaphore_ex_project_environment.json"]
			return "project/" + r.Primary.Attributes["project_id"] + "/environment/" + r.Primary.ID, nil
		}},
		{Config: first, PlanOnly: true},
		{Config: second, Check: resource.TestCheckResourceAttr("data.semaphore_ex_project_environment.json", "variables_json", `{"enabled":false,"items":["next"],"number":9007199254740993}`)},
		{Config: stringsOnly, Check: resource.TestCheckResourceAttr("semaphore_ex_project_environment.json", "variables_json", `{"name":"value"}`)},
		{ResourceName: "semaphore_ex_project_environment.json", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: func(s *terraform.State) (string, error) {
			r := s.RootModule().Resources["semaphore_ex_project_environment.json"]
			return "project/" + r.Primary.Attributes["project_id"] + "/environment/" + r.Primary.ID, nil
		}},
		{Config: stringsOnly, PlanOnly: true},
		{Config: empty, Check: resource.TestCheckResourceAttr("semaphore_ex_project_environment.json", "variables_json", `{}`)},
		{Config: empty, PlanOnly: true},
	}})
}
