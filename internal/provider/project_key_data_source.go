package provider

import (
	"context"
	"fmt"
	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource = &projectKeyDataSource{}
)

func NewProjectKeyDataSource() datasource.DataSource {
	return withNamedLookup(&projectKeyDataSource{}, "project_key")
}

type projectKeyDataSource struct {
	client *apiclient.SemaphoreUI
}

func (d *projectKeyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			"Expected *client.SemaphoreUI, got %T. Please report this issue to the provider developers.",
		)
		return
	}
	d.client = client
}

// Metadata returns the data source type name.
func (d *projectKeyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_key"
}

// Schema defines the schema for the data source.
func (d *projectKeyDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = ProjectKeySchema().GetDataSource(ctx)
}

func (d *projectKeyDataSource) GetKeyByID(ctx context.Context, projectID int64, id int64) (*ProjectKeyModel, error) {
	var key models.AccessKey
	if err := exRequest(ctx, d.client, http.MethodGet, "/project/{project_id}/keys/{key_id}", map[string]string{"project_id": strconv.FormatInt(projectID, 10), "key_id": strconv.FormatInt(id, 10)}, nil, &key); err != nil {
		return nil, err
	}
	if key.ID != id || key.ProjectID != projectID {
		return nil, fmt.Errorf("key detail returned a different identity")
	}
	model := ProjectKeyModel{ProjectID: types.Int64Value(key.ProjectID), ID: types.Int64Value(key.ID), Name: types.StringValue(key.Name)}
	// The read DTO intentionally redacts login and secret material. Unknown values
	// stay null instead of falsely claiming an empty password or private key.
	switch key.Type {
	case ProjectKeyTypeNone:
		model.None = &ProjectKeyNone{}
	case ProjectKeyTypeLoginPassword:
		model.LoginPassword = &ProjectKeyLoginPassword{}
	case ProjectKeyTypeSSH:
		model.SSH = &ProjectKeySSH{}
	case ProjectKeyTypeString:
		model.String = &ProjectKeyString{}
	default:
		return nil, fmt.Errorf("key detail returned an unsupported key type")
	}
	if key.SourceStorageType != nil {
		model.RemoteReference = remoteReferenceFromAccessKey(&key)
	}
	return &model, nil
}

func (d *projectKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config ProjectKeyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if config.ID.IsNull() || config.ID.IsUnknown() {
		resp.Diagnostics.AddError("Invalid Key Identity", "Resolve a known key id before reading details.")
		return
	}
	model, err := d.GetKeyByID(ctx, config.ProjectID.ValueInt64(), config.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Semaphore EX Project Key", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}
