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
	"github.com/stretchr/testify/require"
)

func governanceActionConfig(t *testing.T, actionValue action.Action, values map[string]tftypes.Value) tfsdk.Config {
	t.Helper()
	var response action.SchemaResponse
	actionValue.Schema(context.Background(), action.SchemaRequest{}, &response)
	return tfsdk.Config{Schema: response.Schema, Raw: tftypes.NewValue(response.Schema.Type().TerraformType(context.Background()), values)}
}

func nativeGuardrailPolicy(t *testing.T) tftypes.Value {
	t.Helper()
	value := types.DynamicValue(types.ObjectValueMust(map[string]attr.Type{"version": types.Int64Type, "rules": types.ListType{ElemType: types.StringType}}, map[string]attr.Value{"version": types.Int64Value(1), "rules": types.ListValueMust(types.StringType, []attr.Value{})}))
	result, err := value.ToTerraformValue(context.Background())
	require.NoError(t, err)
	return result
}

func TestAcc_EXGovernanceActions(t *testing.T) {
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: `data "semaphore_ex_global_workflow_artifact_retention" "test" {}`, Check: func(snapshot *terraform.State) error {
		ctx := context.Background()
		retention := &exWorkflowArtifactRetentionPublishAction{client: testClient()}
		publish := func(expected int64) action.InvokeResponse {
			response := action.InvokeResponse{}
			retention.Invoke(ctx, action.InvokeRequest{Config: governanceActionConfig(t, retention, map[string]tftypes.Value{
				"project_id":        tftypes.NewValue(tftypes.Number, nil),
				"expected_revision": tftypes.NewValue(tftypes.Number, expected), "retention_seconds": tftypes.NewValue(tftypes.Number, int64(2_592_000)),
				"max_artifact_bytes": tftypes.NewValue(tftypes.Number, int64(1_048_576)), "max_run_bytes": tftypes.NewValue(tftypes.Number, int64(2_097_152)),
			})}, &response)
			return response
		}
		currentRevision, err := strconv.ParseInt(snapshot.RootModule().Resources["data.semaphore_ex_global_workflow_artifact_retention.test"].Primary.Attributes["global_revision"], 10, 64)
		if err != nil {
			return err
		}
		first := publish(currentRevision)
		if first.Diagnostics.HasError() {
			return fmt.Errorf("publish initial retention: %v", first.Diagnostics)
		}
		stale := publish(currentRevision)
		if !stale.Diagnostics.HasError() {
			return fmt.Errorf("stale retention revision unexpectedly succeeded")
		}

		var initial struct {
			Draft struct {
				Revision int64 `json:"revision"`
			} `json:"draft"`
		}
		if err := exRequest(ctx, testClient(), http.MethodGet, "/policy-guardrails", nil, nil, &initial); err != nil {
			return err
		}
		draft := &exPolicyGuardrailAction{client: testClient()}
		draftResponse := action.InvokeResponse{}
		draft.Invoke(ctx, action.InvokeRequest{Config: governanceActionConfig(t, draft, map[string]tftypes.Value{"project_id": tftypes.NewValue(tftypes.Number, nil), "expected_revision": tftypes.NewValue(tftypes.Number, initial.Draft.Revision), "source_yaml": tftypes.NewValue(tftypes.String, nil), "policy": nativeGuardrailPolicy(t)})}, &draftResponse)
		if draftResponse.Diagnostics.HasError() {
			return fmt.Errorf("save draft: %v", draftResponse.Diagnostics)
		}
		publishGuardrail := &exPolicyGuardrailAction{client: testClient(), publish: true}
		publishResponse := action.InvokeResponse{}
		publishGuardrail.Invoke(ctx, action.InvokeRequest{Config: governanceActionConfig(t, publishGuardrail, map[string]tftypes.Value{"project_id": tftypes.NewValue(tftypes.Number, nil), "expected_revision": tftypes.NewValue(tftypes.Number, initial.Draft.Revision+1), "source_yaml": tftypes.NewValue(tftypes.String, nil), "policy": tftypes.NewValue(tftypes.DynamicPseudoType, nil)})}, &publishResponse)
		if publishResponse.Diagnostics.HasError() {
			return fmt.Errorf("publish draft: %v", publishResponse.Diagnostics)
		}
		var state struct {
			Draft struct {
				SourceYAML     string `json:"source_yaml"`
				Revision       int64  `json:"revision"`
				ActiveRevision *int64 `json:"active_revision"`
			} `json:"draft"`
		}
		if err := exRequest(ctx, testClient(), "GET", "/policy-guardrails", nil, nil, &state); err != nil {
			return err
		}
		if state.Draft.SourceYAML == "" || state.Draft.Revision < initial.Draft.Revision+2 || state.Draft.ActiveRevision == nil {
			return fmt.Errorf("unexpected published guardrail state: source=%q revision=%d active=%v", state.Draft.SourceYAML, state.Draft.Revision, state.Draft.ActiveRevision)
		}
		var project map[string]any
		if err := exRequest(ctx, testClient(), http.MethodPost, "/projects", nil, map[string]any{"name": "guardrail-" + acctest.RandString(8)}, &project); err != nil {
			return err
		}
		projectID, err := identityNumber(project["id"])
		if err != nil {
			return err
		}
		t.Cleanup(func() {
			require.NoError(t, exRequest(context.Background(), testClient(), http.MethodDelete, "/project/{project_id}", map[string]string{"project_id": strconv.FormatInt(projectID, 10)}, nil, nil))
		})
		projectDraft := &exPolicyGuardrailAction{client: testClient(), project: true}
		projectDraftResponse := action.InvokeResponse{}
		projectDraft.Invoke(ctx, action.InvokeRequest{Config: governanceActionConfig(t, projectDraft, map[string]tftypes.Value{"project_id": tftypes.NewValue(tftypes.Number, projectID), "expected_revision": tftypes.NewValue(tftypes.Number, int64(1)), "source_yaml": tftypes.NewValue(tftypes.String, nil), "policy": nativeGuardrailPolicy(t)})}, &projectDraftResponse)
		if projectDraftResponse.Diagnostics.HasError() {
			return fmt.Errorf("save project draft: %v", projectDraftResponse.Diagnostics)
		}
		projectPublish := &exPolicyGuardrailAction{client: testClient(), project: true, publish: true}
		publishProject := func(revision int64) action.InvokeResponse {
			response := action.InvokeResponse{}
			projectPublish.Invoke(ctx, action.InvokeRequest{Config: governanceActionConfig(t, projectPublish, map[string]tftypes.Value{"project_id": tftypes.NewValue(tftypes.Number, projectID), "expected_revision": tftypes.NewValue(tftypes.Number, revision), "source_yaml": tftypes.NewValue(tftypes.String, nil), "policy": tftypes.NewValue(tftypes.DynamicPseudoType, nil)})}, &response)
			return response
		}
		if response := publishProject(2); response.Diagnostics.HasError() {
			return fmt.Errorf("publish project draft: %v", response.Diagnostics)
		}
		if response := publishProject(2); !response.Diagnostics.HasError() {
			return fmt.Errorf("stale project guardrail revision unexpectedly succeeded")
		}
		return nil
	}}}})
}

func TestGovernanceActionConfigUsesNativeRequiredInputs(t *testing.T) {
	retention := &exWorkflowArtifactRetentionPublishAction{}
	config := governanceActionConfig(t, retention, map[string]tftypes.Value{"project_id": tftypes.NewValue(tftypes.Number, nil), "expected_revision": tftypes.NewValue(tftypes.Number, int64(0)), "retention_seconds": tftypes.NewValue(tftypes.Number, int64(3600)), "max_artifact_bytes": tftypes.NewValue(tftypes.Number, int64(1)), "max_run_bytes": tftypes.NewValue(tftypes.Number, int64(1))})
	require.NotNil(t, config.Schema)
}

func TestPolicyGuardrailNativeYAMLPreservesBooleanAndNumberScalars(t *testing.T) {
	encoded, err := exPolicyGuardrailNativeYAML(map[string]any{"version": json.Number("1"), "enabled": true, "rules": []any{map[string]any{"operand": json.Number("42")}}})
	require.NoError(t, err)
	require.Contains(t, encoded, "version: 1")
	require.Contains(t, encoded, "enabled: true")
	require.Contains(t, encoded, "operand: 42")
}

func TestPolicyGuardrailDraftActionAcceptsNativeHCLDynamicValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/api/policy-guardrails/draft", r.URL.Path)
		var body struct {
			SourceYAML       string `json:"source_yaml"`
			ExpectedRevision int64  `json:"expected_revision"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, int64(1), body.ExpectedRevision)
		require.Contains(t, body.SourceYAML, "version: 1")
		require.Contains(t, body.SourceYAML, "enabled: true")
		require.Contains(t, body.SourceYAML, "limit: 42")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	actionValue := &exPolicyGuardrailAction{client: newEXTestClient(t, server.URL)}
	policyValue := types.DynamicValue(types.ObjectValueMust(map[string]attr.Type{"version": types.Int64Type, "enabled": types.BoolType, "limit": types.Int64Type, "rules": types.ListType{ElemType: types.StringType}}, map[string]attr.Value{"version": types.Int64Value(1), "enabled": types.BoolValue(true), "limit": types.Int64Value(42), "rules": types.ListValueMust(types.StringType, []attr.Value{})}))
	policy, err := policyValue.ToTerraformValue(context.Background())
	require.NoError(t, err)
	response := action.InvokeResponse{}
	actionValue.Invoke(context.Background(), action.InvokeRequest{Config: governanceActionConfig(t, actionValue, map[string]tftypes.Value{"project_id": tftypes.NewValue(tftypes.Number, nil), "expected_revision": tftypes.NewValue(tftypes.Number, int64(1)), "source_yaml": tftypes.NewValue(tftypes.String, nil), "policy": policy})}, &response)
	require.False(t, response.Diagnostics.HasError(), response.Diagnostics.Errors())
}
