package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	resourceTest "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestRuntimeSecretsResourceRegistered(t *testing.T) {
	for _, factory := range (&SemaphoreUIProvider{}).Resources(context.Background()) {
		var metadata resource.MetadataResponse
		factory().Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "semaphore_ex"}, &metadata)
		if metadata.TypeName == "semaphore_ex_runtime_secrets" {
			return
		}
	}
	require.Fail(t, "runtime-secrets configured-state resource missing")
}

func TestRuntimeSecretsRejectsUnsupportedServerBeforeMutation(t *testing.T) {
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writes++
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	err := writeRuntimeSecrets(context.Background(), newEXTestClient(t, server.URL), runtimeSecretsModel{State: types.StringValue("disabled")})
	require.ErrorContains(t, err, "does not expose configured")
	assert.Zero(t, writes)
}
func TestRuntimeSecretsReadsConfiguredExpiredState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"runtime_secrets","state":"active","expires_at":"2020-01-01T00:00:00Z"}`))
	}))
	defer server.Close()
	actual, err := readRuntimeSecrets(context.Background(), newEXTestClient(t, server.URL), runtimeSecretsModel{})
	require.NoError(t, err)
	assert.Equal(t, "active", actual.State.ValueString())
	assert.Equal(t, "2020-01-01T00:00:00Z", actual.ExpiresAt.ValueString())
}
func TestRuntimeSecretsDeleteForgetsWithoutDisabling(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(http.StatusInternalServerError) }))
	defer server.Close()
	resourceImpl := runtimeSecretsResource{client: newEXTestClient(t, server.URL)}
	var response resource.DeleteResponse
	resourceImpl.Delete(context.Background(), resource.DeleteRequest{}, &response)
	assert.False(t, response.Diagnostics.HasError())
	assert.Zero(t, calls)
}
func TestAcc_RuntimeSecrets(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance test")
	}
	testAccPreCheck(t)
	initial, err := readRuntimeSecrets(context.Background(), testClient(), runtimeSecretsModel{})
	config := `resource "semaphore_ex_runtime_secrets" "test" { state = "active" }`
	if err != nil && strings.Contains(err.Error(), "does not expose configured") {
		resourceTest.Test(t, resourceTest.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resourceTest.TestStep{{Config: config, ExpectError: regexp.MustCompile("does not expose configured runtime-secrets state")}}})
		return
	}
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, writeRuntimeSecrets(context.Background(), testClient(), initial)) })
	const address = "semaphore_ex_runtime_secrets.test"
	withExpiry := `resource "semaphore_ex_runtime_secrets" "test" {
 state = "active"
 expires_at = "2070-01-01T00:00:00Z"
 }
 data "semaphore_ex_runtime_secrets" "current" { depends_on = [semaphore_ex_runtime_secrets.test] }`
	readOnly := `resource "semaphore_ex_runtime_secrets" "test" { state = "read_only" }
 data "semaphore_ex_runtime_secrets" "current" { depends_on = [semaphore_ex_runtime_secrets.test] }`
	resourceTest.Test(t, resourceTest.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resourceTest.TestStep{
		{Config: withExpiry, Check: resourceTest.TestCheckResourceAttr("data.semaphore_ex_runtime_secrets.current", "expires_at", "2070-01-01T00:00:00Z")},
		{ResourceName: address, ImportState: true, ImportStateVerify: true, ImportStateId: "runtime_secrets"},
		{Config: withExpiry, PlanOnly: true, ExpectNonEmptyPlan: false},
		{Config: readOnly, Check: resourceTest.ComposeAggregateTestCheckFunc(resourceTest.TestCheckResourceAttr(address, "state", "read_only"), resourceTest.TestCheckNoResourceAttr(address, "expires_at"))},
		{Config: readOnly, PlanOnly: true, ExpectNonEmptyPlan: false},
		{Config: `data "semaphore_ex_runtime_secrets" "current" {}`, Check: func(_ *terraform.State) error {
			state, err := readRuntimeSecrets(context.Background(), testClient(), runtimeSecretsModel{})
			if err != nil {
				return err
			}
			if state.State.ValueString() != "read_only" {
				return fmt.Errorf("destroy changed runtime-secrets configuration")
			}
			return nil
		}},
	}})
}
