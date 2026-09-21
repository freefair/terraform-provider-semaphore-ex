package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	acctestresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistrationTokenImportNeverIssuesCredential(t *testing.T) {
	for _, id := range []string{"runner/7", "project/2/runner/7"} {
		t.Run(id, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("import must not issue a request") }))
			defer server.Close()
			r := NewRunnerRegistrationTokenResource()
			var configured resource.ConfigureResponse
			r.(resource.ResourceWithConfigure).Configure(context.Background(), resource.ConfigureRequest{ProviderData: newEXTestClient(t, server.URL)}, &configured)
			var schema resource.SchemaResponse
			r.Schema(context.Background(), resource.SchemaRequest{}, &schema)
			resp := resource.ImportStateResponse{State: tfsdk.State{Schema: schema.Schema}}
			r.(resource.ResourceWithImportState).ImportState(context.Background(), resource.ImportStateRequest{ID: id}, &resp)
			require.False(t, resp.Diagnostics.HasError())
			var got RunnerRegistrationTokenModel
			require.False(t, resp.State.Get(context.Background(), &got).HasError())
			assert.Equal(t, id, got.ID.ValueString())
			assert.Equal(t, types.Int64Value(7), got.RunnerID)
			assert.True(t, got.RegistrationToken.IsNull())
		})
	}
}
func TestAcc_RegistrationTokenImport(t *testing.T) {
	config := fmt.Sprintf("resource \"semaphore_ex_runner\" \"test\" {\n name = %q\n active = false\n}\nresource \"semaphore_ex_runner_registration_token\" \"test\" {\n runner_id = semaphore_ex_runner.test.id\n keepers = { rotation = \"1\" }\n}\n", "import-token-"+acctest.RandString(8))
	runnerConfig := strings.Split(config, "resource \"semaphore_ex_runner_registration_token\"")[0]
	acctestresource.Test(t, acctestresource.TestCase{
		PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []acctestresource.TestStep{
			{Config: config},
			{Config: runnerConfig + `
removed {
 from = semaphore_ex_runner_registration_token.test
 lifecycle { destroy = false }
}
`},
			{Config: config + `
import {
 to = semaphore_ex_runner_registration_token.test
 id = "runner/${semaphore_ex_runner.test.id}"
}
`, Check: acctestresource.TestCheckNoResourceAttr("semaphore_ex_runner_registration_token.test", "registration_token")},
			{Config: config, PlanOnly: true},
			{Config: strings.Replace(config, `rotation = "1"`, `rotation = "2"`, 1),
				ConfigPlanChecks: acctestresource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("semaphore_ex_runner_registration_token.test", plancheck.ResourceActionReplace)}},
				Check:            acctestresource.TestCheckResourceAttrSet("semaphore_ex_runner_registration_token.test", "registration_token")},
		},
	})
}
