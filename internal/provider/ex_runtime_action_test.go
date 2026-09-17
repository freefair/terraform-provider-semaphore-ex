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
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runtimeActionConfig(t *testing.T, value action.Action, configured map[string]tftypes.Value) tfsdk.Config {
	t.Helper()
	ctx := context.Background()
	var response action.SchemaResponse
	value.Schema(ctx, action.SchemaRequest{}, &response)
	values := make(map[string]tftypes.Value, len(response.Schema.Attributes))
	for name, attribute := range response.Schema.Attributes {
		values[name] = tftypes.NewValue(attribute.GetType().TerraformType(ctx), nil)
	}
	for name, configuredValue := range configured {
		values[name] = configuredValue
	}
	return tfsdk.Config{Schema: response.Schema, Raw: tftypes.NewValue(response.Schema.Type().TerraformType(ctx), values)}
}

func TestRuntimeActionsUseOnlyBoundedServerRoutes(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		assert.Equal(t, "Bearer "+exTestToken, r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/api/project/7/tasks":
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "fingerprint", r.Header.Get(runtimePreflightFingerprintHeader))
			assert.Equal(t, "token", r.Header.Get(runtimePreflightTokenHeader))
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, float64(3), body["template_id"])
			_, err := w.Write([]byte(`{"id":11}`))
			require.NoError(t, err)
		case "/api/project/7/tasks/11/stop":
			assert.Equal(t, http.MethodPost, r.Method)
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, true, body["force"])
			w.WriteHeader(http.StatusNoContent)
		case "/api/project/7/workflows/13/run":
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "fingerprint", r.Header.Get(runtimePreflightFingerprintHeader))
			assert.Empty(t, r.Header.Get(runtimePreflightTokenHeader))
			_, err := w.Write([]byte(`{"id":17}`))
			require.NoError(t, err)
		case "/api/project/7/workflows/13/runs/17/stop":
			assert.Equal(t, http.MethodPost, r.Method)
			w.WriteHeader(http.StatusOK)
		case "/api/project/7/workflows/13/runs/17/approvals/19":
			assert.Equal(t, http.MethodPost, r.Method)
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "approved", body["status"])
			assert.Equal(t, "user", body["source"])
			assert.Equal(t, "reviewed", body["comment"])
			_, err := w.Write([]byte(`{"id":23}`))
			require.NoError(t, err)
		default:
			t.Errorf("unexpected runtime route %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	ctx := context.Background()
	invoke := func(value action.Action, config map[string]tftypes.Value) {
		instance := value.(*exRuntimeAction)
		instance.client = newEXTestClient(t, server.URL)
		var response action.InvokeResponse
		instance.Invoke(ctx, action.InvokeRequest{Config: runtimeActionConfig(t, instance, config)}, &response)
		require.False(t, response.Diagnostics.HasError(), "%v", response.Diagnostics)
	}
	invoke(NewProjectTaskStartAction(), map[string]tftypes.Value{
		"project_id": tftypes.NewValue(tftypes.Number, int64(7)), "template_id": tftypes.NewValue(tftypes.Number, int64(3)),
		"preflight_fingerprint": tftypes.NewValue(tftypes.String, "fingerprint"), "preflight_token": tftypes.NewValue(tftypes.String, "token"),
	})
	invoke(NewProjectTaskStopAction(), map[string]tftypes.Value{
		"project_id": tftypes.NewValue(tftypes.Number, int64(7)), "task_id": tftypes.NewValue(tftypes.Number, int64(11)), "force": tftypes.NewValue(tftypes.Bool, true),
	})
	invoke(NewProjectWorkflowStartAction(), map[string]tftypes.Value{
		"project_id": tftypes.NewValue(tftypes.Number, int64(7)), "workflow_id": tftypes.NewValue(tftypes.Number, int64(13)), "preflight_fingerprint": tftypes.NewValue(tftypes.String, "fingerprint"),
	})
	invoke(NewProjectWorkflowStopAction(), map[string]tftypes.Value{
		"project_id": tftypes.NewValue(tftypes.Number, int64(7)), "workflow_id": tftypes.NewValue(tftypes.Number, int64(13)), "run_id": tftypes.NewValue(tftypes.Number, int64(17)),
	})
	invoke(NewProjectWorkflowApprovalAction(), map[string]tftypes.Value{
		"project_id": tftypes.NewValue(tftypes.Number, int64(7)), "workflow_id": tftypes.NewValue(tftypes.Number, int64(13)), "run_id": tftypes.NewValue(tftypes.Number, int64(17)), "node_id": tftypes.NewValue(tftypes.Number, int64(19)), "status": tftypes.NewValue(tftypes.String, "approved"), "comment": tftypes.NewValue(tftypes.String, "reviewed"),
	})
	assert.Equal(t, 5, requests)
}

func TestRuntimeJSONEncodingPreservesNativeScalars(t *testing.T) {
	value := types.DynamicValue(types.ObjectValueMust(map[string]attr.Type{
		"enabled": types.BoolType, "retries": types.Int64Type,
	}, map[string]attr.Value{
		"enabled": types.BoolValue(true), "retries": types.Int64Value(2),
	}))
	var response action.InvokeResponse
	encoded, ok := runtimeJSONString(context.Background(), &response, "environment", value)
	require.True(t, ok)
	require.NotNil(t, encoded)
	assert.JSONEq(t, `{"enabled":true,"retries":2}`, *encoded)
}

func TestAcc_EXProjectWorkflowStartNoteOnly(t *testing.T) {
	suffix := acctest.RandString(8)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: fmt.Sprintf(`resource "semaphore_ex_project" "runtime" { name = "runtime-action-%s" }`, suffix),
			Check: func(state *terraform.State) error {
				project, found := state.RootModule().Resources["semaphore_ex_project.runtime"]
				if !found {
					return fmt.Errorf("acceptance project is missing")
				}
				projectID, err := strconv.ParseInt(project.Primary.ID, 10, 64)
				if err != nil {
					return fmt.Errorf("parse acceptance project ID: %w", err)
				}

				var workflow map[string]any
				// The server rejects a graph containing only notes because a run needs
				// one executable node. A delay is server-internal and starts no task,
				// runner, repository checkout, or external notification.
				if err := exRequest(context.Background(), testClient(), http.MethodPost, "/project/{project_id}/workflows", map[string]string{"project_id": strconv.FormatInt(projectID, 10)}, map[string]any{
					"name": "note and delay runtime action", "nodes": []any{
						map[string]any{"id": -1, "kind": "note", "note": "no task nodes"},
						map[string]any{"id": -2, "kind": "delay", "delay_seconds": 60},
					},
				}, &workflow); err != nil {
					return fmt.Errorf("create note-and-delay workflow: %w", err)
				}
				workflowID, err := identityNumber(workflow["id"])
				if err != nil {
					return fmt.Errorf("read note-and-delay workflow identity: %w", err)
				}

				actionValue := NewProjectWorkflowStartAction().(*exRuntimeAction)
				actionValue.client = testClient()
				var actionResponse action.InvokeResponse
				actionValue.Invoke(context.Background(), action.InvokeRequest{Config: runtimeActionConfig(t, actionValue, map[string]tftypes.Value{
					"project_id": tftypes.NewValue(tftypes.Number, projectID), "workflow_id": tftypes.NewValue(tftypes.Number, workflowID),
				})}, &actionResponse)
				if actionResponse.Diagnostics.HasError() {
					return fmt.Errorf("start note-and-delay workflow: %v", actionResponse.Diagnostics)
				}

				var runs []struct {
					ID     json.Number `json:"id"`
					Status string      `json:"status"`
				}
				if err := exRequest(context.Background(), testClient(), http.MethodGet, "/project/{project_id}/workflows/{workflow_id}/runs", map[string]string{"project_id": strconv.FormatInt(projectID, 10), "workflow_id": strconv.FormatInt(workflowID, 10)}, nil, &runs); err != nil {
					return fmt.Errorf("read note-and-delay workflow run: %w", err)
				}
				if len(runs) != 1 || runs[0].ID.String() == "" || runs[0].Status == "" {
					return fmt.Errorf("note-and-delay workflow run identity/status not returned: %#v", runs)
				}
				return nil
			},
		}},
	})
}
