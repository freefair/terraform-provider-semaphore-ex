package provider

import (
	"context"
	"encoding/json"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTriggerOmittedInputsSendsValidJSON(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body struct {
			Inputs map[string]json.RawMessage `json:"inputs"`
		}
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		require.NotNil(t, body.Inputs)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	defer server.Close()
	spec := operationalSpecs()["workflow_trigger_test"]
	instance := operationalAction{spec: spec, client: newEXTestClient(t, server.URL)}
	var response action.InvokeResponse
	instance.Invoke(context.Background(), action.InvokeRequest{Config: operationTestConfig(t, spec, map[string]any{"project_id": int64(1), "workflow_id": int64(2), "trigger_id": int64(3)})}, &response)
	require.False(t, response.Diagnostics.HasError(), "%v", response.Diagnostics)
	assert.Equal(t, 1, calls)
}
func TestGuardrailFixtureRequiresAuthoredYAML(t *testing.T) {
	for _, scope := range []string{"global", "project"} {
		require.True(t, operationalSpecs()[scope+"_policy_guardrail_test"].inputs["source_yaml"].IsRequired())
	}
}
