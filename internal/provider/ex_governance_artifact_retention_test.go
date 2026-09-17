package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEXWorkflowArtifactRetentionPreservesEffectiveProvenance(t *testing.T) {
	raw := map[string]any{
		"global_policy": nil, "project_policy": nil,
		"effective": map[string]any{"global_revision": int64(0), "project_revision": int64(0), "retention_seconds": int64(2_592_000), "max_artifact_bytes": int64(1_048_576), "max_run_bytes": int64(2_097_152)},
	}
	state, err := exWorkflowArtifactRetentionFromResponse(context.Background(), exWorkflowArtifactRetentionModel{}, raw, false)
	require.NoError(t, err)
	assert.Equal(t, "global", state.ID.ValueString())
	assert.True(t, state.GlobalPolicy.IsNull())
	assert.Equal(t, int64(0), state.GlobalRevision.ValueInt64())

	state, err = exWorkflowArtifactRetentionFromResponse(context.Background(), exWorkflowArtifactRetentionModel{ProjectID: types.Int64Value(7)}, raw, true)
	require.NoError(t, err)
	assert.Equal(t, "project/7", state.ID.ValueString())
	assert.Equal(t, int64(7), state.ProjectID.ValueInt64())
}
