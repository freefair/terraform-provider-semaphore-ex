package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
	actionschema "github.com/hashicorp/terraform-plugin-framework/action/schema"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ldapTestModel(state string, passwordVersion int64) ldapConfigurationModel {
	selected, diagnostics := types.ListValueFrom(context.Background(), types.Int64Type, []int64{7})
	if diagnostics.HasError() {
		panic(diagnostics)
	}
	return ldapConfigurationModel{
		ID: types.StringValue("corporate"), DisplayName: types.StringValue("Corporate"), State: types.StringValue(state),
		ServerURL: types.StringValue("ldaps://directory.example.test:636"), TLSMode: types.StringValue("ldaps"), TrustMode: types.StringValue("custom_ca"),
		CAPEM: types.StringValue("-----BEGIN CERTIFICATE-----\nunit-test\n-----END CERTIFICATE-----\n"), BindDN: types.StringValue("cn=service,dc=example,dc=test"),
		BindPassword: types.StringNull(), BindPasswordWOVersion: types.Int64Value(passwordVersion), SearchBaseDN: types.StringValue("dc=example,dc=test"),
		UserFilter: types.StringValue("(uid={{username}})"), IdentityAttribute: types.StringValue("entryUUID"), UsernameAttribute: types.StringValue("uid"),
		NameAttribute: types.StringValue("cn"), EmailAttribute: types.StringValue("mail"), GroupSearchBaseDN: types.StringValue("ou=groups,dc=example,dc=test"),
		GroupUserFilter: types.StringValue("(objectClass=person)"), GroupFilter: types.StringValue("(objectClass=groupOfNames)"),
		GroupIdentityAttribute: types.StringValue("entryUUID"), GroupMemberAttribute: types.StringValue("member"), GroupMaxDepth: types.Int64Value(4), SelectedUserIDs: selected,
	}
}

func ldapTestConfig(t *testing.T, ctx context.Context, schema resourceschema.Schema, model ldapConfigurationModel, password string) tfsdk.Config {
	t.Helper()
	model.BindPassword = types.StringValue(password)
	state := tfsdk.State{Schema: schema}
	require.False(t, state.Set(ctx, &model).HasError())
	return tfsdk.Config{Schema: schema, Raw: state.Raw}
}

func ldapTestResponse(model ldapConfigurationModel) map[string]any {
	return map[string]any{
		"id": model.ID.ValueString(), "display_name": model.DisplayName.ValueString(), "state": model.State.ValueString(), "server_url": model.ServerURL.ValueString(),
		"tls_mode": model.TLSMode.ValueString(), "trust_mode": model.TrustMode.ValueString(), "ca_pem": "-----BEGIN CERTIFICATE-----\nunit-test\n-----END CERTIFICATE-----",
		"bind_dn": strings.TrimSpace(model.BindDN.ValueString()), "bind_password_configured": true, "search_base_dn": strings.TrimSpace(model.SearchBaseDN.ValueString()), "user_filter": strings.TrimSpace(model.UserFilter.ValueString()),
		"identity_attribute": strings.TrimSpace(model.IdentityAttribute.ValueString()), "username_attribute": strings.TrimSpace(model.UsernameAttribute.ValueString()), "name_attribute": strings.TrimSpace(model.NameAttribute.ValueString()), "email_attribute": strings.TrimSpace(model.EmailAttribute.ValueString()),
		"group_search_base_dn": strings.TrimSpace(model.GroupSearchBaseDN.ValueString()), "group_user_filter": strings.TrimSpace(model.GroupUserFilter.ValueString()), "group_filter": strings.TrimSpace(model.GroupFilter.ValueString()),
		"group_identity_attribute": strings.TrimSpace(model.GroupIdentityAttribute.ValueString()), "group_member_attribute": strings.TrimSpace(model.GroupMemberAttribute.ValueString()), "group_max_depth": 4, "selected_user_ids": []int64{7},
	}
}

func TestLDAPUpdateStateOnlyDoesNotResaveConfiguredBindPassword(t *testing.T) {
	ctx, schema := context.Background(), ldapResourceSchema()
	prior, plan := ldapTestModel("shadow", 1), ldapTestModel("active", 1)
	prior.UserFilter, plan.UserFilter = types.StringValue("(uid={{username}})\n"), types.StringValue("(uid={{username}})\n")
	plan.SelectedUserIDs = types.ListUnknown(types.Int64Type)
	configModel := plan
	configModel.SelectedUserIDs = types.ListNull(types.Int64Type)
	priorState, planState := tfsdk.State{Schema: schema}, tfsdk.Plan{Schema: schema}
	require.False(t, priorState.Set(ctx, &prior).HasError())
	require.False(t, planState.Set(ctx, &plan).HasError())
	requests := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.Path)
		switch request.URL.Path {
		case "/api/capabilities/ldap/state":
			require.Equal(t, http.MethodPut, request.Method)
			var body map[string]any
			require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
			assert.Equal(t, "active", body["state"])
		case "/api/capabilities/ldap":
			require.Equal(t, http.MethodGet, request.Method)
			_ = json.NewEncoder(w).Encode([]map[string]any{ldapTestResponse(plan)})
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	defer server.Close()
	response := frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schema}}
	(&ldapConfigurationResource{client: newEXTestClient(t, server.URL)}).Update(ctx, frameworkresource.UpdateRequest{Plan: planState, Config: ldapTestConfig(t, ctx, schema, configModel, "write-only-password"), State: priorState}, &response)
	require.False(t, response.Diagnostics.HasError(), "%v", response.Diagnostics)
	assert.Equal(t, []string{"PUT /api/capabilities/ldap/state", "GET /api/capabilities/ldap"}, requests)
	var next ldapConfigurationModel
	require.False(t, response.State.Get(ctx, &next).HasError())
	assert.Equal(t, plan.CAPEM, next.CAPEM)
	assert.Equal(t, plan.UserFilter, next.UserFilter)
}

func TestLDAPCreateRejectsActiveInitialStateBeforeHTTP(t *testing.T) {
	ctx, schema := context.Background(), ldapResourceSchema()
	plan := ldapTestModel("active", 1)
	planState := tfsdk.Plan{Schema: schema}
	require.False(t, planState.Set(ctx, &plan).HasError())
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests++
	}))
	defer server.Close()
	response := frameworkresource.CreateResponse{State: tfsdk.State{Schema: schema}}
	(&ldapConfigurationResource{client: newEXTestClient(t, server.URL)}).Create(ctx, frameworkresource.CreateRequest{Plan: planState, Config: ldapTestConfig(t, ctx, schema, plan, "write-only-password")}, &response)
	require.True(t, response.Diagnostics.HasError())
	assert.Zero(t, requests)
}

func TestLDAPCreateRejectsSelectedUsersOutsideSelectedUsersStateBeforeHTTP(t *testing.T) {
	ctx, schema := context.Background(), ldapResourceSchema()
	plan := ldapTestModel("shadow", 1)
	planState := tfsdk.Plan{Schema: schema}
	require.False(t, planState.Set(ctx, &plan).HasError())
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests++
	}))
	defer server.Close()
	response := frameworkresource.CreateResponse{State: tfsdk.State{Schema: schema}}
	(&ldapConfigurationResource{client: newEXTestClient(t, server.URL)}).Create(ctx, frameworkresource.CreateRequest{Plan: planState, Config: ldapTestConfig(t, ctx, schema, plan, "write-only-password")}, &response)
	require.True(t, response.Diagnostics.HasError())
	assert.Zero(t, requests)
}

func TestLDAPCreateRetainsConfiguredStateWhenStateWriteFails(t *testing.T) {
	ctx, schema := context.Background(), ldapResourceSchema()
	plan := ldapTestModel("shadow", 1)
	plan.UserFilter = types.StringValue("(uid={{username}})\n")
	plan.SelectedUserIDs = types.ListUnknown(types.Int64Type)
	configModel := plan
	configModel.SelectedUserIDs = types.ListNull(types.Int64Type)
	planState := tfsdk.Plan{Schema: schema}
	require.False(t, planState.Set(ctx, &plan).HasError())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/capabilities/ldap":
			require.Equal(t, http.MethodPut, request.Method)
			_ = json.NewEncoder(w).Encode(ldapTestResponse(plan))
		case "/api/capabilities/ldap/state":
			require.Equal(t, http.MethodPut, request.Method)
			w.WriteHeader(http.StatusConflict)
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	defer server.Close()
	response := frameworkresource.CreateResponse{State: tfsdk.State{Schema: schema}}
	(&ldapConfigurationResource{client: newEXTestClient(t, server.URL)}).Create(ctx, frameworkresource.CreateRequest{Plan: planState, Config: ldapTestConfig(t, ctx, schema, configModel, "write-only-password")}, &response)
	require.True(t, response.Diagnostics.HasError())
	var retained ldapConfigurationModel
	require.False(t, response.State.Get(ctx, &retained).HasError())
	assert.Equal(t, "corporate", retained.ID.ValueString())
	assert.Equal(t, int64(1), retained.BindPasswordWOVersion.ValueInt64())
	assert.Equal(t, plan.CAPEM, retained.CAPEM)
	assert.Equal(t, plan.UserFilter, retained.UserFilter)
}

func TestLDAPUpdateChangedBindPasswordVersionResavesConfiguration(t *testing.T) {
	ctx, schema := context.Background(), ldapResourceSchema()
	prior, plan := ldapTestModel("disabled", 1), ldapTestModel("disabled", 2)
	plan.SelectedUserIDs = types.ListUnknown(types.Int64Type)
	configModel := plan
	configModel.SelectedUserIDs = types.ListNull(types.Int64Type)
	priorState, planState := tfsdk.State{Schema: schema}, tfsdk.Plan{Schema: schema}
	require.False(t, priorState.Set(ctx, &prior).HasError())
	require.False(t, planState.Set(ctx, &plan).HasError())
	var decodedPlan ldapConfigurationModel
	require.False(t, planState.Get(ctx, &decodedPlan).HasError())
	require.Equal(t, int64(2), decodedPlan.BindPasswordWOVersion.ValueInt64())
	requests := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.Path)
		if request.URL.Path == "/api/capabilities/ldap" && request.Method == http.MethodPut {
			var body map[string]any
			require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
			assert.Equal(t, "rotated-password", body["bind_password"])
			_ = json.NewEncoder(w).Encode(ldapTestResponse(plan))
			return
		}
		if request.URL.Path == "/api/capabilities/ldap/state" && request.Method == http.MethodPut {
			return
		}
		if request.URL.Path == "/api/capabilities/ldap" && request.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode([]map[string]any{ldapTestResponse(plan)})
			return
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
	}))
	defer server.Close()
	response := frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schema}}
	(&ldapConfigurationResource{client: newEXTestClient(t, server.URL)}).Update(ctx, frameworkresource.UpdateRequest{Plan: planState, Config: ldapTestConfig(t, ctx, schema, configModel, "rotated-password"), State: priorState}, &response)
	require.False(t, response.Diagnostics.HasError(), "%v", response.Diagnostics)
	assert.Equal(t, []string{"PUT /api/capabilities/ldap", "PUT /api/capabilities/ldap/state", "GET /api/capabilities/ldap"}, requests)
}

func TestLDAPMergeUnknownOptionalComputedPreservesPriorValues(t *testing.T) {
	prior, plan := ldapTestModel("disabled", 1), ldapTestModel("disabled", 1)
	plan.CAPEM, plan.GroupSearchBaseDN, plan.GroupUserFilter = types.StringUnknown(), types.StringUnknown(), types.StringUnknown()
	plan.GroupFilter, plan.GroupIdentityAttribute, plan.GroupMemberAttribute = types.StringUnknown(), types.StringUnknown(), types.StringUnknown()
	plan.GroupMaxDepth, plan.SelectedUserIDs = types.Int64Unknown(), types.ListUnknown(types.Int64Type)
	ldapMergeUnknownOptionalComputed(&plan, prior)
	assert.Equal(t, prior.CAPEM, plan.CAPEM)
	assert.Equal(t, prior.GroupSearchBaseDN, plan.GroupSearchBaseDN)
	assert.Equal(t, prior.GroupUserFilter, plan.GroupUserFilter)
	assert.Equal(t, prior.GroupFilter, plan.GroupFilter)
	assert.Equal(t, prior.GroupIdentityAttribute, plan.GroupIdentityAttribute)
	assert.Equal(t, prior.GroupMemberAttribute, plan.GroupMemberAttribute)
	assert.Equal(t, prior.GroupMaxDepth, plan.GroupMaxDepth)
	assert.Equal(t, prior.SelectedUserIDs, plan.SelectedUserIDs)
}

func TestLDAPTestActionUsesWriteOnlyCredentialsAndRequiresReadyResponse(t *testing.T) {
	actionValue := NewLDAPTestAction().(*ldapTestAction)
	schemaResponse := action.SchemaResponse{}
	actionValue.Schema(context.Background(), action.SchemaRequest{}, &schemaResponse)
	for _, name := range []string{"provider_id", "username", "password", "recovery_admin_password"} {
		attribute, ok := schemaResponse.Schema.Attributes[name].(actionschema.StringAttribute)
		require.True(t, ok)
		assert.True(t, attribute.WriteOnly, "%s must be write-only", name)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		require.Equal(t, http.MethodPost, request.Method)
		require.Equal(t, "/api/capabilities/ldap/test", request.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
		assert.Equal(t, map[string]any{"provider_id": "corporate", "username": "probe", "password": "directory-password", "recovery_admin_user_id": float64(7), "recovery_admin_password": "recovery-password"}, body)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "failed", "connection": true, "search": true, "bind": true, "recovery": false})
	}))
	defer server.Close()
	actionValue.client = newEXTestClient(t, server.URL)
	var response action.InvokeResponse
	actionValue.Invoke(context.Background(), action.InvokeRequest{Config: runtimeActionConfig(t, actionValue, map[string]tftypes.Value{
		"provider_id": tftypes.NewValue(tftypes.String, "corporate"), "username": tftypes.NewValue(tftypes.String, "probe"), "password": tftypes.NewValue(tftypes.String, "directory-password"), "recovery_admin_user_id": tftypes.NewValue(tftypes.Number, int64(7)), "recovery_admin_password": tftypes.NewValue(tftypes.String, "recovery-password"),
	})}, &response)
	require.True(t, response.Diagnostics.HasError())
	assert.NotContains(t, response.Diagnostics.Errors()[0].Detail(), "directory-password")
	assert.NotContains(t, response.Diagnostics.Errors()[0].Detail(), "recovery-password")
}
