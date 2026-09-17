package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	schemaR "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func identityTestTarget(t *testing.T) types.Object {
	t.Helper()
	v, d := types.ObjectValue(identityTargetTypes, map[string]attr.Value{
		"scope": types.StringValue("global"), "project_id": types.Int64Null(), "role_id": types.StringValue("role-a"),
	})
	if d.HasError() {
		t.Fatal(d)
	}
	return v
}

func TestEXIdentityMappingResourceSchemaRejectsIrrelevantMappingValues(t *testing.T) {
	tests := []struct {
		name, relevant, irrelevant string
		oidc                       bool
	}{
		{name: "LDAP", relevant: "group_external_id", irrelevant: "claim_value"},
		{name: "OIDC", relevant: "claim_value", irrelevant: "group_external_id", oidc: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			attributes := identityMappingResourceSchema("test", test.oidc).Attributes
			relevant, ok := attributes[test.relevant].(schemaR.StringAttribute)
			require.True(t, ok, "%s must be a string attribute", test.relevant)
			assert.True(t, relevant.Required)
			assert.False(t, relevant.Computed)
			assert.False(t, relevant.Optional)
			irrelevant, ok := attributes[test.irrelevant].(schemaR.StringAttribute)
			require.True(t, ok, "%s must be a string attribute", test.irrelevant)
			assert.True(t, irrelevant.Computed)
			assert.False(t, irrelevant.Optional)
			assert.False(t, irrelevant.Required)
		})
	}
}

func TestOIDCMappingWritePreservesCaseOnlyForCaseInsensitiveProvider(t *testing.T) {
	for _, caseInsensitive := range []bool{true, false} {
		t.Run(map[bool]string{true: "folding", false: "strict"}[caseInsensitive], func(t *testing.T) {
			metadataRead := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/capabilities/oidc/group-mapping/providers":
					metadataRead = true
					_, _ = w.Write([]byte(`[{"id":"oidc","claim_configuration":{"case_insensitive":` + map[bool]string{true: "true", false: "false"}[caseInsensitive] + `}}]`))
				case "/api/capabilities/oidc/group-mappings/group":
					require.True(t, metadataRead)
					_, _ = w.Write([]byte(`{"id":"group","provider_id":"oidc","claim_value":"ops","target":{"scope":"global","role_id":"role-a"},"enabled":true,"revision":1}`))
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()
			model := identityMappingModel{ID: types.StringValue("group"), ProviderID: types.StringValue("oidc"), ClaimValue: types.StringValue("Ops"), GroupExternalID: types.StringNull(), Target: identityTestTarget(t), Enabled: types.BoolValue(true), Revision: types.Int64Null()}
			next, err := writeIdentityMapping(context.Background(), newEXTestClient(t, server.URL), true, model)
			require.NoError(t, err)
			if caseInsensitive {
				require.Equal(t, "Ops", next.ClaimValue.ValueString())
			} else {
				require.Equal(t, "ops", next.ClaimValue.ValueString())
			}
		})
	}
}

func TestTOTPPolicyRejectsIrrelevantAndDuplicateSelectedUsersBeforeHTTP(t *testing.T) {
	for _, test := range []struct {
		name, state string
		ids         []int64
	}{{"irrelevant", "optional", []int64{1}}, {"duplicate", "required_selected", []int64{1, 1}}} {
		t.Run(test.name, func(t *testing.T) {
			ids, diagnostics := types.ListValueFrom(context.Background(), types.Int64Type, test.ids)
			require.False(t, diagnostics.HasError())
			_, err := writeTOTP(context.Background(), nil, totpPolicyModel{State: types.StringValue(test.state), SelectedUserIDs: ids})
			require.Error(t, err)
		})
	}
}

func TestOIDCMappingReadReportsCaseOnlyDriftForStrictProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/capabilities/oidc/group-mappings":
			_, _ = w.Write([]byte(`[{"id":"group","provider_id":"oidc","claim_value":"ops","target":{"scope":"global","role_id":"role-a"},"enabled":true,"revision":1}]`))
		case "/api/capabilities/oidc/group-mapping/providers":
			_, _ = w.Write([]byte(`[{"id":"oidc","claim_configuration":{"case_insensitive":false}}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	state := identityMappingModel{ID: types.StringValue("group"), ProviderID: types.StringValue("oidc"), ClaimValue: types.StringValue("Ops")}
	next, err := readIdentityMapping(context.Background(), newEXTestClient(t, server.URL), true, state)
	require.NoError(t, err)
	require.Equal(t, "ops", next.ClaimValue.ValueString())
}

func TestOIDCMappingSchemaRejectsTrimmedClaimViolation(t *testing.T) {
	attribute := identityMappingResourceSchema("test", true).Attributes["claim_value"].(schemaR.StringAttribute)
	require.NotEmpty(t, attribute.Validators)
}

func TestEXIdentityMappingWritesRevisionAndReadsRedactedValues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/capabilities/ldap/group-mappings/mapping-a" || r.Method != http.MethodPut {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) == "" || !strings.Contains(string(body), `"expected_revision":4`) {
			t.Fatalf("payload = %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"mapping-a","provider_id":"ldap-a","group_external_id":"entryuuid:12345678-1234-1234-1234-123456789abc","target":{"scope":"global","role_id":"role-a"},"enabled":true,"revision":5}`))
	}))
	defer server.Close()
	m := identityMappingModel{ID: types.StringValue("mapping-a"), ProviderID: types.StringValue("ldap-a"), GroupExternalID: types.StringValue("entryuuid:12345678-1234-1234-1234-123456789abc"), Target: identityTestTarget(t), Enabled: types.BoolValue(true), Revision: types.Int64Value(4)}
	next, err := writeIdentityMapping(context.Background(), exRoleTestClient(t, server.URL), false, m)
	if err != nil {
		t.Fatal(err)
	}
	if next.Revision.ValueInt64() != 5 || next.Target.IsNull() {
		t.Fatalf("state = %#v", next)
	}
}

func TestEXTOTPReadUsesGlobalPolicyIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/capabilities/totp" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"state":"required_selected","selected_user_ids":[2,7]}`))
	}))
	defer server.Close()
	m, err := readTOTP(context.Background(), exRoleTestClient(t, server.URL))
	if err != nil {
		t.Fatal(err)
	}
	if m.ID.ValueString() != "totp" || m.State.ValueString() != "required_selected" {
		t.Fatalf("state = %#v", m)
	}
}

func TestEXLDAPSingletonReadAndSafeDisableContract(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		switch requests {
		case 1:
			if r.Method != http.MethodGet || r.URL.Path != "/api/capabilities/ldap" {
				t.Fatalf("read = %s %s", r.Method, r.URL.Path)
			}
			_, _ = w.Write([]byte(`[{"id":"corporate","display_name":"Corporate","state":"active","server_url":"ldaps://directory.example.test","tls_mode":"ldaps","trust_mode":"system","bind_dn":"cn=service","bind_password_configured":true,"search_base_dn":"dc=example","user_filter":"(uid=%s)","identity_attribute":"entryUUID","username_attribute":"uid","name_attribute":"cn","email_attribute":"mail","selected_user_ids":[9]}]`))
		case 2:
			if r.Method != http.MethodPut || r.URL.Path != "/api/capabilities/ldap/state" {
				t.Fatalf("delete = %s %s", r.Method, r.URL.Path)
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), `"state":"disabled"`) || !strings.Contains(string(body), `"selected_user_ids":[]`) {
				t.Fatalf("disable payload = %s", body)
			}
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()
	client := exRoleTestClient(t, server.URL)
	m, err := readLDAP(context.Background(), client, "corporate")
	if err != nil {
		t.Fatal(err)
	}
	if !m.BindPasswordConfigured.ValueBool() || !m.BindPassword.IsNull() {
		t.Fatalf("credential state = %#v", m)
	}
	err = exRequest(context.Background(), client, http.MethodPut, "/capabilities/ldap/state", nil, map[string]any{"provider_id": m.ID.ValueString(), "state": "disabled", "selected_user_ids": []int64{}}, nil)
	if err != nil {
		t.Fatal(err)
	}
}
