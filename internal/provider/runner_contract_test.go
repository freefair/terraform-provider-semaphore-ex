package provider

import (
	"context"
	"testing"

	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunnerRequestPreservesImportedRegistrationPolicy(t *testing.T) {
	request, diags := convertRunnerModelToRunnerRequest(context.Background(), RunnerModel{
		Name:               types.StringValue("secure runner"),
		RegistrationPolicy: types.StringValue("secure"),
	}, "standard")
	require.False(t, diags.HasError())
	assert.Equal(t, "secure", request.RegistrationPolicy)
}

func TestRunnerRequestDefaultsPolicyOnlyWhenUnknown(t *testing.T) {
	request, diags := convertRunnerModelToRunnerRequest(context.Background(), RunnerModel{
		Name:               types.StringValue("new runner"),
		RegistrationPolicy: types.StringUnknown(),
	}, "standard")
	require.False(t, diags.HasError())
	assert.Equal(t, "standard", request.RegistrationPolicy)
}

func TestRunnerResponseKeepsOnlyDurableConfiguration(t *testing.T) {
	model, diags := convertRunnerResponseToRunnerModel(context.Background(), &models.Runner{ID: 7, Name: "secure runner", RegistrationPolicy: "secure"})
	require.False(t, diags.HasError())
	assert.Equal(t, "secure", model.RegistrationPolicy.ValueString())
}
