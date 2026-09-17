package provider

import (
	"context"
	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type projectTemplateInventoryDataSource struct{ client *apiclient.SemaphoreUI }

func NewProjectTemplateInventoryDataSource() datasource.DataSource {
	return &projectTemplateInventoryDataSource{}
}
func (d *projectTemplateInventoryDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_template_inventory"
}
func (d *projectTemplateInventoryDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = ProjectTemplateInventorySchema().GetDataSource(ctx)
}
func (d *projectTemplateInventoryDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Invalid Provider Client", "Expected the configured Semaphore EX client.")
		return
	}
	d.client = client
}
func (d *projectTemplateInventoryDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config projectTemplateInventoryModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	attached, err := templateInventoryAttached(ctx, d.client, config)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Inventory Attachment", err.Error())
		return
	}
	if !attached {
		resp.Diagnostics.AddError("Inventory Is Not Attached", "The requested inventory is not attached to the specified template.")
		return
	}
	config.ID = types.StringValue(templateInventoryIdentity(config))
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
