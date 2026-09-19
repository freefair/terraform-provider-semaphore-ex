package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.ResourceWithModifyPlan = &projectTemplateResource{}

// ModifyPlan preserves the computed environment alias when its configured
// counterpart has not changed. Removing an optional SSH override can otherwise
// mark both aliases unknown before the SSH plan modifier restores prior state.
func (r *projectTemplateResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var configuredID, priorID, plannedID types.Int64
	var configuredIDs, priorIDs, plannedIDs types.Set
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("environment_id"), &configuredID)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("environment_ids"), &configuredIDs)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("environment_id"), &priorID)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("environment_ids"), &priorIDs)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("environment_id"), &plannedID)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("environment_ids"), &plannedIDs)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !configuredID.IsNull() && !configuredID.IsUnknown() && configuredID.Equal(priorID) && plannedIDs.IsUnknown() {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("environment_ids"), priorIDs)...)
	}
	if !configuredIDs.IsNull() && !configuredIDs.IsUnknown() && configuredIDs.Equal(priorIDs) && plannedID.IsUnknown() {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("environment_id"), priorID)...)
	}
}
