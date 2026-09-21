package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunnerDataSourceReadsOperationalMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":2,"name":"runner","status":"online","version":"fixture-version","platform":"linux/amd64","current_load":3,"executor_type":"docker","docker_policy_revision":4,"security_compliant":true}`))
	}))
	defer server.Close()
	ctx := context.Background()
	source := NewRunnerDataSource()
	var schema datasource.SchemaResponse
	source.Schema(ctx, datasource.SchemaRequest{}, &schema)
	require.Contains(t, schema.Schema.Attributes, "status")
	assert.NotContains(t, RunnerSchema().GetResource(ctx).Attributes, "status")
	var configured datasource.ConfigureResponse
	source.(datasource.DataSourceWithConfigure).Configure(ctx, datasource.ConfigureRequest{ProviderData: newEXTestClient(t, server.URL)}, &configured)
	response := datasource.ReadResponse{State: tfsdk.State{Schema: schema.Schema}}
	source.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schema.Schema, Raw: unknownConfigObject(schema.Schema.Type().TerraformType(ctx), map[string]any{"id": int64(2)})}}, &response)
	require.False(t, response.Diagnostics.HasError(), "%v", response.Diagnostics)
	var status types.String
	var load types.Int64
	require.False(t, response.State.GetAttribute(ctx, path.Root("status"), &status).HasError())
	require.False(t, response.State.GetAttribute(ctx, path.Root("current_load"), &load).HasError())
	assert.Equal(t, "online", status.ValueString())
	assert.Equal(t, int64(3), load.ValueInt64())
}
