package provider

import (
	"context"
	"encoding/json"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNamedEnvironmentLookup(t *testing.T) {
	for _, tc := range []struct {
		name      string
		records   []map[string]any
		wantError bool
	}{
		{"unique", []map[string]any{{"id": 2, "name": "deploy"}}, false},
		{"missing", []map[string]any{{"id": 3, "name": "other"}}, true},
		{"ambiguous", []map[string]any{{"id": 2, "name": "deploy"}, {"id": 3, "name": "deploy"}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			detailReads := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api/project/1/environment" {
					require.NoError(t, json.NewEncoder(w).Encode(tc.records))
					return
				}
				assert.Equal(t, "/api/project/1/environment/2", r.URL.Path)
				detailReads++
				_, _ = w.Write([]byte(`{"id":2,"project_id":1,"name":"deploy","json":"{\"detail\":\"available\"}","env":"{}"}`))
			}))
			defer server.Close()
			ctx := context.Background()
			source := NewProjectEnvironmentDataSource()
			var schema datasource.SchemaResponse
			source.Schema(ctx, datasource.SchemaRequest{}, &schema)
			require.True(t, schema.Schema.Attributes["name"].IsOptional(), "name must be a selector")
			var configured datasource.ConfigureResponse
			source.(datasource.DataSourceWithConfigure).Configure(ctx, datasource.ConfigureRequest{ProviderData: newEXTestClient(t, server.URL)}, &configured)
			require.False(t, configured.Diagnostics.HasError())
			response := datasource.ReadResponse{State: tfsdk.State{Schema: schema.Schema}}
			source.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schema.Schema, Raw: unknownConfigObject(schema.Schema.Type().TerraformType(ctx), map[string]any{"project_id": int64(1), "name": "deploy"})}}, &response)
			require.Equal(t, tc.wantError, response.Diagnostics.HasError(), "%v", response.Diagnostics)
			if tc.wantError {
				assert.Zero(t, detailReads)
				return
			}
			assert.Equal(t, 1, detailReads)
			var actual types.String
			require.False(t, response.State.GetAttribute(ctx, path.Root("variables_json"), &actual).HasError())
			assert.JSONEq(t, `{"detail":"available"}`, actual.ValueString())
		})
	}
}

func TestLookupPaginationRejectsDuplicatesAcrossPages(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "25", r.URL.Query().Get("count"))
		calls++
		page := []map[string]any{}
		if calls == 1 {
			for i := 1; i <= 25; i++ {
				page = append(page, map[string]any{"id": i, "name": "other"})
			}
		} else {
			page = append(page, map[string]any{"id": 1, "name": "duplicate"})
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(page))
	}))
	defer server.Close()
	_, err := readCollection(context.Background(), newEXTestClient(t, server.URL), collectionSpec{route: "/global-credentials", paged: true}, nil)
	require.ErrorContains(t, err, "repeated an identity")
	assert.Equal(t, 2, calls)
}

func TestNamedLookupFindsAmbiguityOnLaterPage(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/global-credentials", r.URL.Path)
		calls++
		page := []map[string]any{}
		if calls == 1 {
			for i := 1; i <= 25; i++ {
				name := "other"
				if i == 1 {
					name = "shared"
				}
				page = append(page, map[string]any{"id": i, "display_name": name})
			}
		} else {
			page = append(page, map[string]any{"id": 26, "display_name": "shared"})
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(page))
	}))
	defer server.Close()
	ctx := context.Background()
	source := NewGlobalCredentialDataSource()
	var schema datasource.SchemaResponse
	source.Schema(ctx, datasource.SchemaRequest{}, &schema)
	var configured datasource.ConfigureResponse
	source.(datasource.DataSourceWithConfigure).Configure(ctx, datasource.ConfigureRequest{ProviderData: newEXTestClient(t, server.URL)}, &configured)
	response := datasource.ReadResponse{State: tfsdk.State{Schema: schema.Schema}}
	source.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schema.Schema, Raw: unknownConfigObject(schema.Schema.Type().TerraformType(ctx), map[string]any{"display_name": "shared"})}}, &response)
	require.True(t, response.Diagnostics.HasError())
	assert.Contains(t, response.Diagnostics[0].Summary(), "Ambiguous")
	assert.Equal(t, 2, calls)
}

func TestAcc_NamedEnvironmentLookup(t *testing.T) {
	suffix := acctest.RandString(8)
	config := testAccProjectTemplateConfig(suffix, "") + `
 data "semaphore_ex_project_environment" "named" {
 project_id = semaphore_ex_project.test.id
 name = semaphore_ex_project_environment.test.name
 }
 data "semaphore_ex_project_template" "named" {
 project_id = semaphore_ex_project.test.id
 name = semaphore_ex_project_template.test.name
 }
 `
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{{Config: config, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttrPair("data.semaphore_ex_project_environment.named", "id", "semaphore_ex_project_environment.test", "id"), resource.TestCheckResourceAttrPair("data.semaphore_ex_project_template.named", "id", "semaphore_ex_project_template.test", "id"))}}})
}
