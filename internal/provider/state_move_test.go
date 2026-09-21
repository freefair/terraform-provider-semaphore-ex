package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Non-null nested fixtures exercise shape expansion and retention of secrets;
// a null-only source would miss destructive schema conversion mistakes.
func legacyFixture(typ any) any {
	if name, ok := typ.(string); ok {
		switch name {
		case "string":
			return "preserved"
		case "number":
			return json.Number("9007199254740993")
		case "bool":
			return true
		}
	}
	parts := typ.([]any)
	switch parts[0] {
	case "object":
		values := map[string]any{}
		for name, child := range parts[1].(map[string]any) {
			values[name] = legacyFixture(child)
		}
		return values
	case "map":
		return map[string]any{"kept": legacyFixture(parts[1])}
	case "list", "set":
		return []any{legacyFixture(parts[1])}
	}
	panic(fmt.Sprint(typ))
}

func TestLegacyMovesCoverEveryPublishedResource(t *testing.T) {
	var schemas map[string]struct {
		Type any `json:"type"`
	}
	require.NoError(t, json.Unmarshal([]byte(legacyResourceTypes), &schemas))
	targets := map[string]resource.Resource{}
	for _, newResource := range New("test")().Resources(context.Background()) {
		r := newResource()
		var m resource.MetadataResponse
		r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "semaphoreui"}, &m)
		targets[m.TypeName] = r
	}
	for name, source := range schemas {
		t.Run(name, func(t *testing.T) {
			r := targets[name]
			require.NotNil(t, r)
			mover, ok := r.(resource.ResourceWithMoveState)
			require.True(t, ok)
			values := legacyFixture(source.Type).(map[string]any)
			raw, err := json.Marshal(values)
			require.NoError(t, err)
			var schema resource.SchemaResponse
			r.Schema(context.Background(), resource.SchemaRequest{}, &schema)
			response := resource.MoveStateResponse{TargetState: tfsdk.State{Schema: schema.Schema}}
			mover.MoveState(context.Background())[0].StateMover(context.Background(), resource.MoveStateRequest{SourceProviderAddress: "registry.terraform.io/semaphoreui/semaphore", SourceTypeName: name, SourceSchemaVersion: 0, SourceRawState: &tfprotov6.RawState{JSON: raw}}, &response)
			require.False(t, response.Diagnostics.HasError(), "%v", response.Diagnostics)
			require.False(t, response.TargetState.Raw.IsNull())
			var value types.Object
			require.False(t, response.TargetState.Get(context.Background(), &value).HasError())
			got, err := exWireValue(context.Background(), value)
			require.NoError(t, err)
			output := got.(map[string]any)
			if name == "semaphoreui_runner" || name == "semaphoreui_project_runner" {
				delete(values, "token")
				delete(values, "private_key")
			}
			assertLegacyPreserved(t, values, output)
		})
	}
}
func assertLegacyPreserved(t *testing.T, want, got any) {
	t.Helper()
	switch value := want.(type) {
	case map[string]any:
		actual, ok := got.(map[string]any)
		require.True(t, ok)
		for k, v := range value {
			require.Contains(t, actual, k)
			assertLegacyPreserved(t, v, actual[k])
		}
	case []any:
		actual, ok := got.([]any)
		require.True(t, ok)
		require.Len(t, actual, len(value))
		for i, v := range value {
			assertLegacyPreserved(t, v, actual[i])
		}
	case json.Number:
		expected, err := value.Int64()
		require.NoError(t, err)
		assert.Equal(t, expected, got)
	default:
		assert.Equal(t, want, got)
	}
}
func TestLegacyMoveRejectsUnrelatedOrMalformedState(t *testing.T) {
	r := NewProjectResource()
	var schema resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &schema)
	for _, test := range []struct {
		name, address, typ string
		version            int64
		data               string
	}{
		{"wrong provider", "registry.terraform.io/attacker/semaphore", "semaphoreui_project", 0, `{"id":1}`},
		{"suffix spoof", "registry.terraform.io/notsemaphoreui/semaphore", "semaphoreui_project", 0, `{"id":1}`},
		{"wrong type", "registry.terraform.io/semaphoreui/semaphore", "semaphoreui_user", 0, `{"id":1}`},
		{"future schema", "registry.terraform.io/semaphoreui/semaphore", "semaphoreui_project", 1, `{"id":1}`},
		{"malformed", "registry.terraform.io/semaphoreui/semaphore", "semaphoreui_project", 0, `{`},
		{"null", "registry.terraform.io/semaphoreui/semaphore", "semaphoreui_project", 0, `null`},
		{"missing identity", "registry.terraform.io/semaphoreui/semaphore", "semaphoreui_project", 0, `{"id":null}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := resource.MoveStateResponse{TargetState: tfsdk.State{Schema: schema.Schema}}
			r.(resource.ResourceWithMoveState).MoveState(context.Background())[0].StateMover(context.Background(), resource.MoveStateRequest{SourceProviderAddress: test.address, SourceTypeName: test.typ, SourceSchemaVersion: test.version, SourceRawState: &tfprotov6.RawState{JSON: []byte(test.data)}}, &response)
			assert.True(t, response.TargetState.Raw.IsNull())
		})
	}
}

func TestLegacyTokenMoveRejectsMissingRunnerIdentity(t *testing.T) {
	ctx := context.Background()
	r := NewRunnerRegistrationTokenResource()
	var schema resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schema)
	response := resource.MoveStateResponse{TargetState: tfsdk.State{Schema: schema.Schema}}
	r.(resource.ResourceWithMoveState).MoveState(ctx)[0].StateMover(ctx, resource.MoveStateRequest{
		SourceProviderAddress: "registry.terraform.io/semaphoreui/semaphore",
		SourceTypeName:        "semaphoreui_runner_registration_token",
		SourceRawState:        &tfprotov6.RawState{JSON: []byte(`{"id":"runner/7","runner_id":null,"project_id":null,"registration_token":null,"keepers":null}`)},
	}, &response)
	require.True(t, response.Diagnostics.HasError())
	assert.True(t, response.TargetState.Raw.IsNull())
}
