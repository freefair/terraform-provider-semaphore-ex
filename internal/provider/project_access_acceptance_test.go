package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Exercise the real server's deliberate 404 response after revoking a project
// membership, using synthetic users and credentials in the disposable fixture.
func TestAcc_ProjectAccessRevocationRetainsState(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance test requires TF_ACC")
	}
	testAccPreCheck(t)
	ctx := context.Background()
	admin := testClient()
	suffix := acctest.RandString(10)
	var project struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, exRequest(ctx, admin, http.MethodPost, "/projects", nil, map[string]any{"name": "access-" + suffix}, &project))
	require.Positive(t, project.ID)
	t.Cleanup(func() {
		assert.NoError(t, exRequest(ctx, admin, http.MethodDelete, "/project/{project_id}", map[string]string{"project_id": fmt.Sprint(project.ID)}, nil, nil))
	})
	var user struct {
		ID int64 `json:"id"`
	}
	loginName := "access-" + suffix
	loginSecret := acctest.RandString(32)
	require.NoError(t, exRequest(ctx, admin, http.MethodPost, "/users", nil, map[string]any{"username": loginName, "name": "Access test", "email": "access@example.test", "password": loginSecret}, &user))
	require.Positive(t, user.ID)
	t.Cleanup(func() {
		assert.NoError(t, exRequest(ctx, admin, http.MethodDelete, "/users/{user_id}", map[string]string{"user_id": fmt.Sprint(user.ID)}, nil, nil))
	})
	params := map[string]string{"project_id": fmt.Sprint(project.ID), "user_id": fmt.Sprint(user.ID)}
	require.NoError(t, exRequest(ctx, admin, http.MethodPost, "/project/{project_id}/users", params, map[string]any{"user_id": user.ID, "role": "guest"}, nil))
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	session := &http.Client{Jar: jar}
	post := func(route string, payload any) *http.Response {
		t.Helper()
		encoded, err := json.Marshal(payload)
		require.NoError(t, err)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, os.Getenv("SEMAPHOREUI_API_BASE_URL")+route, bytes.NewReader(encoded))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		resp, err := session.Do(req)
		require.NoError(t, err)
		require.Less(t, resp.StatusCode, 300)
		return resp
	}
	signedIn := post("/auth/login", map[string]string{"auth": loginName, "password": loginSecret})
	require.NoError(t, signedIn.Body.Close())
	tokenResponse := post("/user/tokens", map[string]any{"name": "access-test"})
	var token struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(tokenResponse.Body).Decode(&token))
	require.NoError(t, tokenResponse.Body.Close())
	require.NotEmpty(t, token.ID)
	r := NewProjectResource()
	var configured resource.ConfigureResponse
	r.(resource.ResourceWithConfigure).Configure(ctx, resource.ConfigureRequest{ProviderData: configuredLifecycleClient(t, os.Getenv("SEMAPHOREUI_API_BASE_URL"), token.ID)}, &configured)
	state := lifecycleState(t, r)
	require.False(t, state.SetAttribute(ctx, path.Root("id"), project.ID).HasError())
	before := resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, &before)
	require.False(t, before.Diagnostics.HasError(), "%v", before.Diagnostics)
	require.NoError(t, exRequest(ctx, admin, http.MethodDelete, "/project/{project_id}/users/{user_id}", params, nil, nil))
	after := resource.ReadResponse{State: before.State}
	r.Read(ctx, resource.ReadRequest{State: before.State}, &after)
	require.True(t, after.Diagnostics.HasError())
	assert.Equal(t, before.State.Raw, after.State.Raw)
	var stillExists struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, exRequest(ctx, admin, http.MethodGet, "/project/{project_id}", params, nil, &stillExists))
	assert.Equal(t, project.ID, stillExists.ID)
}
