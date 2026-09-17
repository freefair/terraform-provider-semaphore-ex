package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditWebhookConfigureNeverCallsTestEndpoint(t *testing.T) {
	requests := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.URL.Path == "/api/audit-webhook/test" {
			t.Fatal("provider lifecycle must not send test deliveries")
		}
		if r.Method != http.MethodPut || r.URL.Path != "/api/audit-webhook" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["credential"] != "write-only" || body["endpoint"] != "https://audit.example.test/events" {
			t.Fatalf("body = %#v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"endpoint": body["endpoint"], "credential_configured": true, "paused": false, "current_key_id": "current", "current_generation": 1, "signing_revision": 2})
	}))
	defer server.Close()
	plan := exAuditWebhookModel{Endpoint: types.StringValue("https://audit.example.test/events"), Paused: types.BoolValue(false)}
	config := plan
	config.CredentialWO = types.StringValue("write-only")
	state, err := exAuditWebhookConfigure(context.Background(), newEXTestClient(t, server.URL), plan, config)
	if err != nil {
		t.Fatal(err)
	}
	if state.ID.ValueString() != exAuditWebhookID || !state.CredentialConfigured.ValueBool() || len(requests) != 1 {
		t.Fatalf("state=%#v requests=%#v", state, requests)
	}
}

func TestAuditWebhookResponsePreservesWriteOnlyVersion(t *testing.T) {
	old := exAuditWebhookModel{CredentialWOVersion: types.Int64Value(4)}
	next, err := exAuditWebhookFromResponse(old, map[string]any{"endpoint": "https://audit.example.test/events", "credential_configured": true, "paused": false, "signing_revision": 3})
	if err != nil {
		t.Fatal(err)
	}
	if next.CredentialWOVersion.ValueInt64() != 4 || next.SigningRevision.ValueInt64() != 3 {
		t.Fatalf("state=%#v", next)
	}
}

func TestAuditWebhookSigningSecretsFollowGenerations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/audit-webhook/signing-secret" || r.URL.Query().Get("revision") != "2" {
			t.Fatalf("request = %s", r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(exAuditWebhookSigningSecretResponse{Secret: strings.Repeat("a", 32), CurrentKeyID: "current-1", CurrentGeneration: 1, SigningRevision: 3})
	}))
	defer server.Close()
	state, err := exAuditWebhookSigningMutation(context.Background(), newEXTestClient(t, server.URL), exAuditWebhookModel{SigningRevision: types.Int64Value(2)}, "/audit-webhook/signing-secret", false)
	if err != nil || state.CurrentSigningSecret.IsNull() || state.CurrentGeneration.ValueInt64() != 1 {
		t.Fatalf("state=%#v err=%v", state, err)
	}
	state.NextSigningSecret = types.StringValue(strings.Repeat("b", 32))
	state.NextKeyID, state.NextGeneration = types.StringValue("next-2"), types.Int64Value(2)
	promoted, err := exAuditWebhookFromResponse(state, map[string]any{"current_key_id": "next-2", "current_generation": 2, "signing_revision": 4})
	if err != nil || promoted.CurrentSigningSecret.ValueString() != state.NextSigningSecret.ValueString() || !promoted.NextSigningSecret.IsNull() {
		t.Fatalf("promoted=%#v err=%v", promoted, err)
	}
}

func TestAuditWebhookCreateRetainsBootstrapStateWhenStageFails(t *testing.T) {
	ctx := context.Background()
	schema := exAuditWebhookResourceSchema()
	values := map[string]attr.Value{}
	attributeTypes := map[string]attr.Type{}
	for name, attribute := range schema.Attributes {
		attributeTypes[name] = attribute.GetType()
		value, err := exTypedValue(ctx, attribute.GetType(), nil)
		require.NoError(t, err)
		values[name] = value
	}
	values["endpoint"] = types.StringValue("https://audit.example.test/events")
	values["paused"] = types.BoolValue(true)
	values["signing_bootstrap_version"] = types.Int64Value(1)
	values["signing_stage_version"] = types.Int64Value(1)
	object, diagnostics := types.ObjectValue(attributeTypes, values)
	require.False(t, diagnostics.HasError())
	plan := tfsdk.Plan{Schema: schema}
	require.False(t, plan.Set(ctx, object).HasError())
	config := tfsdk.Config{Schema: schema, Raw: plan.Raw}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/audit-webhook":
			_ = json.NewEncoder(w).Encode(map[string]any{"endpoint": "https://audit.example.test/events", "paused": true, "signing_revision": 0})
		case "/api/audit-webhook/signing-secret":
			_ = json.NewEncoder(w).Encode(exAuditWebhookSigningSecretResponse{Secret: strings.Repeat("a", 32), CurrentKeyID: "current", CurrentGeneration: 1, SigningRevision: 1})
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	resourceUnderTest := exAuditWebhookResource{client: newEXTestClient(t, server.URL)}
	response := frameworkresource.CreateResponse{State: tfsdk.State{Schema: schema}}
	resourceUnderTest.Create(ctx, frameworkresource.CreateRequest{Plan: plan, Config: config}, &response)
	require.True(t, response.Diagnostics.HasError())
	var retained exAuditWebhookModel
	require.False(t, response.State.Get(ctx, &retained).HasError())
	assert.Equal(t, exAuditWebhookID, retained.ID.ValueString())
	assert.False(t, retained.CurrentSigningSecret.IsNull())
	assert.Equal(t, int64(1), retained.SigningBootstrapVersion.ValueInt64())
	assert.True(t, retained.SigningStageVersion.IsNull())
}

func TestAuditWebhookUpdateMetadataOnlyPreservesSigningSecrets(t *testing.T) {
	ctx := context.Background()
	schema := exAuditWebhookResourceSchema()
	values, attributeTypes := map[string]attr.Value{}, map[string]attr.Type{}
	for name, attribute := range schema.Attributes {
		attributeTypes[name] = attribute.GetType()
		value, err := exTypedValue(ctx, attribute.GetType(), nil)
		require.NoError(t, err)
		values[name] = value
	}
	values["endpoint"] = types.StringValue("https://new.audit.example.test/events")
	values["paused"] = types.BoolValue(true)
	values["credential_wo_version"] = types.Int64Value(1)
	values["current_signing_secret"] = types.StringUnknown()
	values["next_signing_secret"] = types.StringUnknown()
	planObject, diagnostics := types.ObjectValue(attributeTypes, values)
	require.False(t, diagnostics.HasError())
	plan := tfsdk.Plan{Schema: schema}
	require.False(t, plan.Set(ctx, planObject).HasError())
	configValues := map[string]attr.Value{}
	for name, value := range values {
		configValues[name] = value
	}
	configValues["credential_wo"] = types.StringValue("must-not-be-sent")
	configObject, diagnostics := types.ObjectValue(attributeTypes, configValues)
	require.False(t, diagnostics.HasError())
	configRaw, err := configObject.ToTerraformValue(ctx)
	require.NoError(t, err)
	config := tfsdk.Config{Schema: schema, Raw: configRaw}
	prior := exAuditWebhookModel{ID: types.StringValue(exAuditWebhookID), Endpoint: types.StringValue("https://old.audit.example.test/events"), CredentialWOVersion: types.Int64Value(1), Paused: types.BoolValue(true), CurrentKeyID: types.StringValue("current"), NextKeyID: types.StringValue("next"), CurrentGeneration: types.Int64Value(1), NextGeneration: types.Int64Value(2), SigningRevision: types.Int64Value(3), SigningBootstrapVersion: types.Int64Value(1), SigningStageVersion: types.Int64Value(1), CurrentSigningSecret: types.StringValue(strings.Repeat("a", 32)), NextSigningSecret: types.StringValue(strings.Repeat("b", 32))}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPut || request.URL.Path != "/api/audit-webhook" {
			t.Fatalf("unexpected signing mutation %s %s", request.Method, request.URL.Path)
		}
		var body map[string]any
		require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
		assert.NotContains(t, body, "credential")
		_ = json.NewEncoder(w).Encode(map[string]any{"endpoint": "https://new.audit.example.test/events", "paused": true, "current_key_id": "current", "next_key_id": "next", "current_generation": 1, "next_generation": 2, "signing_revision": 3})
	}))
	defer server.Close()
	priorState := tfsdk.State{Schema: schema}
	require.False(t, priorState.Set(ctx, &prior).HasError())
	response := frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schema}}
	(&exAuditWebhookResource{client: newEXTestClient(t, server.URL)}).Update(ctx, frameworkresource.UpdateRequest{Plan: plan, Config: config, State: priorState}, &response)
	require.False(t, response.Diagnostics.HasError())
	var retained exAuditWebhookModel
	require.False(t, response.State.Get(ctx, &retained).HasError())
	assert.Equal(t, prior.CurrentSigningSecret, retained.CurrentSigningSecret)
	assert.Equal(t, prior.NextSigningSecret, retained.NextSigningSecret)
	assert.Equal(t, prior.SigningBootstrapVersion, retained.SigningBootstrapVersion)
	assert.Equal(t, prior.SigningStageVersion, retained.SigningStageVersion)
}

func TestAuditWebhookUpdateCredentialVersionSendsWriteOnlyMaterialAndPreservesSigningSecrets(t *testing.T) {
	ctx, schema := context.Background(), exAuditWebhookResourceSchema()
	values, attributeTypes := map[string]attr.Value{}, map[string]attr.Type{}
	for name, attribute := range schema.Attributes {
		attributeTypes[name] = attribute.GetType()
		value, err := exTypedValue(ctx, attribute.GetType(), nil)
		require.NoError(t, err)
		values[name] = value
	}
	values["endpoint"], values["paused"], values["credential_wo_version"] = types.StringValue("https://audit.example.test/events"), types.BoolValue(true), types.Int64Value(2)
	values["current_signing_secret"], values["next_signing_secret"] = types.StringUnknown(), types.StringUnknown()
	planObject, diagnostics := types.ObjectValue(attributeTypes, values)
	require.False(t, diagnostics.HasError())
	plan := tfsdk.Plan{Schema: schema}
	require.False(t, plan.Set(ctx, planObject).HasError())
	configValues := map[string]attr.Value{}
	for name, value := range values {
		configValues[name] = value
	}
	configValues["credential_wo"] = types.StringValue("replacement-credential")
	configObject, diagnostics := types.ObjectValue(attributeTypes, configValues)
	require.False(t, diagnostics.HasError())
	configRaw, err := configObject.ToTerraformValue(ctx)
	require.NoError(t, err)
	config := tfsdk.Config{Schema: schema, Raw: configRaw}
	prior := exAuditWebhookModel{ID: types.StringValue(exAuditWebhookID), Endpoint: types.StringValue("https://audit.example.test/events"), CredentialWOVersion: types.Int64Value(1), Paused: types.BoolValue(true), CurrentKeyID: types.StringValue("current"), NextKeyID: types.StringValue("next"), CurrentGeneration: types.Int64Value(1), NextGeneration: types.Int64Value(2), SigningRevision: types.Int64Value(3), SigningBootstrapVersion: types.Int64Value(1), SigningStageVersion: types.Int64Value(1), CurrentSigningSecret: types.StringValue(strings.Repeat("a", 32)), NextSigningSecret: types.StringValue(strings.Repeat("b", 32))}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		require.Equal(t, http.MethodPut, request.Method)
		var body map[string]any
		require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
		assert.Equal(t, "replacement-credential", body["credential"])
		_ = json.NewEncoder(w).Encode(map[string]any{"endpoint": "https://audit.example.test/events", "credential_configured": true, "paused": true, "current_key_id": "current", "next_key_id": "next", "current_generation": 1, "next_generation": 2, "signing_revision": 3})
	}))
	defer server.Close()
	priorState := tfsdk.State{Schema: schema}
	require.False(t, priorState.Set(ctx, &prior).HasError())
	response := frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schema}}
	(&exAuditWebhookResource{client: newEXTestClient(t, server.URL)}).Update(ctx, frameworkresource.UpdateRequest{Plan: plan, Config: config, State: priorState}, &response)
	require.False(t, response.Diagnostics.HasError())
	var retained exAuditWebhookModel
	require.False(t, response.State.Get(ctx, &retained).HasError())
	assert.Equal(t, int64(2), retained.CredentialWOVersion.ValueInt64())
	assert.Equal(t, prior.CurrentSigningSecret, retained.CurrentSigningSecret)
	assert.Equal(t, prior.NextSigningSecret, retained.NextSigningSecret)
}

func TestAcc_EXAuditWebhook(t *testing.T) {
	// The isolated server may attempt delivery after storing its own audit event.
	// A closed loopback port proves that Terraform never uses a public receiver
	// while leaving all delivery-test endpoints untouched.
	config := `
resource "semaphore_ex_audit_webhook" "test" {
  endpoint = "https://127.0.0.1:1/audit"
  credential_wo = "acceptance-write-only-credential"
  credential_wo_version = 1
  signing_bootstrap_version = 1
  signing_stage_version = 1
}
data "semaphore_ex_audit_webhook" "test" {
  depends_on = [semaphore_ex_audit_webhook.test]
}
`
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, CheckDestroy: testAccAuditWebhookPaused, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("semaphore_ex_audit_webhook.test", "id", exAuditWebhookID), resource.TestCheckResourceAttr("data.semaphore_ex_audit_webhook.test", "endpoint", "https://127.0.0.1:1/audit"), resource.TestCheckResourceAttr("semaphore_ex_audit_webhook.test", "credential_configured", "true"), resource.TestCheckResourceAttrSet("semaphore_ex_audit_webhook.test", "current_signing_secret"), resource.TestCheckResourceAttrSet("semaphore_ex_audit_webhook.test", "next_signing_secret"), testAccAuditWebhookSigningLifecycle)},
		{ResourceName: "semaphore_ex_audit_webhook.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"credential_wo_version", "signing_bootstrap_version", "signing_stage_version", "current_signing_secret", "next_signing_secret", "current_generation", "next_generation", "current_key_id", "next_key_id", "signing_revision"}, ImportStateIdFunc: func(*terraform.State) (string, error) { return exAuditWebhookID, nil }},
	}})
}

func testAccAuditWebhookSigningLifecycle(state *terraform.State) error {
	attributes := state.RootModule().Resources["semaphore_ex_audit_webhook.test"].Primary.Attributes
	revision, err := strconv.ParseInt(attributes["signing_revision"], 10, 64)
	if err != nil {
		return err
	}
	previous := exAuditWebhookModel{CurrentKeyID: types.StringValue(attributes["current_key_id"]), NextKeyID: types.StringValue(attributes["next_key_id"]), CurrentGeneration: types.Int64Value(mustAuditInt(attributes["current_generation"])), NextGeneration: types.Int64Value(mustAuditInt(attributes["next_generation"])), CurrentSigningSecret: types.StringValue(attributes["current_signing_secret"]), NextSigningSecret: types.StringValue(attributes["next_signing_secret"])}
	client := testClient()
	if err = exRequestWithOptions(context.Background(), client, http.MethodPost, "/audit-webhook/signing-secret/promote", exRequestOptions{Query: map[string]string{"revision": strconv.FormatInt(revision, 10)}}, nil, nil); err != nil {
		return err
	}
	refreshed, err := exAuditWebhookRead(context.Background(), client, previous)
	if err != nil || refreshed.CurrentSigningSecret.IsNull() || refreshed.NextSigningSecret.IsNull() {
		return fmt.Errorf("promotion did not retain both signing slots")
	}
	if err = exRequestWithOptions(context.Background(), client, http.MethodDelete, "/audit-webhook/signing-secret/next", exRequestOptions{Query: map[string]string{"revision": strconv.FormatInt(refreshed.SigningRevision.ValueInt64(), 10)}}, nil, nil); err != nil {
		return err
	}
	cleared, err := exAuditWebhookRead(context.Background(), client, refreshed)
	if err != nil || !cleared.NextSigningSecret.IsNull() {
		return fmt.Errorf("revoke-next did not clear the staged secret")
	}
	err = exRequestWithOptions(context.Background(), client, http.MethodPost, "/audit-webhook/signing-secret/promote", exRequestOptions{Query: map[string]string{"revision": strconv.FormatInt(revision, 10)}}, nil, nil)
	if err == nil {
		return fmt.Errorf("stale signing revision was accepted")
	}
	return nil
}
func mustAuditInt(value string) int64 { parsed, _ := strconv.ParseInt(value, 10, 64); return parsed }

func testAccAuditWebhookPaused(*terraform.State) error {
	state, err := exAuditWebhookRead(context.Background(), testClient(), exAuditWebhookModel{})
	if err != nil {
		return err
	}
	if !state.Paused.ValueBool() {
		return fmt.Errorf("audit webhook remains unpaused after Terraform destroy")
	}
	return nil
}
