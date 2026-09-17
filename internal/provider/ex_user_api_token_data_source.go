package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func (d *exUserAPITokenDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config exUserAPITokenDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	raw, err := exFindUserAPIToken(ctx, d.client, config.TokenID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Semaphore EX API Token", err.Error())
		return
	}
	state := exUserAPITokenDataSourceModel{ID: types.StringValue(raw.TokenRef), TokenID: types.StringValue(raw.TokenRef), Name: types.StringValue(raw.Name), Created: types.StringValue(raw.Created), Expired: types.BoolValue(raw.Expired), UserID: types.Int64Value(raw.UserID)}
	if raw.ExpiresAt == nil {
		state.ExpiresAt = types.StringNull()
	} else {
		state.ExpiresAt = types.StringValue(*raw.ExpiresAt)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
