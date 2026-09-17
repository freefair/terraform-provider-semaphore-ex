package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func workflowTriggerMappingTypes() map[string]attr.Type {
	return exWorkflowTriggerResourceSchema().Type().(types.ObjectType).AttrTypes["input_mappings"].(types.ListType).ElemType.(types.ObjectType).AttrTypes
}

func workflowTriggerTestMappings(t *testing.T) types.List {
	t.Helper()
	secretTypes := map[string]attr.Type{"access_key_id": types.Int64Type, "global_credential_id": types.Int64Type}
	mapping := types.ObjectValueMust(workflowTriggerMappingTypes(), map[string]attr.Value{
		"parameter": types.StringValue("confirmed"), "source": types.StringValue("fixed"), "key": types.StringNull(), "fixed_string": types.StringNull(), "fixed_integer": types.Int64Null(), "fixed_boolean": types.BoolValue(true), "fixed_secret_reference": types.ObjectNull(secretTypes),
	})
	return types.ListValueMust(types.ObjectType{AttrTypes: workflowTriggerMappingTypes()}, []attr.Value{mapping})
}

func TestEXWorkflowTriggerUsesTypedFixedInputMappings(t *testing.T) {
	model := exWorkflowTriggerModel{Name: types.StringValue("deploy"), Type: types.StringValue("api"), Enabled: types.BoolValue(true), CronFormat: types.StringNull(), InputMappings: workflowTriggerTestMappings(t)}
	body, err := exWorkflowTriggerBody(context.Background(), model, 3)
	require.NoError(t, err)
	assert.Equal(t, int64(3), body["revision"])
	mapping := body["input_mappings"].([]any)[0].(map[string]any)
	assert.Equal(t, true, mapping["value"])
	assert.NotContains(t, mapping, "key")
}

func TestEXWorkflowTriggerRejectsAmbiguousFixedInput(t *testing.T) {
	secretTypes := map[string]attr.Type{"access_key_id": types.Int64Type, "global_credential_id": types.Int64Type}
	mapping := types.ObjectValueMust(workflowTriggerMappingTypes(), map[string]attr.Value{
		"parameter": types.StringValue("confirmed"), "source": types.StringValue("fixed"), "key": types.StringNull(), "fixed_string": types.StringValue("yes"), "fixed_integer": types.Int64Null(), "fixed_boolean": types.BoolValue(true), "fixed_secret_reference": types.ObjectNull(secretTypes),
	})
	model := exWorkflowTriggerModel{Name: types.StringValue("deploy"), Type: types.StringValue("api"), Enabled: types.BoolValue(true), InputMappings: types.ListValueMust(types.ObjectType{AttrTypes: workflowTriggerMappingTypes()}, []attr.Value{mapping})}
	_, err := exWorkflowTriggerBody(context.Background(), model, 0)
	require.ErrorContains(t, err, "exactly one typed fixed value")
}

func TestEXWorkflowTriggerUsesStateRevisionAndPreservesOneTimeSecrets(t *testing.T) {
	state := exWorkflowTriggerModel{ID: types.Int64Value(11), ProjectID: types.Int64Value(7), WorkflowID: types.Int64Value(9), Revision: types.Int64Value(3), Credential: types.StringValue("swt_once"), WebhookSigningSecret: types.StringValue("swhsec_once")}
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		assert.Equal(t, "/api/project/7/workflows/9/triggers/11", r.URL.Path)
		switch requestCount {
		case 1:
			assert.Equal(t, http.MethodGet, r.Method)
			_, _ = w.Write([]byte(`{"id":11,"project_id":7,"workflow_template_id":9,"revision":4,"name":"deploy","type":"api","enabled":true,"input_mappings":[],"credential_generation":1}`))
		case 2:
			assert.Equal(t, http.MethodDelete, r.Method)
			assert.Equal(t, "3", r.URL.Query().Get("expected_revision"))
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	next, err := exWorkflowTriggerRead(context.Background(), newEXTestClient(t, server.URL), state)
	require.NoError(t, err)
	assert.Equal(t, int64(4), next.Revision.ValueInt64())
	assert.Equal(t, "swt_once", next.Credential.ValueString())
	assert.Equal(t, "swhsec_once", next.WebhookSigningSecret.ValueString())
	require.NoError(t, exRequestWithOptions(context.Background(), newEXTestClient(t, server.URL), http.MethodDelete, exWorkflowTriggerRoute(true), exRequestOptions{PathParams: exWorkflowTriggerParams(state), Query: map[string]string{"expected_revision": "3"}}, nil, nil))
}

func testAccWorkflowTriggerFixture(t *testing.T, name string) (int64, int64) {
	t.Helper()
	var project map[string]any
	require.NoError(t, exRequest(context.Background(), testClient(), http.MethodPost, "/projects", nil, map[string]any{"name": name}, &project))
	projectID, err := identityNumber(project["id"])
	require.NoError(t, err)
	t.Cleanup(func() {
		err := exRequest(context.Background(), testClient(), http.MethodDelete, "/project/{project_id}", map[string]string{"project_id": strconv.FormatInt(projectID, 10)}, nil, nil)
		if err != nil && !exNotFound(err) {
			t.Errorf("remove workflow trigger fixture project: %v", err)
		}
	})
	var workflow map[string]any
	require.NoError(t, exRequest(context.Background(), testClient(), http.MethodPost, "/project/{project_id}/workflows", map[string]string{"project_id": strconv.FormatInt(projectID, 10)}, map[string]any{"name": "trigger fixture", "nodes": []any{map[string]any{"id": -1, "kind": "note", "note": "acceptance fixture"}}}, &workflow))
	workflowID, err := identityNumber(workflow["id"])
	require.NoError(t, err)
	return projectID, workflowID
}

func testAccWorkflowTriggerFixtureConfig(projectID, workflowID int64, rotationVersion int64) string {
	return `
resource "semaphore_ex_workflow_trigger" "test" {
  project_id  = ` + strconv.FormatInt(projectID, 10) + `
  workflow_id = ` + strconv.FormatInt(workflowID, 10) + `
  name        = "trigger"
  type        = "api"
  enabled     = false
  credential_rotation_version = ` + strconv.FormatInt(rotationVersion, 10) + `
}
data "semaphore_ex_workflow_trigger" "test" {
  project_id  = ` + strconv.FormatInt(projectID, 10) + `
  workflow_id = ` + strconv.FormatInt(workflowID, 10) + `
  id          = semaphore_ex_workflow_trigger.test.id
}
`
}

func testAccWorkflowTriggerTypeConfig(projectID, workflowID int64, triggerType string) string {
	cron := ""
	if triggerType == "schedule" {
		cron = "\n  cron_format = \"0 0 * * *\""
	}
	return `
resource "semaphore_ex_workflow_trigger" "test" {
  project_id  = ` + strconv.FormatInt(projectID, 10) + `
  workflow_id = ` + strconv.FormatInt(workflowID, 10) + `
  name        = "` + triggerType + `-trigger"
  type        = "` + triggerType + `"
  enabled     = false` + cron + `
}
data "semaphore_ex_workflow_trigger" "test" {
  project_id  = ` + strconv.FormatInt(projectID, 10) + `
  workflow_id = ` + strconv.FormatInt(workflowID, 10) + `
  id          = semaphore_ex_workflow_trigger.test.id
}
`
}

func TestAcc_EXWorkflowTrigger(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance fixture requires TF_ACC before API setup")
	}
	testAccPreCheck(t)
	projectID, workflowID := testAccWorkflowTriggerFixture(t, "workflow-trigger-"+acctest.RandString(8))
	var initialCredential string
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: testAccWorkflowTriggerFixtureConfig(projectID, workflowID, 1), Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttrSet("semaphore_ex_workflow_trigger.test", "id"),
			resource.TestCheckResourceAttrSet("semaphore_ex_workflow_trigger.test", "revision"),
			resource.TestCheckResourceAttr("semaphore_ex_workflow_trigger.test", "enabled", "false"),
			resource.TestCheckResourceAttr("data.semaphore_ex_workflow_trigger.test", "name", "trigger"),
			func(s *terraform.State) error {
				initialCredential = s.RootModule().Resources["semaphore_ex_workflow_trigger.test"].Primary.Attributes["credential"]
				if len(initialCredential) < 5 {
					return fmt.Errorf("create did not return credential material")
				}
				return nil
			},
		)},
		{ResourceName: "semaphore_ex_workflow_trigger.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"credential", "webhook_signing_secret"}, ImportStateIdFunc: func(s *terraform.State) (string, error) {
			r := s.RootModule().Resources["semaphore_ex_workflow_trigger.test"]
			return "project/" + r.Primary.Attributes["project_id"] + "/workflow/" + r.Primary.Attributes["workflow_id"] + "/trigger/" + r.Primary.ID, nil
		}},
		{Config: testAccWorkflowTriggerFixtureConfig(projectID, workflowID, 2), Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_workflow_trigger.test", "credential_rotation_version", "2"), func(s *terraform.State) error {
			current := s.RootModule().Resources["semaphore_ex_workflow_trigger.test"].Primary.Attributes["credential"]
			if current == initialCredential || len(current) < 5 {
				return fmt.Errorf("rotation did not replace credential material")
			}
			return nil
		})},
	}})
}

func TestAcc_EXWorkflowTriggerTypes(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance fixture requires TF_ACC before API setup")
	}
	for _, triggerType := range []string{"manual", "schedule", "webhook"} {
		t.Run(triggerType, func(t *testing.T) {
			testAccPreCheck(t)
			projectID, workflowID := testAccWorkflowTriggerFixture(t, "workflow-trigger-"+triggerType+"-"+acctest.RandString(8))
			resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{
				Config: testAccWorkflowTriggerTypeConfig(projectID, workflowID, triggerType),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("semaphore_ex_workflow_trigger.test", "type", triggerType),
					resource.TestCheckResourceAttr("semaphore_ex_workflow_trigger.test", "enabled", "false"),
					resource.TestCheckResourceAttrSet("semaphore_ex_workflow_trigger.test", "revision"),
					resource.TestCheckResourceAttr("data.semaphore_ex_workflow_trigger.test", "type", triggerType),
				),
			}}})
		})
	}
}

func TestAcc_EXWorkflowTriggerSigningLifecycle(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance fixture requires TF_ACC before API setup")
	}
	projectID, workflowID := testAccWorkflowTriggerFixture(t, "workflow-trigger-signing-"+acctest.RandString(8))
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: testAccWorkflowTriggerTypeConfig(projectID, workflowID, "webhook"), Check: func(s *terraform.State) error {
			r := s.RootModule().Resources["semaphore_ex_workflow_trigger.test"]
			triggerID, err := strconv.ParseInt(r.Primary.ID, 10, 64)
			if err != nil {
				return err
			}
			revision, err := strconv.ParseInt(r.Primary.Attributes["revision"], 10, 64)
			if err != nil {
				return err
			}
			params := map[string]string{"project_id": strconv.FormatInt(projectID, 10), "workflow_id": strconv.FormatInt(workflowID, 10), "trigger_id": strconv.FormatInt(triggerID, 10)}
			var staged struct {
				Trigger              map[string]any `json:"trigger"`
				WebhookSigningSecret string         `json:"webhook_signing_secret"`
			}
			if err = exRequest(context.Background(), testClient(), http.MethodPost, exWorkflowTriggerRoute(true)+"/webhook-signing/stage", params, map[string]any{"revision": revision}, &staged); err != nil {
				return err
			}
			if staged.WebhookSigningSecret == "" {
				return fmt.Errorf("stage did not return one-time signing material")
			}
			stagedRevision, err := identityNumber(staged.Trigger["revision"])
			if err != nil {
				return err
			}
			var promoted map[string]any
			if err = exRequest(context.Background(), testClient(), http.MethodPost, exWorkflowTriggerRoute(true)+"/webhook-signing/promote", params, map[string]any{"revision": stagedRevision}, &promoted); err != nil {
				return err
			}
			promotedRevision, err := identityNumber(promoted["revision"])
			if err != nil {
				return err
			}
			if err = exRequest(context.Background(), testClient(), http.MethodPost, exWorkflowTriggerRoute(true)+"/webhook-signing/revoke", params, map[string]any{"revision": promotedRevision}, nil); err != nil {
				return err
			}
			return nil
		}},
		// Signing actions increment the server revision outside Terraform. Refresh
		// the managed resource before import and final destroy use that CAS token.
		{Config: testAccWorkflowTriggerTypeConfig(projectID, workflowID, "webhook")},
		{ResourceName: "semaphore_ex_workflow_trigger.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"credential", "webhook_signing_secret", "next_webhook_signing_secret", "revision", "current_signing_generation", "current_signing_key_id", "next_signing_generation", "next_signing_key_id"}, ImportStateIdFunc: func(s *terraform.State) (string, error) {
			r := s.RootModule().Resources["semaphore_ex_workflow_trigger.test"]
			return "project/" + r.Primary.Attributes["project_id"] + "/workflow/" + r.Primary.Attributes["workflow_id"] + "/trigger/" + r.Primary.ID, nil
		}},
	}})
}

func TestEXWorkflowTriggerStateClearsSecretForExternalGenerationChange(t *testing.T) {
	old := exWorkflowTriggerModel{Credential: types.StringValue("swt_once"), WebhookSigningSecret: types.StringValue("swhsec_once"), NextWebhookSigningSecret: types.StringValue("swhsec_next"), CredentialGeneration: types.Int64Value(1), CurrentSigningGeneration: types.Int64Value(1), NextSigningGeneration: types.Int64Value(2)}
	next, err := exWorkflowTriggerState(context.Background(), old, map[string]any{"credential_generation": json.Number("2"), "current_signing_generation": json.Number("2"), "next_signing_generation": json.Number("3"), "input_mappings": []any{}})
	require.NoError(t, err)
	require.True(t, next.Credential.IsNull())
	require.True(t, next.WebhookSigningSecret.IsNull())
	require.True(t, next.NextWebhookSigningSecret.IsNull())
}

func TestEXWorkflowTriggerStateClearsOmittedSigningState(t *testing.T) {
	old := exWorkflowTriggerModel{WebhookSigningSecret: types.StringValue("swhsec_current"), NextWebhookSigningSecret: types.StringValue("swhsec_next"), CurrentSigningKeyID: types.StringValue("swhkid_current"), NextSigningKeyID: types.StringValue("swhkid_next"), CurrentSigningGeneration: types.Int64Value(2), NextSigningGeneration: types.Int64Value(3)}
	next, err := exWorkflowTriggerState(context.Background(), old, map[string]any{"id": json.Number("11"), "project_id": json.Number("7"), "workflow_template_id": json.Number("9"), "revision": json.Number("4"), "name": "webhook", "type": "webhook", "enabled": false, "input_mappings": []any{}})
	require.NoError(t, err)
	require.True(t, next.CurrentSigningKeyID.IsNull())
	require.True(t, next.NextSigningKeyID.IsNull())
	require.True(t, next.CurrentSigningGeneration.IsNull())
	require.True(t, next.NextSigningGeneration.IsNull())
	require.True(t, next.WebhookSigningSecret.IsNull())
	require.True(t, next.NextWebhookSigningSecret.IsNull())
}

func TestEXWorkflowTriggerSigningActionsUseRevisionFencedRoutes(t *testing.T) {
	for _, operation := range []string{"promote", "revoke"} {
		t.Run(operation, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, "/api/project/7/workflows/9/triggers/11/webhook-signing/"+operation, r.URL.Path)
				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				require.Equal(t, float64(3), body["revision"])
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			require.NoError(t, exRequest(context.Background(), newEXTestClient(t, server.URL), http.MethodPost, exWorkflowTriggerRoute(true)+"/webhook-signing/"+operation, map[string]string{"project_id": "7", "workflow_id": "9", "trigger_id": "11"}, map[string]any{"revision": int64(3)}, nil))
		})
	}
}

func TestEXWorkflowTriggerResponseRejectsNonIntegerFixedValue(t *testing.T) {
	_, err := exWorkflowTriggerMappingValue(context.Background(), map[string]any{"parameter": "replicas", "source": "fixed", "value": json.Number("1.5")}, workflowTriggerMappingTypes())
	require.ErrorContains(t, err, "integer")
}

func TestEXWorkflowTriggerMetadataUpdatePreservesOneTimeCredential(t *testing.T) {
	ctx := context.Background()
	schema := exWorkflowTriggerResourceSchema()
	mappingType := types.ObjectType{AttrTypes: exWorkflowTriggerMappingTypes()}
	stateModel := exWorkflowTriggerModel{ID: types.Int64Value(11), ProjectID: types.Int64Value(7), WorkflowID: types.Int64Value(9), Name: types.StringValue("before"), Type: types.StringValue("api"), Enabled: types.BoolValue(true), CronFormat: types.StringNull(), InputMappings: types.ListValueMust(mappingType, []attr.Value{}), Revision: types.Int64Value(3), Credential: types.StringValue("swt_once"), WebhookSigningSecret: types.StringNull(), NextWebhookSigningSecret: types.StringNull(), CredentialGeneration: types.Int64Value(1), CurrentSigningKeyID: types.StringNull(), NextSigningKeyID: types.StringNull(), CurrentSigningGeneration: types.Int64Null(), NextSigningGeneration: types.Int64Null(), LastFired: types.StringNull(), LastResult: types.StringNull()}
	planModel := stateModel
	planModel.Name = types.StringValue("after")
	planModel.Credential = types.StringUnknown()
	planModel.WebhookSigningSecret = types.StringUnknown()
	planModel.NextWebhookSigningSecret = types.StringUnknown()
	plan := tfsdk.Plan{Schema: schema}
	require.False(t, plan.Set(ctx, &planModel).HasError())
	state := tfsdk.State{Schema: schema}
	require.False(t, state.Set(ctx, &stateModel).HasError())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPut, r.Method)
		require.Equal(t, "/api/project/7/workflows/9/triggers/11", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":11,"project_id":7,"workflow_template_id":9,"revision":4,"name":"after","type":"api","enabled":true,"input_mappings":[],"credential_generation":1}`))
	}))
	defer server.Close()
	instance := &exWorkflowTriggerResource{client: newEXTestClient(t, server.URL)}
	response := frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schema}}
	instance.Update(ctx, frameworkresource.UpdateRequest{Plan: plan, State: state}, &response)
	require.False(t, response.Diagnostics.HasError(), response.Diagnostics.Errors())
	var next exWorkflowTriggerModel
	require.False(t, response.State.Get(ctx, &next).HasError())
	require.Equal(t, "swt_once", next.Credential.ValueString())
	require.Equal(t, int64(4), next.Revision.ValueInt64())
}

func workflowTriggerLifecycleModel(triggerType string) exWorkflowTriggerModel {
	mappingType := types.ObjectType{AttrTypes: exWorkflowTriggerMappingTypes()}
	return exWorkflowTriggerModel{ID: types.Int64Unknown(), ProjectID: types.Int64Value(7), WorkflowID: types.Int64Value(9), Name: types.StringValue("trigger"), Type: types.StringValue(triggerType), Enabled: types.BoolValue(true), CronFormat: types.StringNull(), InputMappings: types.ListValueMust(mappingType, []attr.Value{}), Revision: types.Int64Unknown(), CredentialRotationVersion: types.Int64Null(), SigningStageVersion: types.Int64Null(), SigningBootstrapVersion: types.Int64Null(), Credential: types.StringUnknown(), WebhookSigningSecret: types.StringUnknown(), NextWebhookSigningSecret: types.StringUnknown(), CredentialGeneration: types.Int64Unknown(), CurrentSigningKeyID: types.StringUnknown(), NextSigningKeyID: types.StringUnknown(), CurrentSigningGeneration: types.Int64Unknown(), NextSigningGeneration: types.Int64Unknown(), LastFired: types.StringUnknown(), LastResult: types.StringUnknown()}
}

func TestEXWorkflowTriggerCreateRejectsMissingOneTimeCredentialAfterPersistingTrigger(t *testing.T) {
	ctx, schema := context.Background(), exWorkflowTriggerResourceSchema()
	model := workflowTriggerLifecycleModel("api")
	plan := tfsdk.Plan{Schema: schema}
	require.False(t, plan.Set(ctx, &model).HasError())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		_, _ = w.Write([]byte(`{"trigger":{"id":11,"project_id":7,"workflow_template_id":9,"revision":1,"name":"trigger","type":"api","enabled":true,"input_mappings":[],"credential_generation":1}}`))
	}))
	defer server.Close()
	response := frameworkresource.CreateResponse{State: tfsdk.State{Schema: schema}}
	(&exWorkflowTriggerResource{client: newEXTestClient(t, server.URL)}).Create(ctx, frameworkresource.CreateRequest{Plan: plan}, &response)
	require.True(t, response.Diagnostics.HasError())
	var state exWorkflowTriggerModel
	require.False(t, response.State.Get(ctx, &state).HasError())
	require.Equal(t, int64(11), state.ID.ValueInt64())
	require.True(t, state.Credential.IsNull())
}

func TestEXWorkflowTriggerRejectsInvalidSigningTypeBeforeCreateRequest(t *testing.T) {
	ctx, schema := context.Background(), exWorkflowTriggerResourceSchema()
	model := workflowTriggerLifecycleModel("api")
	model.SigningStageVersion = types.Int64Value(1)
	plan := tfsdk.Plan{Schema: schema}
	require.False(t, plan.Set(ctx, &model).HasError())
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer server.Close()
	response := frameworkresource.CreateResponse{State: tfsdk.State{Schema: schema}}
	(&exWorkflowTriggerResource{client: newEXTestClient(t, server.URL)}).Create(ctx, frameworkresource.CreateRequest{Plan: plan}, &response)
	require.True(t, response.Diagnostics.HasError())
	assert.False(t, called)
}

func TestEXWorkflowTriggerPersistsMetadataAfterRotateFailure(t *testing.T) {
	ctx, schema := context.Background(), exWorkflowTriggerResourceSchema()
	stateModel := workflowTriggerLifecycleModel("api")
	stateModel.ID, stateModel.Revision, stateModel.CredentialGeneration, stateModel.Credential, stateModel.CredentialRotationVersion = types.Int64Value(11), types.Int64Value(3), types.Int64Value(1), types.StringValue("swt_once"), types.Int64Value(1)
	planModel := stateModel
	planModel.Name, planModel.CredentialRotationVersion, planModel.Credential = types.StringValue("renamed"), types.Int64Value(2), types.StringUnknown()
	plan, state := tfsdk.Plan{Schema: schema}, tfsdk.State{Schema: schema}
	require.False(t, plan.Set(ctx, &planModel).HasError())
	require.False(t, state.Set(ctx, &stateModel).HasError())
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch requests {
		case 1:
			require.Equal(t, http.MethodPut, r.Method)
			_, _ = w.Write([]byte(`{"id":11,"project_id":7,"workflow_template_id":9,"revision":4,"name":"renamed","type":"api","enabled":true,"input_mappings":[],"credential_generation":1}`))
		case 2:
			require.Equal(t, http.MethodPost, r.Method)
			w.WriteHeader(http.StatusConflict)
		}
	}))
	defer server.Close()
	response := frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schema}}
	(&exWorkflowTriggerResource{client: newEXTestClient(t, server.URL)}).Update(ctx, frameworkresource.UpdateRequest{Plan: plan, State: state}, &response)
	require.True(t, response.Diagnostics.HasError())
	var next exWorkflowTriggerModel
	require.False(t, response.State.Get(ctx, &next).HasError())
	assert.Equal(t, int64(4), next.Revision.ValueInt64())
	assert.Equal(t, "renamed", next.Name.ValueString())
	assert.Equal(t, "swt_once", next.Credential.ValueString())
	assert.Equal(t, int64(1), next.CredentialRotationVersion.ValueInt64())
}
