package provider

import (
	"context"
	"net/http"
	"regexp"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type optionDataSource struct{ client *apiclient.SemaphoreUI }
type optionModel struct {
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

func NewOptionDataSource() datasource.DataSource { return &optionDataSource{} }
func (d *optionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_option"
}
func (d *optionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *optionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Reads one persisted global system option. Only the selected value is stored in state. Values are sensitive because system options may contain credentials. The persisted value may require a server restart to become effective.", Attributes: map[string]schema.Attribute{
		"key":   schema.StringAttribute{Required: true, MarkdownDescription: "Exact system option key.", Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[\w.]+$`), "Use letters, digits, underscores and dots.")}},
		"value": schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Persisted option value."},
	}}
}
func (d *optionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config optionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var values map[string]string
	if err := exRequest(ctx, d.client, http.MethodGet, "/options", nil, nil, &values); err != nil {
		resp.Diagnostics.AddError("Error Reading System Option", err.Error())
		return
	}
	value, exists := values[config.Key.ValueString()]
	if !exists {
		resp.Diagnostics.AddError("System Option Not Found", "The requested persisted key does not exist.")
		return
	}
	config.Value = types.StringValue(value)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
