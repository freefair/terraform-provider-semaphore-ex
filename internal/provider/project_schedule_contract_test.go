package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

func TestScheduleConfigRejectsConflictingAndInvalidTiming(t *testing.T) {
	for _, test := range []struct{ name, kind, cron, once string }{
		{"both timings", "", "0 0 * * *", "2099-01-01T00:00:00Z"},
		{"missing timing", "", "", ""},
		{"kind mismatch", "cron", "", "2099-01-01T00:00:00Z"},
		{"unknown kind", "unsupported", "0 0 * * *", ""},
		{"invalid time", "run_at", "", "tomorrow"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			model := ProjectScheduleModel{Type: stringOrNull(test.kind), CronFormat: stringOrNull(test.cron), RunAt: stringOrNull(test.once)}
			schema := ProjectScheduleSchema().GetResource(ctx)
			state := tfsdk.State{Schema: schema}
			require.False(t, state.Set(ctx, &model).HasError())
			var response resource.ValidateConfigResponse
			(&projectScheduleResource{}).ValidateConfig(ctx, resource.ValidateConfigRequest{Config: tfsdk.Config{Schema: schema, Raw: state.Raw}}, &response)
			require.True(t, response.Diagnostics.HasError())
		})
	}
}

func TestScheduleDeleteAlreadyRemovedIsSuccessful(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	ctx := context.Background()
	schema := ProjectScheduleSchema().GetResource(ctx)
	state := tfsdk.State{Schema: schema}
	require.False(t, state.Set(ctx, &ProjectScheduleModel{ID: types.Int64Value(4), ProjectID: types.Int64Value(2)}).HasError())
	r := projectScheduleResource{client: newEXTestClient(t, server.URL)}
	var response resource.DeleteResponse
	r.Delete(ctx, resource.DeleteRequest{State: state}, &response)
	require.False(t, response.Diagnostics.HasError(), "%v", response.Diagnostics)
}
