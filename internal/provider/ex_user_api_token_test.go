package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	schemaD "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	schemaR "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	resourceTest "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const exTestStableUserAPITokenID = exUserAPITokenIDPrefix + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func exUserAPITokenTestValue(t *testing.T) string {
	t.Helper()
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	require.NoError(t, err)
	return hex.EncodeToString(bytes)
}

func TestEXUserAPITokenPreflightRejectsLegacyServerBeforeCreate(t *testing.T) {
	var getCount, postCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/user/tokens" && request.Method == http.MethodGet {
			getCount++
			_, _ = w.Write([]byte(`[{"id":"legacypr"}]`))
			return
		}
		if request.URL.Path == "/api/user/tokens" && request.Method == http.MethodPost {
			postCount++
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := exCreateUserAPIToken(context.Background(), newEXTestClient(t, server.URL), "automation", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stable token references")
	assert.Equal(t, 1, getCount)
	assert.Zero(t, postCount)
}

func TestEXUserAPITokenListRejectsEmptyAuthenticatedOwnerList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "/api/user/tokens", request.URL.Path)
		assert.Equal(t, http.MethodGet, request.Method)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	_, err := exListUserAPITokens(context.Background(), newEXTestClient(t, server.URL))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not list the configured API token")
}

func TestEXUserAPITokenFindUsesExactStableIdentifier(t *testing.T) {
	secondID := exUserAPITokenIDPrefix + strings.Repeat("a", 64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "/api/user/tokens", request.URL.Path)
		_ = json.NewEncoder(w).Encode([]exUserAPITokenAPIResponse{
			{ID: "shortened", TokenRef: exTestStableUserAPITokenID, Name: "first"},
			{ID: "shortened", TokenRef: secondID, Name: "second"},
		})
	}))
	defer server.Close()

	found, err := exFindUserAPIToken(context.Background(), newEXTestClient(t, server.URL), secondID)
	require.NoError(t, err)
	assert.Equal(t, "second", found.Name)
	assert.Equal(t, secondID, found.TokenRef)
}

func TestEXUserAPITokenStateKeepsOneTimeValueOnlyFromCreate(t *testing.T) {
	oneTimeValue := exUserAPITokenTestValue(t)
	state, err := exUserAPITokenState(exUserAPITokenModel{}, exUserAPITokenAPIResponse{ID: oneTimeValue, TokenRef: exTestStableUserAPITokenID, Name: "automation"}, types.StringValue(oneTimeValue))
	require.NoError(t, err)
	assert.Equal(t, oneTimeValue, state.Credential.ValueString())

	refreshed, err := exUserAPITokenState(state, exUserAPITokenAPIResponse{ID: "shortened", TokenRef: exTestStableUserAPITokenID, Name: "automation"}, state.Credential)
	require.NoError(t, err)
	assert.Equal(t, oneTimeValue, refreshed.Credential.ValueString())
	assert.NotEqual(t, "shortened", refreshed.Credential.ValueString())

	imported, err := exUserAPITokenState(exUserAPITokenModel{Credential: types.StringNull()}, exUserAPITokenAPIResponse{TokenRef: exTestStableUserAPITokenID, Name: "automation"}, types.StringNull())
	require.NoError(t, err)
	assert.True(t, imported.Credential.IsNull())
}

func TestEXUserAPITokenDeleteUsesOnlyStableIdentifier(t *testing.T) {
	oneTimeValue := exUserAPITokenTestValue(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assert.Equal(t, http.MethodDelete, request.Method)
		assert.Equal(t, "/api/user/tokens/"+exTestStableUserAPITokenID, request.URL.Path)
		assert.NotContains(t, request.RequestURI, oneTimeValue)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	err := exRequest(context.Background(), newEXTestClient(t, server.URL), http.MethodDelete, "/user/tokens/{token_id}", map[string]string{"token_id": exTestStableUserAPITokenID}, nil, nil)
	require.NoError(t, err)
}

func TestEXUserAPITokenSchemaKeepsOneTimeValueSensitiveAndDataSourceValueFree(t *testing.T) {
	resourceAttribute, ok := exUserAPITokenResourceSchema().Attributes["credential"].(schemaR.StringAttribute)
	require.True(t, ok)
	assert.True(t, resourceAttribute.Computed)
	assert.True(t, resourceAttribute.Sensitive)
	_, listedByDataSource := exUserAPITokenDataSourceSchema().Attributes["credential"]
	assert.False(t, listedByDataSource)
	tokenIDInput, ok := exUserAPITokenDataSourceSchema().Attributes["token_id"].(schemaD.StringAttribute)
	require.True(t, ok)
	assert.True(t, tokenIDInput.Required)
}

func TestEXUserAPITokenStableIDValidation(t *testing.T) {
	assert.True(t, exUserAPITokenIDValid(exTestStableUserAPITokenID))
	assert.False(t, exUserAPITokenIDValid("legacy-prefix"))
	assert.False(t, exUserAPITokenIDValid(strings.ToUpper(exTestStableUserAPITokenID)))
}

func TestAcc_EXUserAPIToken(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}

	name := "acceptance-api-token-" + acctest.RandString(8)
	config := `
resource "semaphore_ex_user_api_token" "test" {
  name = "` + name + `"
  keepers = {
    rotation = "one"
  }
}

data "semaphore_ex_user_api_token" "test" {
  token_id = semaphore_ex_user_api_token.test.id
}
`
	replacement := `
resource "semaphore_ex_user_api_token" "test" {
  name = "` + name + `"
  keepers = {
    rotation = "two"
  }
}

data "semaphore_ex_user_api_token" "test" {
  token_id = semaphore_ex_user_api_token.test.id
}
`

	var initialID string
	captureInitialID := func(state *terraform.State) error {
		initialID = state.RootModule().Resources["semaphore_ex_user_api_token.test"].Primary.ID
		return nil
	}
	requireReplacement := func(state *terraform.State) error {
		currentID := state.RootModule().Resources["semaphore_ex_user_api_token.test"].Primary.ID
		if initialID == "" || currentID == initialID {
			return fmt.Errorf("changing keepers did not replace the API token")
		}
		return nil
	}

	resourceTest.Test(t, resourceTest.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resourceTest.TestStep{
			{
				Config: config,
				Check: resourceTest.ComposeAggregateTestCheckFunc(
					resourceTest.TestCheckResourceAttrSet("semaphore_ex_user_api_token.test", "id"),
					resourceTest.TestCheckResourceAttrSet("semaphore_ex_user_api_token.test", "credential"),
					resourceTest.TestCheckResourceAttr("data.semaphore_ex_user_api_token.test", "name", name),
					captureInitialID,
				),
			},
			{
				Config: replacement,
				Check: resourceTest.ComposeAggregateTestCheckFunc(
					resourceTest.TestCheckResourceAttr("data.semaphore_ex_user_api_token.test", "name", name),
					requireReplacement,
				),
			},
			{
				ResourceName:            "semaphore_ex_user_api_token.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"credential", "keepers"},
				ImportStateIdFunc: func(state *terraform.State) (string, error) {
					return state.RootModule().Resources["semaphore_ex_user_api_token.test"].Primary.ID, nil
				},
			},
		},
	})
}
