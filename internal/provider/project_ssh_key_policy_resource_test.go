package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

// This regression test defines the public singleton resource contract before
// its implementation exists. It must fail to compile against the baseline.
func TestProjectSSHKeyPolicyResourceHasTypedBindings(t *testing.T) {
	instance := NewProjectSSHKeyPolicyResource()

	var response resource.SchemaResponse
	instance.Schema(context.Background(), resource.SchemaRequest{}, &response)

	assert.Contains(t, response.Schema.Attributes, "project_id")
	assert.Contains(t, response.Schema.Attributes, "default_ssh_keys")
	assert.Contains(t, response.Schema.Attributes, "always_ssh_keys")
}

func TestSSHKeyBindingsPreserveNullAndExplicitEmpty(t *testing.T) {
	ctx := context.Background()
	nilBindings, err := sshKeyBindingsFromAPI(nil)
	assert.NoError(t, err)
	assert.True(t, nilBindings.IsNull())

	emptyBindings, err := sshKeyBindingsFromAPI([]any{})
	assert.NoError(t, err)
	assert.False(t, emptyBindings.IsNull())
	assert.Empty(t, emptyBindings.Elements())

	wire, err := sshKeyBindingsToAPI(ctx, emptyBindings)
	assert.NoError(t, err)
	assert.Equal(t, []any{}, wire)

	binding, diagnostics := types.ObjectValue(sshKeyBindingType.AttrTypes, map[string]attr.Value{
		"access_key_id": types.Int64Value(7),
		"hosts":         types.ListNull(types.StringType),
	})
	assert.False(t, diagnostics.HasError())
	configured, diagnostics := types.ListValue(sshKeyBindingType, []attr.Value{binding})
	assert.False(t, diagnostics.HasError())
	wire, err = sshKeyBindingsToAPI(ctx, configured)
	assert.NoError(t, err)
	assert.Equal(t, []any{map[string]any{"access_key_id": int64(7), "hosts": nil}}, wire)
}

func TestTemplateSSHKeyInheritanceUsesNullBindings(t *testing.T) {
	selection, err := sshKeyPolicySelectionFromAPI(nil)
	assert.NoError(t, err)
	bindings, ok := selection.Attributes()["bindings"].(types.List)
	assert.True(t, ok)
	assert.True(t, bindings.IsNull())

	emptySelection, err := sshKeyPolicySelectionFromAPI([]any{})
	assert.NoError(t, err)
	emptyBindings, ok := emptySelection.Attributes()["bindings"].(types.List)
	assert.True(t, ok)
	assert.False(t, emptyBindings.IsNull())
	assert.Empty(t, emptyBindings.Elements())
}
