package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ resource.ResourceWithValidateConfig = &projectScheduleResource{}

func (r *projectScheduleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config ProjectScheduleModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if config.CronFormat.IsUnknown() || config.RunAt.IsUnknown() {
		return
	}
	cron, once := !config.CronFormat.IsNull(), !config.RunAt.IsNull()
	if cron == once {
		resp.Diagnostics.AddAttributeError(path.Root("run_at"), "Invalid Schedule Timing", "Configure exactly one of cron_format or run_at.")
	}
	if once {
		if _, err := time.Parse(time.RFC3339Nano, config.RunAt.ValueString()); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("run_at"), "Invalid Schedule Time", "run_at must be an RFC3339 timestamp including a timezone.")
		}
	}
	if !config.Type.IsNull() && !config.Type.IsUnknown() {
		want := "cron"
		if once {
			want = "run_at"
		}
		if config.Type.ValueString() != want {
			resp.Diagnostics.AddAttributeError(path.Root("type"), "Inconsistent Schedule Type", "type must match the configured cron_format or run_at timing.")
		}
	}
}
