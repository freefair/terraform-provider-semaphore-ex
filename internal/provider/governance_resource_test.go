package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	resourceTest "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestManagedGovernanceResourcesRegistered(t *testing.T) {
	expected := map[string]bool{"semaphore_ex_global_policy_guardrail": false, "semaphore_ex_project_policy_guardrail": false, "semaphore_ex_global_workflow_artifact_retention": false, "semaphore_ex_project_workflow_artifact_retention": false}
	for _, factory := range (&SemaphoreUIProvider{}).Resources(context.Background()) {
		var metadata resource.MetadataResponse
		factory().Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "semaphore_ex"}, &metadata)
		if _, ok := expected[metadata.TypeName]; ok {
			expected[metadata.TypeName] = true
		}
	}
	for name, found := range expected {
		require.True(t, found, name)
	}
}

func TestGovernanceUpdateUsesStateRevisionWithoutRefreshing(t *testing.T) {
	for _, draft := range []bool{false, true} {
		t.Run(fmt.Sprint(draft), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				assert.Equal(t, http.MethodPut, r.Method)
				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, float64(7), body["expected_revision"])
				w.WriteHeader(http.StatusConflict)
			}))
			defer server.Close()
			ctx := context.Background()
			r := &governanceResource{draft: draft, client: newEXTestClient(t, server.URL)}
			var schema resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &schema)
			values := map[string]any{"id": "global", "revision": int64(7), "source_yaml": "version: 1\nrules: []\n", "retention_seconds": int64(3600), "max_artifact_bytes": int64(1024), "max_run_bytes": int64(2048)}
			raw := unknownConfigObject(schema.Schema.Type().TerraformType(ctx), values)
			state := tfsdk.State{Schema: schema.Schema, Raw: raw}
			response := resource.UpdateResponse{State: state}
			r.Update(ctx, resource.UpdateRequest{State: state, Plan: tfsdk.Plan{Schema: schema.Schema, Raw: raw}}, &response)
			assert.True(t, response.Diagnostics.HasError())
			assert.Equal(t, 1, calls)
			assert.True(t, response.State.Raw.Equal(state.Raw))
		})
	}
}
func TestGovernanceDeletePreservesPolicyAndHistory(t *testing.T) {
	for _, draft := range []bool{false, true} {
		r := &governanceResource{draft: draft}
		var response resource.DeleteResponse
		r.Delete(context.Background(), resource.DeleteRequest{}, &response)
		assert.False(t, response.Diagnostics.HasError())
	}
}

func TestAcc_ManagedGovernance(t *testing.T) {
	for _, draft := range []bool{true, false} {
		for _, project := range []bool{false, true} {
			t.Run(fmt.Sprintf("draft_%t_project_%t", draft, project), func(t *testing.T) {
				suffix := acctest.RandString(8)
				scope := "global"
				if project {
					scope = "project"
				}
				kind := "workflow_artifact_retention"
				if draft {
					kind = "policy_guardrail"
				}
				address := "semaphore_ex_" + scope + "_" + kind + ".test"
				base := fmt.Sprintf("resource \"semaphore_ex_project\" \"scope\" { name = %q }\n", "governance-"+suffix)
				configuration := func(updated bool) string {
					fields := ""
					if project {
						fields += "project_id = semaphore_ex_project.scope.id\n"
					}
					if draft {
						source := "version: 1\nrules: []\n"
						if updated {
							source += "# updated draft\n"
						}
						fields += fmt.Sprintf("source_yaml = %q\n", source)
					} else {
						retention := 2592000
						if project {
							retention = 3600
						}
						if updated {
							retention++
						}
						fields += fmt.Sprintf("retention_seconds = %d\nmax_artifact_bytes = 1048576\nmax_run_bytes = 2097152\n", retention)
					}
					return base + fmt.Sprintf("resource %q \"test\" {\n%s}\n", strings.TrimSuffix(address, ".test"), fields)
				}
				var ownedRevision string
				capture := func(state *terraform.State) error {
					ownedRevision = state.RootModule().Resources[address].Primary.Attributes["revision"]
					return nil
				}
				resourceTest.Test(t, resourceTest.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resourceTest.TestStep{
					{Config: configuration(false), Check: resourceTest.TestCheckResourceAttrSet(address, "revision")},
					{ResourceName: address, ImportState: true, ImportStateVerify: true, ImportStateIdFunc: func(state *terraform.State) (string, error) {
						return state.RootModule().Resources[address].Primary.Attributes["id"], nil
					}},
					{Config: configuration(false), PlanOnly: true, ExpectNonEmptyPlan: false},
					{Config: configuration(true), Check: capture},
					{Config: configuration(true), PlanOnly: true, ExpectNonEmptyPlan: false},
					{Config: base, Check: func(state *terraform.State) error {
						parameters := map[string]string(nil)
						route := exWorkflowArtifactRetentionRoute(project)
						if draft {
							route = exPolicyGuardrailRoute(project, "")
						}
						if project {
							parameters = map[string]string{"project_id": state.RootModule().Resources["semaphore_ex_project.scope"].Primary.ID}
						}
						var raw map[string]any
						if err := exRequest(context.Background(), testClient(), http.MethodGet, route, parameters, nil, &raw); err != nil {
							return err
						}
						field := "global_policy"
						if project {
							field = "project_policy"
						}
						if draft {
							field = "draft"
						}
						record, ok := raw[field].(map[string]any)
						if !ok {
							return fmt.Errorf("destroy removed policy")
						}
						if fmt.Sprint(record["revision"]) != ownedRevision {
							return fmt.Errorf("destroy changed policy revision")
						}
						return nil
					}},
				}})
			})
		}
	}
}
