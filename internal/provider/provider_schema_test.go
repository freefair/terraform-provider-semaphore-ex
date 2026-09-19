package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderRegisteredSchemasAreValid(t *testing.T) {
	server, err := providerserver.NewProtocol6WithError(New("test")())()
	require.NoError(t, err)
	response, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	require.NoError(t, err)
	for _, diagnostic := range response.Diagnostics {
		assert.NotEqual(t, tfprotov6.DiagnosticSeverityError, diagnostic.Severity, "%s: %s", diagnostic.Summary, diagnostic.Detail)
	}
	assert.NotEmpty(t, response.ResourceSchemas)
	assert.NotEmpty(t, response.DataSourceSchemas)
	template, found := response.ResourceSchemas["semaphore_ex_project_template"]
	require.True(t, found)
	var sshKeys *tfprotov6.SchemaAttribute
	for _, attribute := range template.Block.Attributes {
		if attribute.Name == "ssh_keys" {
			sshKeys = attribute
			break
		}
	}
	require.NotNil(t, sshKeys)
	require.NotNil(t, sshKeys.NestedType)
	children := map[string]*tfprotov6.SchemaAttribute{}
	for _, child := range sshKeys.NestedType.Attributes {
		children[child.Name] = child
	}
	require.Contains(t, children, "inherit")
	require.Contains(t, children, "bindings")
	require.NotNil(t, children["bindings"].NestedType)
	bindings := map[string]*tfprotov6.SchemaAttribute{}
	for _, child := range children["bindings"].NestedType.Attributes {
		bindings[child.Name] = child
	}
	assert.Contains(t, bindings, "access_key_id")
	assert.Contains(t, bindings, "hosts")
}
