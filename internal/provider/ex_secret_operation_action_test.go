package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretOperationActionsUseOnlyFixedRoutes(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		assert.Equal(t, "Bearer "+exTestToken, r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/api/project/7/secret_storages/9/sync":
			assert.Equal(t, http.MethodPost, r.Method)
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "terraform:sync-20260916", body["request_id"])
			assert.Equal(t, float64(41), body["resolve_operation_id"])
			w.WriteHeader(http.StatusAccepted)
		case "/api/project/7/secret_storages/9/test":
			assert.Equal(t, http.MethodPost, r.Method)
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{"state":"healthy","latency_millis":1}`))
			require.NoError(t, err)
		case "/api/project/7/environment/5/sync":
			assert.Equal(t, http.MethodPost, r.Method)
			w.WriteHeader(http.StatusNoContent)
		case "/api/project/7/keys/12/rotate":
			assert.Equal(t, http.MethodPost, r.Method)
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "ed25519", body["algorithm"])
			assert.Equal(t, true, body["confirm_rotation"])
			w.Header().Set("Content-Type", "application/json")
			_, err := w.Write([]byte(`{"public_key":"ssh-ed25519 public-only","fingerprint":"SHA256:test","algorithm":"ed25519"}`))
			require.NoError(t, err)
		default:
			t.Errorf("unexpected secret operation route %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	invoke := func(value action.Action, config map[string]tftypes.Value) {
		instance := value.(*exSecretOperationAction)
		instance.client = newEXTestClient(t, server.URL)
		var response action.InvokeResponse
		instance.Invoke(context.Background(), action.InvokeRequest{Config: runtimeActionConfig(t, instance, config)}, &response)
		require.False(t, response.Diagnostics.HasError(), "%v", response.Diagnostics)
	}
	invoke(NewProjectSecretStorageSyncAction(), map[string]tftypes.Value{
		"project_id": tftypes.NewValue(tftypes.Number, int64(7)), "storage_id": tftypes.NewValue(tftypes.Number, int64(9)), "request_id": tftypes.NewValue(tftypes.String, "terraform:sync-20260916"), "resolve_operation_id": tftypes.NewValue(tftypes.Number, int64(41)),
	})
	invoke(NewProjectSecretStorageConnectionTestAction(), map[string]tftypes.Value{
		"project_id": tftypes.NewValue(tftypes.Number, int64(7)), "storage_id": tftypes.NewValue(tftypes.Number, int64(9)),
	})
	invoke(NewProjectEnvironmentSyncAction(), map[string]tftypes.Value{
		"project_id": tftypes.NewValue(tftypes.Number, int64(7)), "environment_id": tftypes.NewValue(tftypes.Number, int64(5)),
	})
	invoke(NewProjectGeneratedSSHKeyRotateAction(), map[string]tftypes.Value{
		"project_id": tftypes.NewValue(tftypes.Number, int64(7)), "key_id": tftypes.NewValue(tftypes.Number, int64(12)), "algorithm": tftypes.NewValue(tftypes.String, "ed25519"), "confirm_rotation": tftypes.NewValue(tftypes.Bool, true),
	})
	assert.Equal(t, 4, requests)
}

func TestGeneratedSSHKeyRotationRequiresExplicitConfirmation(t *testing.T) {
	actionValue := NewProjectGeneratedSSHKeyRotateAction().(*exSecretOperationAction)
	var response action.InvokeResponse
	actionValue.Invoke(context.Background(), action.InvokeRequest{Config: runtimeActionConfig(t, actionValue, map[string]tftypes.Value{
		"project_id": tftypes.NewValue(tftypes.Number, int64(7)), "key_id": tftypes.NewValue(tftypes.Number, int64(12)), "algorithm": tftypes.NewValue(tftypes.String, "ed25519"), "confirm_rotation": tftypes.NewValue(tftypes.Bool, false),
	})}, &response)
	assert.True(t, response.Diagnostics.HasError())
}
