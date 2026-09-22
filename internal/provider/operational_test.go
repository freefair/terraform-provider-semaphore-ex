package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/action"
	as "github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	providerfw "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	resourceTest "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
)

func TestOperationalEndpointsRegistered(t *testing.T) {
	p := New("test")()
	expected := map[string]bool{"semaphore_ex_ldap_group_apply": false, "semaphore_ex_project_task_confirm": false, "semaphore_ex_project_task_reject": false, "semaphore_ex_project_task_retry_recovery": false, "semaphore_ex_project_workflow_retry_reconcile": false, "semaphore_ex_workflow_trigger_test": false, "semaphore_ex_audit_webhook_test": false}
	for _, factory := range p.(providerfw.ProviderWithActions).Actions(context.Background()) {
		var metadata action.MetadataResponse
		factory().Metadata(context.Background(), action.MetadataRequest{ProviderTypeName: "semaphore_ex"}, &metadata)
		if _, ok := expected[metadata.TypeName]; ok {
			expected[metadata.TypeName] = true
		}
	}
	for name, found := range expected {
		require.True(t, found, name)
	}
	_, ok := p.(providerfw.ProviderWithEphemeralResources)
	require.True(t, ok, "preview results require ephemeral support")
}
func TestOperationalDynamicResultsHandleHeterogeneousObjects(t *testing.T) {
	require.NotPanics(t, func() {
		dynamicValueFromAPI([]any{map[string]any{"id": json.Number("1")}, map[string]any{"id": json.Number("2"), "reason": "changed"}})
	})
}

func operationTestConfig(t *testing.T, spec operationalSpec, supplied map[string]any) tfsdk.Config {
	t.Helper()
	schema := as.Schema{Attributes: spec.inputs}
	values := map[string]any{}
	for name, value := range supplied {
		if _, dynamic := spec.inputs[name].(as.DynamicAttribute); dynamic {
			raw, err := dynamicValueFromAPI(value).ToTerraformValue(context.Background())
			require.NoError(t, err)
			values[name] = raw
		} else {
			values[name] = value
		}
	}
	return tfsdk.Config{Schema: schema, Raw: unknownConfigObject(schema.Type().TerraformType(context.Background()), values)}
}

func TestOperationalWireContracts(t *testing.T) {
	cases := []struct {
		name, method, path, query, body string
		config                          map[string]any
	}{
		{"ldap_group_preview", "POST", "/capabilities/ldap/group-mappings/preview", "", `{"provider_id":"directory"}`, map[string]any{"provider_id": "directory"}},
		{"ldap_group_apply", "POST", "/capabilities/ldap/group-mappings/apply", "", `{"provider_id":"directory","preview_token":"synthetic-token"}`, map[string]any{"provider_id": "directory", "preview_token": "synthetic-token"}},
		{"ldap_group_reconcile", "POST", "/capabilities/ldap/group-mappings/reconcile", "", `{"provider_id":"directory"}`, map[string]any{"provider_id": "directory"}},
		{"oidc_group_preview", "POST", "/capabilities/oidc/group-mappings/preview", "", `{"provider_id":"oidc","user_id":9,"claim":["team"]}`, map[string]any{"provider_id": "oidc", "user_id": int64(9), "claim": []any{"team"}}},
		{"audit_webhook_test", "POST", "/audit-webhook/test", "key=next", "", map[string]any{"key": "next"}},
		{"docker_execution_policy_test", "POST", "/runners/docker-policy/test", "", `{"image":"example/image","privileged":true}`, map[string]any{"request": map[string]any{"image": "example/image", "privileged": true}}},
		{"kubernetes_execution_policy_test", "POST", "/runners/kubernetes-policies/test-cluster/test", "", `{"cluster_alias":"test-cluster","namespace":"jobs","has_host_path":true}`, map[string]any{"cluster_alias": "test-cluster", "request": map[string]any{"namespace": "jobs", "has_host_path": true}}},
		{"project_deployment_window_preview", "POST", "/project/1/deployment-windows/preview", "", `{"template_id":2,"revision":1,"timezone":"UTC","default":"allow","rules":[]}`, map[string]any{"project_id": int64(1), "policy": map[string]any{"template_id": json.Number("2"), "revision": json.Number("1"), "timezone": "UTC", "default": "allow", "rules": []any{}}}},
		{"project_workflow_retry_reconcile", "POST", "/project/1/workflows/2/runs/3/retry-reconcile", "", "", map[string]any{"project_id": int64(1), "workflow_id": int64(2), "run_id": int64(3)}},
		{"workflow_trigger_test", "POST", "/project/1/workflows/2/triggers/3/test", "", `{"inputs":{"build":42,"approved":false}}`, map[string]any{"project_id": int64(1), "workflow_id": int64(2), "trigger_id": int64(3), "inputs": map[string]any{"build": json.Number("42"), "approved": false}}},
	}
	for _, op := range []string{"confirm", "reject", "retry_recovery"} {
		cases = append(cases, struct {
			name, method, path, query, body string
			config                          map[string]any
		}{"project_task_" + op, "POST", "/project/1/tasks/2/" + strings.ReplaceAll(op, "_", "-"), "", "", map[string]any{"project_id": int64(1), "task_id": int64(2)}})
	}
	for _, scope := range []string{"global", "project"} {
		prefix := ""
		base := func(extra map[string]any) map[string]any {
			if scope == "project" {
				extra["project_id"] = int64(1)
			}
			return extra
		}
		if scope == "project" {
			prefix = "/project/1"
		}
		for _, op := range []string{"destination_test", "delivery_retry"} {
			field, route := "destination_id", "destinations/2/test"
			if op == "delivery_retry" {
				field, route = "delivery_id", "deliveries/2/retry"
			}
			cases = append(cases, struct {
				name, method, path, query, body string
				config                          map[string]any
			}{scope + "_notification_" + op, "POST", prefix + "/notification-governance/" + route, "", "", base(map[string]any{field: int64(2)})})
		}
		eventBody := `{"scope":"global","source_revision":4}`
		if scope == "project" {
			eventBody = `{"scope":"project","project_id":1,"source_revision":4}`
		}
		cases = append(cases, struct {
			name, method, path, query, body string
			config                          map[string]any
		}{scope + "_notification_routing_preview", "POST", prefix + "/notification-governance/routing/preview", "", eventBody, base(map[string]any{"event": map[string]any{"source_revision": json.Number("4")}})})
		for _, op := range []string{"validate", "diff", "test", "impact", "rollback"} {
			method, query, body := "POST", "", `{"source_yaml":"version: 1"}`
			fields := map[string]any{"source_yaml": "version: 1"}
			switch op {
			case "diff":
				method, query, body = "GET", "from_revision=2&to_revision=3", ""
				fields = map[string]any{"from_revision": int64(2), "to_revision": int64(3)}
			case "test":
				body = `{"source_yaml":"version: 1","input":{"project_id":1}}`
				fields = map[string]any{"source_yaml": "version: 1", "input": map[string]any{"project_id": json.Number("1")}}
			case "impact":
				body = `{"inputs":[{"project_id":1}]}`
				fields = map[string]any{"inputs": []any{map[string]any{"project_id": json.Number("1")}}}
			case "rollback":
				body = `{"revision":2,"expected_draft_revision":3,"reason":"restore"}`
				fields = map[string]any{"revision": int64(2), "expected_draft_revision": int64(3), "reason": "restore"}
			}
			cases = append(cases, struct {
				name, method, path, query, body string
				config                          map[string]any
			}{scope + "_policy_guardrail_" + op, method, prefix + "/policy-guardrails/" + op, query, body, base(fields)})
		}
	}
	require.Len(t, cases, len(operationalSpecs()), "every operation needs an independent wire fixture")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				assert.Equal(t, tc.method, r.Method)
				assert.Equal(t, "/api"+tc.path, r.URL.Path)
				assert.Equal(t, tc.query, r.URL.RawQuery)
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				if tc.body == "" {
					assert.Empty(t, body)
				} else {
					assert.JSONEq(t, tc.body, string(body))
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"allowed":false,"token":"synthetic-token","credential":"synthetic-password","issues":[{"message":"private detail"}]}`))
			}))
			defer server.Close()
			spec := operationalSpecs()[tc.name]
			instance := operationalAction{spec: spec, client: newEXTestClient(t, server.URL)}
			progress := ""
			response := action.InvokeResponse{SendProgress: func(event action.InvokeProgressEvent) { progress += event.Message }}
			instance.Invoke(context.Background(), action.InvokeRequest{Config: operationTestConfig(t, spec, tc.config)}, &response)
			require.False(t, response.Diagnostics.HasError(), "%v", response.Diagnostics)
			assert.Equal(t, 1, calls)
			assert.Contains(t, progress, `"allowed":false`)
			assert.Contains(t, progress, `"issues_count":1`)
			assert.NotContains(t, progress, "synthetic-")
			assert.NotContains(t, progress, "private detail")
		})
	}
}

func TestOperationalRejectsInvalidScopeBeforeHTTP(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config map[string]any
	}{
		{"project_notification_routing_preview", map[string]any{"project_id": int64(1), "event": map[string]any{"project_id": json.Number("2")}}},
		{"global_notification_routing_preview", map[string]any{"event": map[string]any{"scope": "project"}}},
		{"project_policy_guardrail_test", map[string]any{"project_id": int64(1), "input": map[string]any{"project_id": json.Number("2")}}},
		{"global_policy_guardrail_impact", map[string]any{"inputs": []any{}}},
		{"project_deployment_window_preview", map[string]any{"project_id": int64(1), "policy": map[string]any{"template_id": json.Number("2"), "workflow_id": json.Number("3")}}},
		{"kubernetes_execution_policy_test", map[string]any{"cluster_alias": "../escape", "request": map[string]any{}}},
		{"workflow_trigger_test", map[string]any{"project_id": int64(1), "workflow_id": int64(2), "trigger_id": int64(3), "inputs": []any{"invalid"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := operationalSpecs()[tc.name]
			config := operationTestConfig(t, spec, tc.config)
			var value types.Object
			require.False(t, config.Get(context.Background(), &value).HasError())
			_, err := spec.execute(context.Background(), nil, value)
			require.Error(t, err)
		})
	}
}

func TestOperationalPreviewSeparatesToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"synthetic-token","additions":[{"id":1},{"id":2,"reason":"new"}]}`))
	}))
	defer server.Close()
	instance := operationalPreview{spec: operationalSpecs()["ldap_group_preview"], client: newEXTestClient(t, server.URL)}
	ctx := context.Background()
	var schema ephemeral.SchemaResponse
	instance.Schema(ctx, ephemeral.SchemaRequest{}, &schema)
	require.True(t, schema.Schema.Attributes["result"].IsSensitive())
	require.True(t, schema.Schema.Attributes["preview_token"].IsSensitive())
	config := tfsdk.Config{Schema: schema.Schema, Raw: unknownConfigObject(schema.Schema.Type().TerraformType(ctx), map[string]any{"provider_id": "directory"})}
	response := ephemeral.OpenResponse{Result: tfsdk.EphemeralResultData{Schema: schema.Schema}}
	instance.Open(ctx, ephemeral.OpenRequest{Config: config}, &response)
	require.False(t, response.Diagnostics.HasError(), "%v", response.Diagnostics)
	var result types.Object
	require.False(t, response.Result.Get(ctx, &result).HasError())
	assert.Equal(t, types.StringValue("synthetic-token"), result.Attributes()["preview_token"])
	assert.NotContains(t, result.Attributes()["result"].String(), "synthetic-token")
}

func TestAcc_OperationalEphemeralResults(t *testing.T) {
	resourceTest.Test(t, resourceTest.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resourceTest.TestStep{{Config: `
 ephemeral "semaphore_ex_global_policy_guardrail_validate" "valid" {
   source_yaml = "version: 1\nrules: []\n"
   lifecycle {
 postcondition {
     condition = self.result.valid && self.result.rule_count == 0
     error_message = "Expected an empty valid policy."
   }
 }
 }
 ephemeral "semaphore_ex_global_policy_guardrail_validate" "invalid" {
   source_yaml = "version: 999\nrules: []\n"
   lifecycle {
 postcondition {
     condition = !self.result.valid && length(self.result.issues) > 0
     error_message = "Expected validation issues."
   }
 }
 }
 ephemeral "semaphore_ex_global_policy_guardrail_test" "fixture" {
   source_yaml = "version: 1\nrules: []\n"
   input = {
     project_id = 1
     intent = "task"
     evaluated_at = "2026-09-21T12:00:00Z"
     template = { id = 2, application = "ansible", source = "task" }
     executor = { type = "local", image_reference_kind = "none" }
   }
   lifecycle {
     postcondition {
       condition = self.result.allowed
       error_message = "Expected the empty authored policy to allow the fixture."
     }
   }
 }
 ephemeral "semaphore_ex_global_notification_routing_preview" "routing" {
   event = {
     source_revision = 1
     source = { kind = "task", id = "task:12" }
     lifecycle_id = "template:7"
     severity = "error"
     lifecycle_action = "trigger"
     details = { task_id = 12, template_id = 7, status = "failed" }
   }
 }
 ephemeral "semaphore_ex_docker_execution_policy_test" "denied" {
   request = { image = "example/image", privileged = true }
   lifecycle {
 postcondition {
     condition = !self.result.allowed
     error_message = "Expected the default policy to deny privileged execution."
   }
 }
 }
 `, Check: func(state *terraform.State) error {
		for address := range state.RootModule().Resources {
			if strings.Contains(address, "policy_guardrail_validate") || strings.Contains(address, "docker_execution_policy_test") {
				return fmt.Errorf("ephemeral result persisted: %s", address)
			}
		}
		return nil
	}}}})
}

// The release explicitly retains the upstream WriteOnly Action validation defect.
// An upstream fix must fail this expectation so the limitation is reviewed and removed.
func TestAcc_OperationalWriteOnlyKnownFrameworkDefect(t *testing.T) {
	var applies atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/capabilities/ldap/group-mappings/preview":
			_, _ = w.Write([]byte(`{"token":"synthetic-ephemeral-token","additions":[]}`))
		case "/api/capabilities/ldap/group-mappings/apply":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "synthetic-ephemeral-token", body["preview_token"])
			assert.Equal(t, "directory", body["provider_id"])
			applies.Add(1)
			_, _ = w.Write([]byte(`{"status":"applied"}`))
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	resourceTest.Test(t, resourceTest.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resourceTest.TestStep{{Config: fmt.Sprintf(`
 provider "semaphore" {
   api_base_url = %q
   api_token = "synthetic-test-token"
 }
 ephemeral "semaphore_ex_ldap_group_preview" "review" { provider_id = "directory" }
 action "semaphore_ex_ldap_group_apply" "apply" {
   config {
     provider_id = "directory"
     preview_token = ephemeral.semaphore_ex_ldap_group_preview.review.preview_token
   }
 }
 resource "terraform_data" "invoke" {
   lifecycle {
     action_trigger {
       events = [after_create]
       actions = [action.semaphore_ex_ldap_group_apply.apply]
     }
   }
 }
 `, server.URL+"/api"), ExpectError: regexp.MustCompile("WriteOnly Attribute Not Allowed")}}})
	assert.Zero(t, applies.Load(), "known validation failure must precede all apply API calls")
}
