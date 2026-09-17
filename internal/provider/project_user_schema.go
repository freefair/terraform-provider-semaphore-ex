package provider

import (
	schemaD "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	schemaR "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	superschema "github.com/orange-cloudavenue/terraform-plugin-framework-superschema"
)

type ProjectUserModel struct {
	ProjectID types.Int64  `tfsdk:"project_id"`
	UserID    types.Int64  `tfsdk:"user_id"`
	Role      types.String `tfsdk:"role"`
	RoleID    types.String `tfsdk:"role_id"`
	Revision  types.Int64  `tfsdk:"revision"`
	Username  types.String `tfsdk:"username"`
	Name      types.String `tfsdk:"name"`
}

func ProjectUserSchema() superschema.Schema {
	return superschema.Schema{
		Common: superschema.SchemaDetails{
			MarkdownDescription: "The project user",
		},
		Resource: superschema.SchemaDetails{
			MarkdownDescription: "resource allows you to manage a User's role in a project.",
		},
		DataSource: superschema.SchemaDetails{
			MarkdownDescription: "data source allows you to read a User's role in a project.",
		},
		Attributes: map[string]superschema.Attribute{
			"project_id": superschema.Int64Attribute{
				Common: &schemaR.Int64Attribute{
					MarkdownDescription: "ID of the project.",
					Required:            true,
				},
			},
			"user_id": superschema.Int64Attribute{
				Common: &schemaR.Int64Attribute{
					MarkdownDescription: "The ID of the user.",
					Required:            true,
				},
			},
			"role": superschema.StringAttribute{
				Common: &schemaR.StringAttribute{
					MarkdownDescription: "Role of the user in the project.",
				},
				Resource: &schemaR.StringAttribute{
					Required: true,
				},
				DataSource: &schemaD.StringAttribute{
					Computed: true,
				},
			},
			"role_id": superschema.StringAttribute{
				Common:     &schemaR.StringAttribute{MarkdownDescription: "Server-resolved custom role ID; empty for built-in roles.", Computed: true},
				Resource:   &schemaR.StringAttribute{},
				DataSource: &schemaD.StringAttribute{},
			},
			"revision": superschema.Int64Attribute{
				Common: &schemaR.Int64Attribute{
					MarkdownDescription: "Current membership revision used for optimistic concurrency control.",
					Computed:            true,
				},
				Resource:   &schemaR.Int64Attribute{},
				DataSource: &schemaD.Int64Attribute{},
			},
			"username": superschema.StringAttribute{
				Common: &schemaR.StringAttribute{
					MarkdownDescription: "Username of the user.",
					Computed:            true,
				},
				Resource:   &schemaR.StringAttribute{},
				DataSource: &schemaD.StringAttribute{},
			},
			"name": superschema.StringAttribute{
				Common: &schemaR.StringAttribute{
					MarkdownDescription: "Display name of the user.",
					Computed:            true,
				},
				Resource:   &schemaR.StringAttribute{},
				DataSource: &schemaD.StringAttribute{},
			},
		},
	}
}
