package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestTaskStartExposesInvocationParameters(t *testing.T) {
	var schema action.SchemaResponse
	NewProjectTaskStartAction().Schema(context.Background(), action.SchemaRequest{}, &schema)
	for _, name := range []string{"params", "version", "build_task_id", "commit_hash", "secret"} {
		require.Contains(t, schema.Schema.Attributes, name)
	}
	require.True(t, schema.Schema.Attributes["secret"].IsWriteOnly())
}

func TestTaskStartPostsCompleteParameters(t *testing.T) {
	for _, app := range []string{"ansible", "terraform"} {
		t.Run(app, func(t *testing.T) {
			posts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodGet {
					require.Equal(t, "/api/project/1/templates/2", r.URL.Path)
					_, _ = fmt.Fprintf(w, `{"id":2,"project_id":1,"allow_override_branch_in_task":true,"app":%q,"task_params":{"allow_debug":true,"allow_override_limit":true,"allow_override_tags":true,"allow_override_skip_tags":true,"allow_override_skip_galaxy_install":true,"allow_auto_approve":true}}`, app)
					return
				}
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, "/api/project/1/tasks", r.URL.Path)
				posts++
				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, "deadbeef", body["commit_hash"])
				assert.Equal(t, "requested-version", body["version"])
				assert.Equal(t, float64(8), body["build_task_id"])
				assert.JSONEq(t, `{"token":"synthetic-survey-secret"}`, body["secret"].(string))
				params := body["params"].(map[string]any)
				if app == "ansible" {
					assert.Equal(t, []any{"web"}, params["limit"])
					assert.Equal(t, true, params["debug"])
					assert.Equal(t, float64(3), params["debug_level"])
					assert.Equal(t, true, params["diff"])
				} else {
					assert.Equal(t, true, params["plan"])
					assert.Equal(t, true, params["destroy"])
					assert.Equal(t, true, params["auto_approve"])
					assert.Equal(t, true, params["reconfigure"])
				}
				if directory := os.Getenv("SEMAPHORE_PROVIDER_WIRE_EVIDENCE"); directory != "" {
					delete(body, "secret")
					encoded, err := json.MarshalIndent(body, "", "  ")
					require.NoError(t, err)
					require.NoError(t, os.WriteFile(filepath.Join(directory, "task-start-"+app+"-wire.json"), encoded, 0600))
				}
				_, _ = w.Write([]byte(`{"id":12}`))
			}))
			defer server.Close()
			instance := NewProjectTaskStartAction().(*exRuntimeAction)
			instance.client = newEXTestClient(t, server.URL)
			var schema action.SchemaResponse
			instance.Schema(context.Background(), action.SchemaRequest{}, &schema)
			params := map[string]any{"debug": true, "debug_level": int64(3), "limit": []tftypes.Value{tftypes.NewValue(tftypes.String, "web")}, "tags": []tftypes.Value{tftypes.NewValue(tftypes.String, "deploy")}, "skip_tags": []tftypes.Value{}, "diff": true, "dry_run": true, "skip_galaxy_install": true}
			if app == "terraform" {
				params = map[string]any{"plan": true, "destroy": true, "auto_approve": true, "upgrade": true, "reconfigure": true}
			}
			secret, err := types.DynamicValue(types.ObjectValueMust(map[string]attr.Type{"token": types.StringType}, map[string]attr.Value{"token": types.StringValue("synthetic-survey-secret")})).ToTerraformValue(context.Background())
			require.NoError(t, err)
			config := tfsdk.Config{Schema: schema.Schema, Raw: unknownConfigObject(schema.Schema.Type().TerraformType(context.Background()), map[string]any{"project_id": int64(1), "template_id": int64(2), "params": params, "version": "requested-version", "build_task_id": int64(8), "commit_hash": "deadbeef", "secret": secret})}
			var response action.InvokeResponse
			instance.Invoke(context.Background(), action.InvokeRequest{Config: config}, &response)
			require.False(t, response.Diagnostics.HasError(), "%v", response.Diagnostics)
			assert.Equal(t, 1, posts)
		})
	}
}

func TestTaskStartRejectsIgnoredOverrideBeforePosting(t *testing.T) {
	for _, tc := range []struct {
		name, app, settings string
		params              map[string]any
	}{
		{"limit disabled", "ansible", `{}`, map[string]any{"limit": []tftypes.Value{tftypes.NewValue(tftypes.String, "web")}}},
		{"hidden diff", "ansible", `{"hide_diff":true}`, map[string]any{"diff": true}},
		{"debug disabled", "ansible", `{}`, map[string]any{"debug": true}},
		{"wrong family", "ansible", `{}`, map[string]any{"destroy": true}},
		{"approval disabled", "terraform", `{}`, map[string]any{"auto_approve": true}},
		{"forced approval", "terraform", `{"auto_approve":true}`, map[string]any{"auto_approve": false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					posts++
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{"app":%q,"task_params":%s}`, tc.app, tc.settings)
			}))
			defer server.Close()
			instance := NewProjectTaskStartAction().(*exRuntimeAction)
			instance.client = newEXTestClient(t, server.URL)
			var schema action.SchemaResponse
			instance.Schema(context.Background(), action.SchemaRequest{}, &schema)
			var response action.InvokeResponse
			instance.Invoke(context.Background(), action.InvokeRequest{Config: tfsdk.Config{Schema: schema.Schema, Raw: unknownConfigObject(schema.Schema.Type().TerraformType(context.Background()), map[string]any{"project_id": int64(1), "template_id": int64(2), "params": tc.params})}}, &response)
			assert.True(t, response.Diagnostics.HasError())
			assert.Zero(t, posts)
		})
	}
}

func TestTaskStartRejectsLockedTopLevelOverrides(t *testing.T) {
	arguments, _ := types.DynamicValue(types.ListValueMust(types.StringType, []attr.Value{types.StringValue("-v")})).ToTerraformValue(context.Background())
	for name, value := range map[string]any{"commit_hash": "deadbeef", "git_branch": "release", "inventory_id": int64(3), "arguments": arguments} {
		t.Run(name, func(t *testing.T) {
			posts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					posts++
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"app":"ansible","task_params":{}}`))
			}))
			defer server.Close()
			instance := NewProjectTaskStartAction().(*exRuntimeAction)
			instance.client = newEXTestClient(t, server.URL)
			var schema action.SchemaResponse
			instance.Schema(context.Background(), action.SchemaRequest{}, &schema)
			config := map[string]any{"project_id": int64(1), "template_id": int64(2), name: value}
			var response action.InvokeResponse
			instance.Invoke(context.Background(), action.InvokeRequest{Config: tfsdk.Config{Schema: schema.Schema, Raw: unknownConfigObject(schema.Schema.Type().TerraformType(context.Background()), config)}}, &response)
			assert.True(t, response.Diagnostics.HasError())
			assert.Zero(t, posts)
		})
	}
}
