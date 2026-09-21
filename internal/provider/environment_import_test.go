package provider

import (
	"context"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCoverageEnvironmentStringJSONImport(t *testing.T) {
	model, err := convertEnvironmentResponseToProjectEnvironmentModel(context.Background(), &models.Environment{JSON: `{"name":"value"}`, Env: `{"LABEL":"text"}`}, &ProjectEnvironmentModel{})
	require.NoError(t, err)
	require.JSONEq(t, `{"name":"value"}`, model.VariablesJSON.ValueString(), "import must work for JSON inputs even when every value is a string")
	require.JSONEq(t, `{"LABEL":"text"}`, model.EnvironmentJSON.ValueString())
}
