package provider

import (
	"context"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCoverageExternalUserReadNeverCreates(t *testing.T) {
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		writes++
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":1,"username":"missing","name":"missing","email":"missing","external":true}`))
	}))
	defer server.Close()
	ctx := context.Background()
	d := &externalUserDataSource{client: newEXTestClient(t, server.URL)}
	var schema datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schema)
	config := tfsdk.Config{Schema: schema.Schema, Raw: unknownConfigObject(schema.Schema.Type().TerraformType(ctx), map[string]any{"username": "missing"})}
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schema.Schema}}
	d.Read(ctx, datasource.ReadRequest{Config: config}, &resp)
	assert.True(t, resp.Diagnostics.HasError(), "a missing user must fail lookup")
	assert.Zero(t, writes, "data source Read must not mutate the API")
}

func TestCoverageEnvironmentNullIsSafe(t *testing.T) {
	for _, input := range []models.Environment{{JSON: "null", Env: "{}"}, {JSON: "{}", Env: "null"}} {
		require.NotPanics(t, func() {
			convertEnvironmentResponseToProjectEnvironmentModel(context.Background(), &input, &ProjectEnvironmentModel{})
		})
	}
}

func TestCoverageExternalEnvironmentDeletionIsVisible(t *testing.T) {
	ctx := context.Background()
	previous := ProjectEnvironmentModel{Secrets: projectEnvironmentTestList(t, projectEnvironmentSecretAttributeTypes(), []ProjectEnvironmentSecretModel{{ID: types.Int64Value(1), Name: types.StringValue("removed"), Type: types.StringValue("var"), Value: types.StringValue("synthetic-value")}})}
	current, err := convertEnvironmentResponseToProjectEnvironmentModel(ctx, &models.Environment{JSON: "{}", Env: "{}"}, &previous)
	require.NoError(t, err)
	assert.True(t, current.Secrets.IsNull() || len(current.Secrets.Elements()) == 0, "externally deleted secret must disappear from state")
}

func TestCoverageTemplateExternalScalarDeletionIsVisible(t *testing.T) {
	previous := ProjectTemplateModel{Description: types.StringValue("removed"), GitBranch: types.StringValue("old-branch"), ViewID: types.Int64Value(4)}
	current := convertTemplateResponseToProjectTemplateModel(context.Background(), &models.Template{}, &previous)
	assert.True(t, current.Description.IsNull())
	assert.True(t, current.GitBranch.IsNull())
	assert.True(t, current.ViewID.IsNull())
}

func TestCoverageTerragruntDoesNotRequirePlaybook(t *testing.T) {
	ctx := context.Background()
	schema := ProjectTemplateSchema().GetResource(ctx)
	config := tfsdk.Config{Schema: schema, Raw: unknownConfigObject(schema.Type().TerraformType(ctx), map[string]any{"app": "terragrunt"})}
	var resp resource.ValidateConfigResponse
	(playbookRequiredValidator{}).ValidateResource(ctx, resource.ValidateConfigRequest{Config: config}, &resp)
	assert.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
}

func TestCoverageDuplicateProjectNameFailsLookup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":1,"name":"same"},{"id":2,"name":"same"}]`))
	}))
	defer server.Close()
	d := &projectDataSource{client: newEXTestClient(t, server.URL)}
	_, err := d.GetProjectByName("same")
	require.Error(t, err)
}
