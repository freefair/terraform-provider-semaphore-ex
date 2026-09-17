package provider

import (
	"context"
	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	schemaD "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

type projectSecretStorageDataSource struct{ client *apiclient.SemaphoreUI }

func NewProjectSecretStorageDataSource() datasource.DataSource {
	return &projectSecretStorageDataSource{}
}
func (d *projectSecretStorageDataSource) Metadata(_ context.Context, q datasource.MetadataRequest, p *datasource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_project_secret_storage"
}
func (d *projectSecretStorageDataSource) Configure(_ context.Context, q datasource.ConfigureRequest, p *datasource.ConfigureResponse) {
	if q.ProviderData == nil {
		return
	}
	c, ok := q.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		p.Diagnostics.AddError("Unexpected Data Source Configure Type", "Expected *client.SemaphoreUI.")
		return
	}
	d.client = c
}
func (d *projectSecretStorageDataSource) Schema(c context.Context, _ datasource.SchemaRequest, p *datasource.SchemaResponse) {
	p.Schema = ProjectSecretStorageSchema().GetDataSource(c)
	p.Schema.Attributes["params"] = schemaD.DynamicAttribute{Computed: true}
}
func (d *projectSecretStorageDataSource) Read(c context.Context, q datasource.ReadRequest, p *datasource.ReadResponse) {
	var m ProjectSecretStorageModel
	p.Diagnostics.Append(q.Config.Get(c, &m)...)
	if p.Diagnostics.HasError() {
		return
	}
	r := projectSecretStorageResource{client: d.client}
	n, e := r.read(c, m)
	if e != nil {
		p.Diagnostics.AddError("Error Reading Project Secret Storage", "The Semaphore EX API could not read the storage.")
		return
	}
	p.Diagnostics.Append(p.State.Set(c, &n)...)
}
