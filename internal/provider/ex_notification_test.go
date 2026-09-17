package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func notificationSet(t *testing.T, values ...string) types.Set {
	t.Helper()
	value, diagnostics := types.SetValueFrom(context.Background(), types.StringType, values)
	if diagnostics.HasError() {
		t.Fatal(diagnostics.Errors())
	}
	return value
}

func TestNotificationDestinationBodyKeepsCredentialSeparate(t *testing.T) {
	credential := "write-only"
	body, err := notificationDestinationBody(context.Background(), exNotificationDestinationModel{
		Name: types.StringValue("primary"), DestinationType: types.StringValue("pagerduty"), Environment: types.StringValue("prod"), Region: types.StringValue("eu"), Enabled: types.BoolValue(true),
		Opsgenie: types.ObjectNull(notificationDestinationOpsgenieType()), ServiceNow: types.ObjectNull(notificationDestinationServiceNowType()),
	}, &credential, false)
	if err != nil {
		t.Fatal(err)
	}
	if body["credential"] != credential || body["provider"] != "pagerduty" {
		t.Fatalf("body = %#v", body)
	}
	if _, present := body["credential_wo"]; present {
		t.Fatalf("write-only Terraform field leaked to API body: %#v", body)
	}
}

func TestNotificationDestinationResponsePreservesWriteOnlyState(t *testing.T) {
	old := exNotificationDestinationModel{CredentialWO: types.StringNull(), CredentialWOVersion: types.Int64Value(2), Opsgenie: types.ObjectNull(notificationDestinationOpsgenieType()), ServiceNow: types.ObjectNull(notificationDestinationServiceNowType())}
	next, err := notificationDestinationFromResponse(context.Background(), old, map[string]any{"id": 7, "name": "primary", "provider": "pagerduty", "environment": "prod", "region": "us", "credential_configured": true, "enabled": true, "paused": false, "revision": 3})
	if err != nil {
		t.Fatal(err)
	}
	if next.CredentialWOVersion.ValueInt64() != 2 || !next.CredentialConfigured.ValueBool() || next.ID.ValueInt64() != 7 {
		t.Fatalf("state = %#v", next)
	}
}

func TestNotificationDestinationTypeChangeRequiresFreshWriteOnlyCredential(t *testing.T) {
	state := exNotificationDestinationModel{DestinationType: types.StringValue("pagerduty"), CredentialWOVersion: types.Int64Value(1)}
	plan := exNotificationDestinationModel{DestinationType: types.StringValue("opsgenie")}
	if !notificationDestinationTypeChangeRequiresCredential(plan, state, exNotificationDestinationModel{}) {
		t.Fatal("destination type change without a fresh credential must fail before API mutation")
	}
	config := exNotificationDestinationModel{CredentialWO: types.StringValue("new-write-only-value"), CredentialWOVersion: types.Int64Value(2)}
	if notificationDestinationTypeChangeRequiresCredential(plan, state, config) {
		t.Fatal("destination type change with a fresh credential version must proceed")
	}
	config.CredentialWOVersion = types.Int64Value(1)
	if !notificationDestinationTypeChangeRequiresCredential(plan, state, config) {
		t.Fatal("destination type change with an unchanged credential version must fail before API mutation")
	}
}

func TestNotificationReadRuleUsesPagedListContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/notification-governance/rules" || r.URL.Query().Get("count") != "100" || r.URL.Query().Get("offset") != "0" {
			t.Fatalf("request = %s", r.URL.String())
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{{"id": 9, "destination_id": 4, "source_kinds": []string{"task"}, "lifecycle_actions": []string{"trigger"}, "minimum_severity": "warning", "enabled": true, "revision": 5}})
	}))
	defer server.Close()
	next, err := notificationReadRule(context.Background(), newEXTestClient(t, server.URL), false, exNotificationRuleModel{ID: types.Int64Value(9), ProjectID: types.Int64Null(), SourceKinds: notificationSet(t), LifecycleActions: notificationSet(t)})
	if err != nil {
		t.Fatal(err)
	}
	if next.DestinationID.ValueInt64() != 4 || next.Revision.ValueInt64() != 5 {
		t.Fatalf("state = %#v", next)
	}
}

func TestNotificationRuleBodyUsesNativeSetValues(t *testing.T) {
	body, err := notificationRuleBody(context.Background(), exNotificationRuleModel{DestinationID: types.Int64Value(3), SourceKinds: notificationSet(t, "task"), LifecycleActions: notificationSet(t, "trigger"), MinimumSeverity: types.StringValue("error"), Enabled: types.BoolValue(true), Revision: types.Int64Value(2)}, true)
	if err != nil {
		t.Fatal(err)
	}
	if body["revision"] != int64(2) || len(body["source_kinds"].([]any)) != 1 {
		t.Fatalf("body = %#v", body)
	}
}

var _ attr.Value

func TestAcc_EXNotificationGovernance(t *testing.T) {
	name := "acceptance-notification-" + acctest.RandString(8)
	config := `
resource "semaphore_ex_project" "test" { name = "` + name + `" }
resource "semaphore_ex_global_notification_destination" "global" {
  name = "` + name + `-global"
  destination_type = "pagerduty"
  environment = "production"
  region = "eu"
  credential_wo = "0123456789abcdef0123456789abcdef"
  credential_wo_version = 1
}
resource "semaphore_ex_global_notification_rule" "global" {
  destination_id = semaphore_ex_global_notification_destination.global.id
  source_kinds = ["task"]
  lifecycle_actions = ["trigger"]
  minimum_severity = "warning"
}
resource "semaphore_ex_project_notification_destination" "project" {
  project_id = semaphore_ex_project.test.id
  name = "` + name + `-project"
  destination_type = "pagerduty"
  environment = "production"
  region = "us"
  credential_wo = "0123456789abcdef0123456789abcdef"
  credential_wo_version = 1
}
resource "semaphore_ex_project_notification_rule" "project" {
  project_id = semaphore_ex_project.test.id
  destination_id = semaphore_ex_project_notification_destination.project.id
  source_kinds = ["workflow"]
  lifecycle_actions = ["trigger", "resolve"]
  minimum_severity = "error"
}
data "semaphore_ex_global_notification_destination" "global" { id = semaphore_ex_global_notification_destination.global.id }
data "semaphore_ex_global_notification_rule" "global" { id = semaphore_ex_global_notification_rule.global.id }
data "semaphore_ex_project_notification_destination" "project" {
  project_id = semaphore_ex_project.test.id
  id = semaphore_ex_project_notification_destination.project.id
}
data "semaphore_ex_project_notification_rule" "project" {
  project_id = semaphore_ex_project.test.id
  id = semaphore_ex_project_notification_rule.project.id
}
`
	updated := strings.Replace(config, `minimum_severity = "warning"`, `minimum_severity = "critical"`, 1)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttrSet("semaphore_ex_global_notification_destination.global", "id"), resource.TestCheckResourceAttr("data.semaphore_ex_global_notification_destination.global", "destination_type", "pagerduty"), resource.TestCheckResourceAttrSet("semaphore_ex_global_notification_rule.global", "revision"), resource.TestCheckResourceAttrSet("semaphore_ex_project_notification_destination.project", "id"), resource.TestCheckResourceAttrSet("data.semaphore_ex_project_notification_rule.project", "revision"))},
		{ResourceName: "semaphore_ex_global_notification_rule.global", ImportState: true, ImportStateVerify: true, ImportStateIdFunc: func(s *terraform.State) (string, error) {
			return "rule/" + s.RootModule().Resources["semaphore_ex_global_notification_rule.global"].Primary.ID, nil
		}},
		{Config: updated, Check: resource.TestCheckResourceAttr("semaphore_ex_global_notification_rule.global", "minimum_severity", "critical")},
	}})
}
