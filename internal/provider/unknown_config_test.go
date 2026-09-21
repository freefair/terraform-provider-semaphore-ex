package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Build values from the actual registered schema so protocol validation exercises
// both schema validators and ResourceWithValidateConfig.
func unknownConfigObject(typ tftypes.Type, overrides map[string]any) tftypes.Value {
	object := typ.(tftypes.Object)
	values := make(map[string]tftypes.Value, len(object.AttributeTypes))
	for name, childType := range object.AttributeTypes {
		value := overrides[name]
		switch nested := value.(type) {
		case map[string]any:
			values[name] = unknownConfigObject(childType, nested)
		case tftypes.Value:
			values[name] = nested
		default:
			values[name] = tftypes.NewValue(childType, value)
		}
	}
	return tftypes.NewValue(typ, values)
}

func TestUnknownConfigValidation(t *testing.T) {
	ctx := context.Background()
	server, err := providerserver.NewProtocol6WithError(New("test")())()
	require.NoError(t, err)
	schemas, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	require.NoError(t, err)

	for _, name := range []string{"variables", "environment"} {
		for _, value := range []any{
			tftypes.UnknownValue,
			map[string]tftypes.Value{"pending": tftypes.NewValue(tftypes.String, tftypes.UnknownValue)},
			map[string]tftypes.Value{},
			nil,
		} {
			t.Run("environment/"+name+"/"+tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, value).String(), func(t *testing.T) {
				validateUnknownConfig(t, server, schemas.ResourceSchemas["semaphore_ex_project_environment"].ValueType(),
					"semaphore_ex_project_environment", map[string]any{name: value}, "")
			})
		}
	}
	for _, name := range []string{"login_password", "ssh", "string", "none", "remote_reference"} {
		t.Run("key/"+name, func(t *testing.T) {
			values := map[string]any{name: tftypes.UnknownValue}
			if name == "remote_reference" {
				values["login_password"] = map[string]any{"login": "user"}
			}
			validateUnknownConfig(t, server, schemas.ResourceSchemas["semaphore_ex_project_key"].ValueType(),
				"semaphore_ex_project_key", values, "")
		})
	}
	for _, name := range []string{"path", "storage_id", "field", "storage_type"} {
		t.Run("remote/"+name, func(t *testing.T) {
			ref := map[string]any{"storage_type": "vault", "storage_id": 12, "path": "service/token", "field": "token"}
			ref[name] = tftypes.UnknownValue
			validateUnknownConfig(t, server, schemas.ResourceSchemas["semaphore_ex_project_key"].ValueType(),
				"semaphore_ex_project_key", map[string]any{
					"login_password": map[string]any{"login": "user"}, "remote_reference": ref,
				}, "")
		})
	}
	t.Run("secret_storage", func(t *testing.T) {
		validateUnknownConfig(t, server, schemas.ResourceSchemas["semaphore_ex_project_environment"].ValueType(),
			"semaphore_ex_project_environment", map[string]any{"secret_storage": tftypes.UnknownValue}, "")
	})
	for _, listName := range []string{"secrets", "sync_paths"} {
		typ := schemas.ResourceSchemas["semaphore_ex_project_environment"].ValueType()
		listType := typ.(tftypes.Object).AttributeTypes[listName].(tftypes.List)
		t.Run(listName+"/unknown_list", func(t *testing.T) {
			validateUnknownConfig(t, server, typ, "semaphore_ex_project_environment",
				map[string]any{listName: tftypes.UnknownValue}, "")
		})
		t.Run(listName+"/unknown_element", func(t *testing.T) {
			// Framework required-child validation rejects a wholly unknown
			// secrets element before resource validation. Exercise our boundary
			// directly so that this test isolates the provider's decoding.
			schema := ProjectEnvironmentSchema().GetResource(ctx)
			config := tfsdk.Config{Schema: schema, Raw: unknownConfigObject(typ,
				map[string]any{listName: []tftypes.Value{tftypes.NewValue(listType.ElementType, tftypes.UnknownValue)}})}
			resp := resource.ValidateConfigResponse{}
			(&projectEnvironmentResource{}).ValidateConfig(ctx, resource.ValidateConfigRequest{Config: config}, &resp)
			assert.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
		})
	}
	environmentType := schemas.ResourceSchemas["semaphore_ex_project_environment"].ValueType()
	secretType := environmentType.(tftypes.Object).AttributeTypes["secrets"].(tftypes.List).ElementType
	for _, tc := range []struct {
		name      string
		secret    map[string]any
		wantError string
	}{
		{"unknown_plaintext", map[string]any{"value": tftypes.UnknownValue}, ""},
		{"unknown_storage", map[string]any{"storage_id": tftypes.UnknownValue, "path": "service/token", "field": "token"}, ""},
		{"unknown_path", map[string]any{"storage_id": 12, "path": tftypes.UnknownValue, "field": "token"}, ""},
		{"unknown_field", map[string]any{"storage_id": 12, "path": "service/token", "field": tftypes.UnknownValue}, ""},
		{"unknown_mount", map[string]any{"mount": tftypes.UnknownValue}, ""},
		{"unknown_version", map[string]any{"version": tftypes.UnknownValue}, ""},
		{"known_missing_source", map[string]any{}, "Missing secret source"},
		{"known_conflict", map[string]any{"value": "test", "storage_id": 12, "path": "service/token", "field": "token"}, "Conflicting secret sources"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.secret["name"], tc.secret["type"] = "TOKEN", "env"
			validateUnknownConfig(t, server, environmentType, "semaphore_ex_project_environment",
				map[string]any{"variables": tftypes.UnknownValue, "secrets": []tftypes.Value{unknownConfigObject(secretType, tc.secret)}}, tc.wantError)
		})
	}
	t.Run("unknown_key_does_not_hide_invalid_reference", func(t *testing.T) {
		validateUnknownConfig(t, server, schemas.ResourceSchemas["semaphore_ex_project_key"].ValueType(),
			"semaphore_ex_project_key", map[string]any{
				"login_password":   tftypes.UnknownValue,
				"remote_reference": map[string]any{"storage_type": "vault", "path": "service/token", "field": "token"},
			}, "Incomplete remote secret reference")
	})
	for _, tc := range []struct {
		name      string
		key       map[string]any
		ref       map[string]any
		wantError string
	}{
		{"known_literal", map[string]any{"login": "user", "password": "test"}, nil, ""},
		{"unknown_password", map[string]any{"login": "user", "password": tftypes.UnknownValue}, nil, ""},
		{"unknown_write_only", map[string]any{"login": "user", "password_wo": tftypes.UnknownValue, "password_wo_version": 1}, nil, ""},
		{"known_remote", map[string]any{"login": "user"}, map[string]any{"storage_type": "vault", "storage_id": 12, "path": "service/token", "field": "token"}, ""},
		{"known_key_conflict", map[string]any{"password": "test"}, map[string]any{"storage_type": "env", "path": "TOKEN"}, "Conflicting secret sources"},
		{"missing_vault_field", map[string]any{}, map[string]any{"storage_type": "vault", "storage_id": 12, "path": "service/token"}, "Incomplete remote secret reference"},
		{"non_vault_storage", map[string]any{}, map[string]any{"storage_type": "env", "storage_id": 12, "path": "TOKEN"}, "Invalid remote secret reference"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := map[string]any{"login_password": tc.key}
			if tc.ref != nil {
				values["remote_reference"] = tc.ref
			}
			validateUnknownConfig(t, server, schemas.ResourceSchemas["semaphore_ex_project_key"].ValueType(),
				"semaphore_ex_project_key", values, tc.wantError)
		})
	}
}

func validateUnknownConfig(t *testing.T, server tfprotov6.ProviderServer, typ tftypes.Type, name string, values map[string]any, wantError string) {
	t.Helper()
	values["name"], values["project_id"] = "unknown-regression", 1
	config, err := tfprotov6.NewDynamicValue(typ, unknownConfigObject(typ, values))
	require.NoError(t, err)
	response, err := server.ValidateResourceConfig(context.Background(), &tfprotov6.ValidateResourceConfigRequest{
		TypeName: name, Config: &config, ClientCapabilities: &tfprotov6.ValidateResourceConfigClientCapabilities{WriteOnlyAttributesAllowed: true},
	})
	require.NoError(t, err)
	errors := []string{}
	for _, diagnostic := range response.Diagnostics {
		if diagnostic.Severity == tfprotov6.DiagnosticSeverityError {
			errors = append(errors, diagnostic.Summary)
			assert.NotContains(t, diagnostic.Detail, "target type cannot handle unknown", diagnostic.Detail)
		}
	}
	if wantError == "" {
		assert.Empty(t, errors, "%v", response.Diagnostics)
	} else {
		assert.Contains(t, errors, wantError)
	}
}

func TestUnknownEnvironmentModifyPlan(t *testing.T) {
	ctx := context.Background()
	schema := ProjectEnvironmentSchema().GetResource(ctx)
	typ := schema.Type().TerraformType(ctx)
	listType := typ.(tftypes.Object).AttributeTypes["sync_paths"].(tftypes.List)
	for _, tc := range []struct {
		name      string
		storage   any
		syncPaths any
		wantError bool
	}{
		{"unknown_maps", nil, nil, false},
		{"unknown_storage", tftypes.UnknownValue, []tftypes.Value{tftypes.NewValue(listType.ElementType, tftypes.UnknownValue)}, false},
		{"unknown_storage_id", map[string]any{"id": tftypes.UnknownValue}, []tftypes.Value{tftypes.NewValue(listType.ElementType, tftypes.UnknownValue)}, false},
		{"known_storage_id", map[string]any{"id": 12}, []tftypes.Value{tftypes.NewValue(listType.ElementType, tftypes.UnknownValue)}, false},
		{"missing_storage", nil, []tftypes.Value{tftypes.NewValue(listType.ElementType, tftypes.UnknownValue)}, true},
		{"empty_storage", map[string]any{}, []tftypes.Value{tftypes.NewValue(listType.ElementType, tftypes.UnknownValue)}, true},
		{"unknown_paths", nil, tftypes.UnknownValue, false},
		{"empty_paths", nil, []tftypes.Value{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := unknownConfigObject(typ, map[string]any{
				"name": "unknown-regression", "project_id": 1,
				"variables": tftypes.UnknownValue, "environment": tftypes.UnknownValue,
				"secret_storage": tc.storage, "sync_paths": tc.syncPaths,
			})
			plan := tfsdk.Plan{Schema: schema, Raw: raw}
			resp := resource.ModifyPlanResponse{Plan: plan}
			(&projectEnvironmentResource{}).ModifyPlan(ctx, resource.ModifyPlanRequest{Plan: plan}, &resp)
			assert.Equal(t, tc.wantError, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
			assert.True(t, resp.Plan.Raw.Equal(raw), "planning must preserve unknown values")
			for _, diagnostic := range resp.Diagnostics {
				assert.NotContains(t, diagnostic.Detail(), "target type cannot handle unknown")
			}
		})
	}
	t.Run("destroy", func(t *testing.T) {
		plan := tfsdk.Plan{Schema: schema, Raw: tftypes.NewValue(typ, nil)}
		resp := resource.ModifyPlanResponse{Plan: plan}
		(&projectEnvironmentResource{}).ModifyPlan(ctx, resource.ModifyPlanRequest{Plan: plan}, &resp)
		assert.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
		assert.True(t, resp.Plan.Raw.IsNull())
	})
}
