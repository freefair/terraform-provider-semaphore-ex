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

func TestKeyDataSourceUsesDetailAndLeavesRedactedValuesNull(t *testing.T) {
	detail := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/project/1/keys/2" {
			detail = true
			_, _ = w.Write([]byte(`{"id":2,"project_id":1,"name":"login","type":"login_password","login_password":{}}`))
			return
		}
		_, _ = w.Write([]byte(`[{"id":2,"project_id":1,"name":"login","type":"login_password"}]`))
	}))
	defer server.Close()
	ctx := context.Background()
	source := NewProjectKeyDataSource()
	var schema datasource.SchemaResponse
	source.Schema(ctx, datasource.SchemaRequest{}, &schema)
	var configured datasource.ConfigureResponse
	source.(datasource.DataSourceWithConfigure).Configure(ctx, datasource.ConfigureRequest{ProviderData: newEXTestClient(t, server.URL)}, &configured)
	response := datasource.ReadResponse{State: tfsdk.State{Schema: schema.Schema}}
	source.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schema.Schema, Raw: unknownConfigObject(schema.Schema.Type().TerraformType(ctx), map[string]any{"project_id": int64(1), "id": int64(2)})}}, &response)
	require.False(t, response.Diagnostics.HasError(), "%v", response.Diagnostics)
	assert.True(t, detail)
	var password types.String
	require.False(t, response.State.GetAttribute(ctx, path.Root("login_password").AtName("password"), &password).HasError())
	assert.True(t, password.IsNull(), "redacted is not an empty password")
}
