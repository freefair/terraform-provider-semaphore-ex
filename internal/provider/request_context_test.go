package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestGeneratedReadsHonorCanceledContext(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":2,"project_id":1,"name":"env","json":"{}","env":"{}"}`))
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	source := &projectEnvironmentDataSource{client: newEXTestClient(t, server.URL)}
	var schema datasource.SchemaResponse
	source.Schema(ctx, datasource.SchemaRequest{}, &schema)
	response := datasource.ReadResponse{State: tfsdk.State{Schema: schema.Schema}}
	source.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schema.Schema, Raw: unknownConfigObject(schema.Schema.Type().TerraformType(ctx), map[string]any{"project_id": int64(1), "id": int64(2)})}}, &response)
	assert.True(t, response.Diagnostics.HasError(), "%v", response.Diagnostics)
	assert.Zero(t, requests.Load(), "a canceled read must never reach the server")
}
func TestGeneratedReadsHonorDeadlineInFlight(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(2 * time.Second):
			w.WriteHeader(http.StatusGatewayTimeout)
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	source := &projectEnvironmentDataSource{client: newEXTestClient(t, server.URL)}
	var schema datasource.SchemaResponse
	source.Schema(ctx, datasource.SchemaRequest{}, &schema)
	response := datasource.ReadResponse{State: tfsdk.State{Schema: schema.Schema}}
	started := time.Now()
	source.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schema.Schema, Raw: unknownConfigObject(schema.Schema.Type().TerraformType(ctx), map[string]any{"project_id": int64(1), "id": int64(2)})}}, &response)
	require.True(t, response.Diagnostics.HasError())
	assert.Less(t, time.Since(started), time.Second, "deadline must cancel an in-flight generated request")
}
