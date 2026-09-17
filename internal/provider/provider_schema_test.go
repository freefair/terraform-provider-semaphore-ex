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
}
