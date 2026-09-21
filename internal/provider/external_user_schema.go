package provider

import (
	schemaD "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	schemaR "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	superschema "github.com/orange-cloudavenue/terraform-plugin-framework-superschema"
)

type ExternalUserModel struct {
	ID       types.Int64  `tfsdk:"id"`
	Username types.String `tfsdk:"username"`
	Name     types.String `tfsdk:"name"`
	Email    types.String `tfsdk:"email"`
	Admin    types.Bool   `tfsdk:"admin"`
	Alert    types.Bool   `tfsdk:"alert"`
	External types.Bool   `tfsdk:"external"`
	Created  types.String `tfsdk:"created"`
}

func ExternalUserSchema() superschema.Schema {
	return superschema.Schema{
		DataSource: superschema.SchemaDetails{
			MarkdownDescription: "Looks up an existing external user by username. A missing user is an error. Manage users with the semaphore_ex_user resource and external = true; this data source never creates or updates users.",
		},
		Attributes: map[string]superschema.Attribute{
			"username": superschema.StringAttribute{
				Common: &schemaR.StringAttribute{
					MarkdownDescription: "Username.",
				},
				DataSource: &schemaD.StringAttribute{
					Required: true,
				},
			},
			"id": superschema.Int64Attribute{
				DataSource: &schemaD.Int64Attribute{
					MarkdownDescription: "The ID of the external user.",
					Computed:            true,
				},
			},
			"name": superschema.StringAttribute{
				DataSource: &schemaD.StringAttribute{
					MarkdownDescription: "Display name returned by the server. Optional input is retained for compatibility and does not change the user; leave it unset for lookup.",
					Optional:            true,
					Computed:            true,
				},
			},
			"email": superschema.StringAttribute{
				DataSource: &schemaD.StringAttribute{
					MarkdownDescription: "Email address returned by the server. Optional input is retained for compatibility and does not change the user; leave it unset for lookup.",
					Optional:            true,
					Computed:            true,
				},
			},
			"admin": superschema.BoolAttribute{
				DataSource: &schemaD.BoolAttribute{
					MarkdownDescription: "Indicates if the user is an admin.",
					Computed:            true,
				},
			},
			"alert": superschema.BoolAttribute{
				DataSource: &schemaD.BoolAttribute{
					MarkdownDescription: "Indicates if alerts should be sent to the user's email.",
					Computed:            true,
				},
			},
			"external": superschema.BoolAttribute{
				DataSource: &schemaD.BoolAttribute{
					MarkdownDescription: "Indicates if the user is linked to an external identity provider.",
					Computed:            true,
				},
			},
			"created": superschema.StringAttribute{
				DataSource: &schemaD.StringAttribute{
					MarkdownDescription: "Creation date of the user.",
					Computed:            true,
				},
			},
		},
	}
}
