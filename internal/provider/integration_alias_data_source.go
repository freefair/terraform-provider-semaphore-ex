package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// integrationAliasDataSource shares the resource's scoped lookup so reads cannot
// create webhook URLs or accidentally select an alias from a different scope.
type integrationAliasDataSource struct{ integrationAliasResource }

func NewIntegrationAliasDataSource() datasource.DataSource { return &integrationAliasDataSource{} }
func (d *integrationAliasDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_alias"
}
func (d *integrationAliasDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = IntegrationAliasSchema().GetDataSource(ctx)
}
func (d *integrationAliasDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	var configured resource.ConfigureResponse
	d.integrationAliasResource.Configure(ctx, resource.ConfigureRequest{ProviderData: req.ProviderData}, &configured)
	resp.Diagnostics.Append(configured.Diagnostics...)
}
func (d *integrationAliasDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config IntegrationAliasModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	alias, err := d.findAlias(config.ProjectID.ValueInt64(), config.IntegrationID.ValueInt64(), config.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Integration Alias", err.Error())
		return
	}
	if alias == nil {
		resp.Diagnostics.AddError("Integration Alias Not Found", fmt.Sprintf("No alias with ID %d exists in the selected scope.", config.ID.ValueInt64()))
		return
	}
	config.URL = types.StringValue(alias.URL)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
