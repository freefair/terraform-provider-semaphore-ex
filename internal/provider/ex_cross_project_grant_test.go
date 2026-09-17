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
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEXCrossProjectGrantOperationsAndResponseValidation(t *testing.T) {
	ctx := context.Background()
	operations, diagnostics := types.SetValueFrom(ctx, types.StringType, []string{"reference", "run"})
	require.False(t, diagnostics.HasError(), "%v", diagnostics)
	mask, err := exCrossProjectGrantOperations(ctx, operations)
	require.NoError(t, err)
	assert.Equal(t, int64(3), mask)

	state, err := exCrossProjectGrantState(ctx, exCrossProjectGrantModel{}, exCrossProjectGrantResponse{ID: 4, OwnerProjectID: 1, ConsumerProjectID: 2, TemplateID: 3, MinVersion: 1, MaxVersion: 2, Operations: 3, Status: "active", Revision: 5, Reason: "shared deployment", Created: "2026-09-16T00:00:00Z"})
	require.NoError(t, err)
	assert.True(t, state.Accepted.ValueBool())
	assert.Equal(t, int64(5), state.Revision.ValueInt64())
	_, err = exCrossProjectGrantState(ctx, state, exCrossProjectGrantResponse{ID: 4, OwnerProjectID: 1, ConsumerProjectID: 2, TemplateID: 3, MinVersion: 1, MaxVersion: 2, Operations: 4, Status: "active", Revision: 5})
	assert.Error(t, err)
}

func TestEXCrossProjectGrantTransitionsUseCorrectProjectAndRevision(t *testing.T) {
	requests := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/project/22/cross-project-template-grants/9/accept":
			assert.Equal(t, float64(7), body["expected_revision"])
			_, _ = w.Write([]byte(`{"id":9,"owner_project_id":11,"consumer_project_id":22,"template_id":33,"min_version":1,"max_version":1,"operations":3,"status":"active","revision":8,"reason":"release","created":"2026-09-16T00:00:00Z"}`))
		case "/api/project/11/cross-project-template-grants/9/revoke":
			assert.Equal(t, float64(8), body["expected_revision"])
			assert.Equal(t, "Terraform resource removal", body["reason"])
			_, _ = w.Write([]byte(`{"id":9,"owner_project_id":11,"consumer_project_id":22,"template_id":33,"min_version":1,"max_version":1,"operations":3,"status":"revoked","revision":9,"reason":"release","created":"2026-09-16T00:00:00Z"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	r := &exCrossProjectGrantResource{client: newEXTestClient(t, server.URL)}
	state := exCrossProjectGrantModel{ID: types.Int64Value(9), OwnerProjectID: types.Int64Value(11), ConsumerProjectID: types.Int64Value(22), TemplateID: types.Int64Value(33), Revision: types.Int64Value(7)}
	accepted, err := r.transition(context.Background(), state, "accept")
	require.NoError(t, err)
	assert.Equal(t, "active", accepted.Status.ValueString())
	revoked, err := r.revoke(context.Background(), accepted)
	require.NoError(t, err)
	assert.Equal(t, "revoked", revoked.Status.ValueString())
	assert.Equal(t, []string{"POST /api/project/22/cross-project-template-grants/9/accept", "POST /api/project/11/cross-project-template-grants/9/revoke"}, requests)
}

func TestEXCrossProjectGrantReadKeepsOwnerScoping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/project/11/cross-project-template-grants", r.URL.Path)
		assert.Equal(t, "100", r.URL.Query().Get("count"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":9,"owner_project_id":11,"consumer_project_id":22,"template_id":33,"min_version":1,"max_version":1,"operations":1,"status":"pending","revision":7,"reason":"release","created":"2026-09-16T00:00:00Z"}]`))
	}))
	defer server.Close()
	state, err := exCrossProjectGrantRead(context.Background(), newEXTestClient(t, server.URL), 11, 9, exCrossProjectGrantModel{OwnerProjectID: types.Int64Value(11), ID: types.Int64Value(9)})
	require.NoError(t, err)
	assert.Equal(t, int64(22), state.ConsumerProjectID.ValueInt64())
	assert.False(t, state.Accepted.ValueBool())
}

func TestEXCrossProjectGrantReadFindsTargetBeyondFirstPage(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		assert.Equal(t, "/api/project/11/cross-project-template-grants", r.URL.Path)
		assert.Equal(t, "100", r.URL.Query().Get("count"))
		w.Header().Set("Content-Type", "application/json")
		if requests == 1 {
			assert.Empty(t, r.URL.Query().Get("before"))
			page := make([]exCrossProjectGrantResponse, 100)
			for index := range page {
				page[index] = exCrossProjectGrantResponse{ID: int64(200 - index)}
			}
			_ = json.NewEncoder(w).Encode(page)
			return
		}
		assert.Equal(t, "101", r.URL.Query().Get("before"))
		_ = json.NewEncoder(w).Encode([]exCrossProjectGrantResponse{{ID: 42, OwnerProjectID: 11, ConsumerProjectID: 22, TemplateID: 33, MinVersion: 1, MaxVersion: 1, Operations: 1, Status: "pending", Revision: 7, Reason: "later page", Created: "2026-09-16T00:00:00Z"}})
	}))
	defer server.Close()

	state, err := exCrossProjectGrantRead(context.Background(), newEXTestClient(t, server.URL), 11, 42, exCrossProjectGrantModel{OwnerProjectID: types.Int64Value(11), ID: types.Int64Value(42)})
	require.NoError(t, err)
	assert.Equal(t, int64(42), state.ID.ValueInt64())
	assert.Equal(t, 2, requests)
}

func TestEXCrossProjectGrantReadRejectsNonAdvancingCursor(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]exCrossProjectGrantResponse{{ID: 10}})
	}))
	defer server.Close()

	_, err := exCrossProjectGrantRead(context.Background(), newEXTestClient(t, server.URL), 11, 42, exCrossProjectGrantModel{OwnerProjectID: types.Int64Value(11), ID: types.Int64Value(42)})
	require.ErrorContains(t, err, "pagination did not advance")
	assert.Equal(t, 2, requests)
}

func TestEXCrossProjectGrantRejectsStaleRevisionWithoutRetry(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		assert.Equal(t, "/api/project/22/cross-project-template-grants/9/accept", r.URL.Path)
		w.WriteHeader(http.StatusConflict)
	}))
	defer server.Close()
	r := &exCrossProjectGrantResource{client: newEXTestClient(t, server.URL)}
	_, err := r.transition(context.Background(), exCrossProjectGrantModel{ID: types.Int64Value(9), OwnerProjectID: types.Int64Value(11), ConsumerProjectID: types.Int64Value(22), Revision: types.Int64Value(3)}, "accept")
	require.Error(t, err)
	assert.Equal(t, 1, requests)
}

func TestAcc_EXCrossProjectGrant(t *testing.T) {
	suffix := acctest.RandString(8)
	base := testAccProjectTemplateConfig(suffix, "") + fmt.Sprintf(`
resource "semaphore_ex_project" "consumer" { name = "consumer-%s" }
`, suffix)
	grant := base + `
resource "semaphore_ex_cross_project_grant" "test" {
  owner_project_id    = semaphore_ex_project.test.id
  consumer_project_id = semaphore_ex_project.consumer.id
  template_id         = semaphore_ex_project_template.test.id
  min_version         = 1
  max_version         = 1
  operations          = ["reference", "run"]
  reason              = "acceptance coverage"
  accepted            = true
}
data "semaphore_ex_cross_project_grant" "test" {
  owner_project_id = semaphore_ex_project.test.id
  id               = semaphore_ex_cross_project_grant.test.id
}
`
	pending := base + `
resource "semaphore_ex_cross_project_grant" "test" {
  owner_project_id    = semaphore_ex_project.test.id
  consumer_project_id = semaphore_ex_project.consumer.id
  template_id         = semaphore_ex_project_template.test.id
  min_version         = 1
  max_version         = 1
  operations          = ["reference", "run"]
  reason              = "acceptance coverage"
  accepted            = false
}
`
	revoked := base + `
resource "semaphore_ex_cross_project_grant" "test" {
  owner_project_id    = semaphore_ex_project.test.id
  consumer_project_id = semaphore_ex_project.consumer.id
  template_id         = semaphore_ex_project_template.test.id
  min_version         = 1
  max_version         = 1
  operations          = ["reference", "run"]
  reason              = "acceptance coverage"
  accepted            = false
}
`
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: base, Check: func(state *terraform.State) error {
			template := state.RootModule().Resources["semaphore_ex_project_template.test"]
			if template == nil {
				return fmt.Errorf("template is missing")
			}
			projectID, err := strconv.ParseInt(template.Primary.Attributes["project_id"], 10, 64)
			if err != nil || projectID <= 0 {
				return fmt.Errorf("invalid template project ID")
			}
			templateID, err := strconv.ParseInt(template.Primary.ID, 10, 64)
			if err != nil || templateID <= 0 {
				return fmt.Errorf("invalid template ID")
			}
			a := &projectTemplatePublishAction{client: testClient()}
			resp := action.InvokeResponse{}
			a.Invoke(context.Background(), action.InvokeRequest{Config: templatePublishConfig(t, projectID, templateID)}, &resp)
			if resp.Diagnostics.HasError() {
				return fmt.Errorf("publish immutable template version: %v", resp.Diagnostics)
			}
			return nil
		}},
		{Config: pending, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_cross_project_grant.test", "status", "pending"), resource.TestCheckResourceAttr("semaphore_ex_cross_project_grant.test", "accepted", "false"))},
		{Config: grant, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_cross_project_grant.test", "status", "active"), resource.TestCheckResourceAttr("semaphore_ex_cross_project_grant.test", "accepted", "true"), resource.TestCheckResourceAttrSet("data.semaphore_ex_cross_project_grant.test", "revision"))},
		{ResourceName: "semaphore_ex_cross_project_grant.test", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: func(state *terraform.State) (string, error) {
			r := state.RootModule().Resources["semaphore_ex_cross_project_grant.test"]
			return fmt.Sprintf("project/%s/grant/%s", r.Primary.Attributes["owner_project_id"], r.Primary.ID), nil
		}},
		{Config: revoked, Check: resource.TestCheckResourceAttr("semaphore_ex_cross_project_grant.test", "status", "revoked")},
	}})
}
