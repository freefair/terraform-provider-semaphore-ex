package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateJWTFromAPI_NormalizesAudienceResponse(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     []string
	}{
		{
			name:     "single audience string",
			response: `{"enabled":true,"audience":"service","ttl":"10m"}`,
			want:     []string{"service"},
		},
		{
			name:     "multiple audience array",
			response: `{"enabled":true,"audience":["service","worker"],"ttl":"10m"}`,
			want:     []string{"service", "worker"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var response models.TemplateJWTParams
			require.NoError(t, json.Unmarshal([]byte(tt.response), &response))

			state := templateJWTFromAPI(context.Background(), &response)
			var audience []string
			diagnostics := state.Audience.ElementsAs(context.Background(), &audience, false)
			require.False(t, diagnostics.HasError(), diagnostics.Errors())
			assert.Equal(t, tt.want, audience)
			assert.Equal(t, types.BoolValue(true), state.Enabled)
			assert.Equal(t, types.StringValue("10m"), state.TTL)
		})
	}
}

func TestTemplateJWTToAPI_AlwaysSendsAudienceArray(t *testing.T) {
	value := &ProjectTemplateJWTModel{
		Enabled:  types.BoolValue(true),
		Audience: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("service")}),
		TTL:      types.StringValue("10m"),
	}

	params := templateJWTToAPI(context.Background(), value)
	require.NotNil(t, params)
	assert.Equal(t, []string{"service"}, params.Audience)
}
