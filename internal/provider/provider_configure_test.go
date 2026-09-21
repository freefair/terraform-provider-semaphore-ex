package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderConfigureReportsUnknownTokenOnce(t *testing.T) {
	for _, unknownTLS := range []bool{false, true} {
		t.Run(map[bool]string{false: "token", true: "token_and_tls"}[unknownTLS], func(t *testing.T) {
			ctx := context.Background()
			server, err := providerserver.NewProtocol6WithError(New("test")())()
			require.NoError(t, err)
			schema, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
			require.NoError(t, err)
			values := map[string]any{"api_token": tftypes.UnknownValue, "api_base_url": "https://example.invalid/api", "tls_skip_verify": false}
			if unknownTLS {
				values["tls_skip_verify"] = tftypes.UnknownValue
			}
			config, err := tfprotov6.NewDynamicValue(schema.Provider.ValueType(), unknownConfigObject(schema.Provider.ValueType(), values))
			require.NoError(t, err)
			response, err := server.ConfigureProvider(ctx, &tfprotov6.ConfigureProviderRequest{Config: &config})
			require.NoError(t, err)
			counts := map[string]int{}
			for _, diagnostic := range response.Diagnostics {
				assert.Equal(t, tfprotov6.DiagnosticSeverityError, diagnostic.Severity)
				counts[diagnostic.Summary]++
			}
			expected := map[string]int{"Unknown SemaphoreUI API Token": 1}
			if unknownTLS {
				expected["Unknown SemaphoreUI TLS Skip Verify"] = 1
			}
			assert.Equal(t, expected, counts)
		})
	}
}
