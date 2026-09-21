package provider

import (
	"context"
	providerfw "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestProviderEndpointIsKnownAndAbsolute(t *testing.T) {
	for _, value := range []any{tftypes.UnknownValue, "localhost:3000/api", "/api", "ftp://example.test/api", "http:///api"} {
		t.Run("endpoint", func(t *testing.T) {
			ctx := context.Background()
			p := New("test")()
			var schema providerfw.SchemaResponse
			p.Schema(ctx, providerfw.SchemaRequest{}, &schema)
			var response providerfw.ConfigureResponse
			p.Configure(ctx, providerfw.ConfigureRequest{Config: tfsdk.Config{Schema: schema.Schema, Raw: unknownConfigObject(schema.Schema.Type().TerraformType(ctx), map[string]any{"api_token": exTestToken, "api_base_url": value, "tls_skip_verify": false})}}, &response)
			require.True(t, response.Diagnostics.HasError())
			assert.Nil(t, response.ResourceData)
			if value == tftypes.UnknownValue {
				assert.Contains(t, response.Diagnostics[0].Summary(), "Unknown")
			}
		})
	}
}
