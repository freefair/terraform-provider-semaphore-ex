package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func taskGroupTestModel() projectTaskGroupModel {
	return projectTaskGroupModel{ID: types.Int64Value(5), ProjectID: types.Int64Value(2), OwnerProjectID: types.Int64Value(2), Name: types.StringValue("state"), Description: types.StringValue(""), MaxParallelTasks: types.Int64Value(1), RunnerIDs: types.SetValueMust(types.Int64Type, nil), SharedProjectIDs: types.SetValueMust(types.Int64Type, nil), Revision: types.Int64Value(3)}
}
func TestTaskGroupUpdateUsesStoredRevisionAndReads204(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(fmt.Sprint(conflict), func(t *testing.T) {
			ctx := context.Background()
			schema := projectTaskGroupSchema().GetResource(ctx)
			old := taskGroupTestModel()
			plan := old
			plan.Revision = types.Int64Unknown()
			plan.MaxParallelTasks = types.Int64Value(2)
			state, planned := tfsdk.State{Schema: schema}, tfsdk.Plan{Schema: schema}
			require.False(t, state.Set(ctx, &old).HasError())
			require.False(t, planned.Set(ctx, &plan).HasError())
			methods := []string{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				methods = append(methods, r.Method)
				assert.Equal(t, "/api/project/2/task_groups/5", r.URL.Path)
				if r.Method == http.MethodPut {
					var body map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					assert.Equal(t, float64(3), body["revision"])
					assert.Equal(t, float64(2), body["max_parallel_tasks"])
					assert.Equal(t, []any{}, body["shared_project_ids"])
					if conflict {
						w.WriteHeader(400)
						_, _ = w.Write([]byte(`{"error":"task group was modified concurrently"}`))
						return
					}
					w.WriteHeader(204)
					return
				}
				assert.Equal(t, http.MethodGet, r.Method)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(taskGroupResponse{ID: 5, ProjectID: 2, Name: "state", MaxParallelTasks: 2, Revision: 4})
			}))
			defer server.Close()
			response := frameworkresource.UpdateResponse{State: state}
			(&projectTaskGroupResource{client: newEXTestClient(t, server.URL)}).Update(ctx, frameworkresource.UpdateRequest{Plan: planned, State: state}, &response)
			assert.Equal(t, conflict, response.Diagnostics.HasError(), "%v", response.Diagnostics)
			if conflict {
				assert.Equal(t, []string{"PUT"}, methods)
				return
			}
			assert.Equal(t, []string{"PUT", "GET"}, methods)
			var next projectTaskGroupModel
			require.False(t, response.State.Get(ctx, &next).HasError())
			assert.Equal(t, int64(4), next.Revision.ValueInt64())
			assert.Empty(t, next.RunnerIDs.Elements())
		})
	}
}
func TestTaskGroupDeleteUsesRevisionAndPreservesConflict(t *testing.T) {
	ctx := context.Background()
	schema := projectTaskGroupSchema().GetResource(ctx)
	old := taskGroupTestModel()
	state := tfsdk.State{Schema: schema}
	require.False(t, state.Set(ctx, &old).HasError())
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		assert.Equal(t, http.MethodDelete, r.Method)
		var body map[string]int64
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, int64(3), body["revision"])
		w.WriteHeader(400)
	}))
	defer server.Close()
	response := frameworkresource.DeleteResponse{State: state}
	(&projectTaskGroupResource{client: newEXTestClient(t, server.URL)}).Delete(ctx, frameworkresource.DeleteRequest{State: state}, &response)
	assert.True(t, response.Diagnostics.HasError())
	assert.Equal(t, 1, calls)
}
func TestTemplateManagedGroupsMutation(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name    string
		ids     types.Set
		present bool
	}{{"omitted", types.SetNull(types.Int64Type), false}, {"unknown", types.SetUnknown(types.Int64Type), false}, {"selected", types.SetValueMust(types.Int64Type, []attr.Value{types.Int64Value(2), types.Int64Value(8)}), true}, {"cleared", types.SetValueMust(types.Int64Type, nil), true}} {
		t.Run(tc.name, func(t *testing.T) {
			plan := ProjectTemplateModel{Vaults: types.ListNull(ProjectTemplateVaultType), TaskGroups: tc.ids}
			body, err := templateRequestWithSSHKeys(ctx, plan)
			require.NoError(t, err)
			value, present := body["task_groups"]
			assert.Equal(t, tc.present, present)
			if tc.present {
				ids, ok := value.([]int64)
				require.True(t, ok)
				assert.Len(t, ids, len(tc.ids.Elements()))
				assert.NotNil(t, ids)
			}
		})
	}
}
func TestAcc_ProjectTaskGroupAndTemplateBindings(t *testing.T) {
	suffix := acctest.RandString(8)
	config := func(groups string, limit int) string {
		return testAccProjectTemplateConfig(suffix, groups) + fmt.Sprintf(`
resource "semaphore_ex_project" "consumer" { name = "consumer-%s" }
resource "semaphore_ex_project_task_group" "test" {
 project_id = semaphore_ex_project.test.id
 name = "state-%s"
 max_parallel_tasks = %d
 shared_project_ids = [semaphore_ex_project.consumer.id]
}
resource "semaphore_ex_project_task_group" "second" {
 project_id = semaphore_ex_project.test.id
 name = "deploy-%s"
}
data "semaphore_ex_project_task_group" "shared" {
 project_id = semaphore_ex_project.consumer.id
 id = semaphore_ex_project_task_group.test.id
}
data "semaphore_ex_project_template" "read" {
 project_id = semaphore_ex_project.test.id
 id = semaphore_ex_project_template.test.id
}
`, suffix, suffix, limit, suffix)
	}
	both := `task_groups = [semaphore_ex_project_task_group.test.id, semaphore_ex_project_task_group.second.id]`
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config(both, 1), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_project_template.test", "task_groups.#", "2"), resource.TestCheckResourceAttr("data.semaphore_ex_project_template.read", "task_groups.#", "2"), resource.TestCheckResourceAttrPair("data.semaphore_ex_project_task_group.shared", "owner_project_id", "semaphore_ex_project.test", "id"), resource.TestCheckNoResourceAttr("data.semaphore_ex_project_task_group.shared", "shared_project_ids.#"))},
		{ResourceName: "semaphore_ex_project_task_group.test", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: func(s *terraform.State) (string, error) {
			v := s.RootModule().Resources["semaphore_ex_project_task_group.test"].Primary.Attributes
			return "project/" + v["project_id"] + "/task_group/" + v["id"], nil
		}},
		{ResourceName: "semaphore_ex_project_template.test", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: testAccProjectTemplateImportID("semaphore_ex_project_template.test")},
		{Config: config(both+"\ndescription = \"updated\"", 2), Check: resource.TestCheckResourceAttr("semaphore_ex_project_task_group.test", "max_parallel_tasks", "2")},
		{Config: strings.ReplaceAll(strings.ReplaceAll(config(both+"\ndescription = \"preserve imported policy\"", 2), " shared_project_ids = [semaphore_ex_project.consumer.id]", ""), " max_parallel_tasks = 2", ""), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_project_task_group.test", "max_parallel_tasks", "2"), resource.TestCheckResourceAttr("semaphore_ex_project_task_group.test", "shared_project_ids.#", "1"))},
		{Config: config(`task_groups = [semaphore_ex_project_task_group.test.id]`, 2), Check: resource.TestCheckResourceAttr("semaphore_ex_project_template.test", "task_groups.#", "1")},
		{Config: config(`description = "omitted bindings"`, 2), Check: resource.TestCheckResourceAttr("semaphore_ex_project_template.test", "task_groups.#", "1")},
		{Config: config(`task_groups = []`, 2), Check: resource.TestCheckResourceAttr("semaphore_ex_project_template.test", "task_groups.#", "0")},
	}})
}

func TestTaskGroupNamesRejectServerNormalization(t *testing.T) {
	for _, name := range []string{" state", "state ", "\u00a0state", "state\nline", "state\x00", ""} {
		t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
			resp := validator.StringResponse{}
			taskGroupNameValidator{}.ValidateString(context.Background(), validator.StringRequest{Path: path.Root("name"), ConfigValue: types.StringValue(name)}, &resp)
			assert.True(t, resp.Diagnostics.HasError())
		})
	}
}
