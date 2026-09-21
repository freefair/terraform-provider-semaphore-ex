package provider

import (
	"context"
	"fmt"
	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client/user"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource = &externalUserDataSource{}
)

func NewExternalUserDataSource() datasource.DataSource {
	return &externalUserDataSource{}
}

type externalUserDataSource struct {
	client *apiclient.SemaphoreUI
}

func (d *externalUserDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *externalUserDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_external_user"
}

// Schema defines the schema for the data source.
func (d *externalUserDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = ExternalUserSchema().GetDataSource(ctx)
}

func convertResponseToExternalUserModel(user *models.User) ExternalUserModel {
	return ExternalUserModel{
		ID:       types.Int64Value(user.ID),
		Username: types.StringValue(user.Username),
		Name:     types.StringValue(user.Name),
		Email:    types.StringValue(user.Email),
		Admin:    types.BoolValue(user.Admin),
		Alert:    types.BoolValue(user.Alert),
		External: types.BoolValue(user.External),
		Created:  types.StringValue(user.Created),
	}
}

func (r *externalUserDataSource) GetExternalUserByUsername(ctx context.Context, username string) (*ExternalUserModel, error) {
	response, err := r.client.User.GetUsersContext(ctx, &user.GetUsersParams{}, nil)
	if err != nil {
		return nil, fmt.Errorf("could not get users: %s", err.Error())
	}
	for _, usr := range response.Payload {
		if usr.Username == username {
			if !usr.External {
				return nil, fmt.Errorf("user with username %s is not an external user", username)
			}
			model := convertResponseToExternalUserModel(usr)
			return &model, nil
		}
	}
	return nil, fmt.Errorf("user with username %s not found", username)
}

func (d *externalUserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config ExternalUserModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Lookup user by username
	externalUser, err := d.GetExternalUserByUsername(ctx, config.Username.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error Reading SemaphoreUI User", err.Error()+". Manage external users with semaphore_ex_user and external = true; data sources do not create users.")
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, externalUser)...)
	if resp.Diagnostics.HasError() {
		return
	}
}
