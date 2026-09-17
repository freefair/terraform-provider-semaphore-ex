package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"
)

func TestDeploymentWindowReadRemovesMissingProject(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	defer server.Close()
	state := tfsdk.State{Schema: exProjectDeploymentWindowResourceSchema()}
	require.False(t, state.Set(ctx, &exProjectDeploymentWindowModel{ID: types.Int64Value(7), ProjectID: types.Int64Value(7), Revision: types.Int64Value(1), Timezone: types.StringValue("UTC"), Default: types.StringValue("allow"), Rules: deploymentWindowRules(t)}).HasError())
	response := frameworkresource.ReadResponse{State: state}
	(&exProjectDeploymentWindowResource{client: newEXTestClient(t, server.URL)}).Read(ctx, frameworkresource.ReadRequest{State: state}, &response)
	require.False(t, response.Diagnostics.HasError(), response.Diagnostics)
	require.True(t, response.State.Raw.IsNull())
}

func TestGlobalRetentionRejectsProjectScopeBeforeRequest(t *testing.T) {
	operation := &exWorkflowArtifactRetentionPublishAction{}
	response := action.InvokeResponse{}
	operation.Invoke(context.Background(), action.InvokeRequest{Config: governanceActionConfig(t, operation, map[string]tftypes.Value{
		"project_id": tftypes.NewValue(tftypes.Number, int64(7)), "expected_revision": tftypes.NewValue(tftypes.Number, int64(0)),
		"retention_seconds": tftypes.NewValue(tftypes.Number, int64(3600)), "max_artifact_bytes": tftypes.NewValue(tftypes.Number, int64(1024)), "max_run_bytes": tftypes.NewValue(tftypes.Number, int64(4096)),
	})}, &response)
	require.True(t, response.Diagnostics.HasError())
	require.Contains(t, response.Diagnostics.Errors()[0].Detail(), "project_id")
}
