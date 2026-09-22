package provider

import (
	"context"
	"fmt"
	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type projectTaskGroupDataSource struct{ client *apiclient.SemaphoreUI }

func NewProjectTaskGroupDataSource() datasource.DataSource { return &projectTaskGroupDataSource{} }
func (d *projectTaskGroupDataSource) Metadata(_ context.Context, q datasource.MetadataRequest, p *datasource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_project_task_group"
}
func (d *projectTaskGroupDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, p *datasource.SchemaResponse) {
	p.Schema = projectTaskGroupSchema().GetDataSource(ctx)
}
func (d *projectTaskGroupDataSource) Configure(_ context.Context, q datasource.ConfigureRequest, p *datasource.ConfigureResponse) {
	if q.ProviderData == nil {
		return
	}
	client, ok := q.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		p.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *client.SemaphoreUI, got %T", q.ProviderData))
		return
	}
	d.client = client
}
func (d *projectTaskGroupDataSource) Read(ctx context.Context, q datasource.ReadRequest, p *datasource.ReadResponse) {
	var m projectTaskGroupModel
	p.Diagnostics.Append(q.Config.Get(ctx, &m)...)
	if p.Diagnostics.HasError() {
		return
	}
	m, err := readTaskGroup(ctx, d.client, m)
	if err != nil {
		p.Diagnostics.AddError("Error Reading Task Group", err.Error())
		return
	}
	if !m.ProjectID.Equal(m.OwnerProjectID) {
		m.SharedProjectIDs = types.SetNull(types.Int64Type)
	}
	p.Diagnostics.Append(p.State.Set(ctx, &m)...)
}
