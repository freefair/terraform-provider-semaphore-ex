package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func optionActionConfig(key, value string) tfsdk.Config {
	var schema action.SchemaResponse
	(&optionSetAction{}).Schema(context.Background(), action.SchemaRequest{}, &schema)
	return tfsdk.Config{Schema: schema.Schema, Raw: tftypes.NewValue(schema.Schema.Type().TerraformType(context.Background()), map[string]tftypes.Value{"key": tftypes.NewValue(tftypes.String, key), "value": tftypes.NewValue(tftypes.String, value)})}
}
func TestOptionSetActionWritesOnlyOnInvoke(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/options", r.URL.Path)
		var values map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&values))
		assert.Equal(t, map[string]string{"key": "provider_test.option", "value": "fake-sensitive"}, values)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	a := &optionSetAction{client: newEXTestClient(t, server.URL)}
	var schema action.SchemaResponse
	a.Schema(context.Background(), action.SchemaRequest{}, &schema)
	assert.False(t, called)
	response := action.InvokeResponse{SendProgress: func(event action.InvokeProgressEvent) { assert.NotContains(t, event.Message, "fake-sensitive") }}
	a.Invoke(context.Background(), action.InvokeRequest{Config: optionActionConfig("provider_test.option", "fake-sensitive")}, &response)
	require.False(t, response.Diagnostics.HasError(), "%v", response.Diagnostics)
	assert.True(t, called)
}
func TestAcc_Option(t *testing.T) {
	key := "provider_test." + acctest.RandString(8)
	resource.Test(t, resource.TestCase{PreCheck: func() {
		testAccPreCheck(t)
		a := &optionSetAction{client: testClient()}
		response := action.InvokeResponse{}
		a.Invoke(context.Background(), action.InvokeRequest{Config: optionActionConfig(key, "acceptance")}, &response)
		require.False(t, response.Diagnostics.HasError(), "%v", response.Diagnostics)
	}, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: fmt.Sprintf(`data "semaphore_ex_option" "test" { key = %q }`, key), Check: resource.TestCheckResourceAttr("data.semaphore_ex_option.test", "value", "acceptance")},
	}})
}
