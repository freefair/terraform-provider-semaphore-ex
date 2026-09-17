package provider

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	schemaD "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	schemaR "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	superschema "github.com/orange-cloudavenue/terraform-plugin-framework-superschema"
)

type (
	ProjectKeyModel struct {
		ID              types.Int64                `tfsdk:"id"`
		ProjectID       types.Int64                `tfsdk:"project_id"`
		Name            types.String               `tfsdk:"name"`
		LoginPassword   *ProjectKeyLoginPassword   `tfsdk:"login_password"`
		SSH             *ProjectKeySSH             `tfsdk:"ssh"`
		String          *ProjectKeyString          `tfsdk:"string"`
		RemoteReference *ProjectKeyRemoteReference `tfsdk:"remote_reference"`
		None            *ProjectKeyNone            `tfsdk:"none"`
	}

	ProjectKeyLoginPassword struct {
		Login             types.String `tfsdk:"login"`
		Password          types.String `tfsdk:"password"`
		PasswordWO        types.String `tfsdk:"password_wo"`
		PasswordWOVersion types.Int64  `tfsdk:"password_wo_version"`
	}

	ProjectKeySSH struct {
		Login               types.String `tfsdk:"login"`
		Passphrase          types.String `tfsdk:"passphrase"`
		PassphraseWO        types.String `tfsdk:"passphrase_wo"`
		PassphraseWOVersion types.Int64  `tfsdk:"passphrase_wo_version"`
		PrivateKey          types.String `tfsdk:"private_key"`
		PrivateKeyWO        types.String `tfsdk:"private_key_wo"`
		PrivateKeyWOVersion types.Int64  `tfsdk:"private_key_wo_version"`
	}

	ProjectKeyString struct {
		Value          types.String `tfsdk:"value"`
		ValueWO        types.String `tfsdk:"value_wo"`
		ValueWOVersion types.Int64  `tfsdk:"value_wo_version"`
	}

	// ProjectKeyRemoteReference describes a value-free secret reference. It is
	// deliberately separate from key-type blocks so Terraform can validate that
	// a key is sourced either remotely or from literal secret material.
	ProjectKeyRemoteReference struct {
		StorageType types.String `tfsdk:"storage_type"`
		StorageID   types.Int64  `tfsdk:"storage_id"`
		Mount       types.String `tfsdk:"mount"`
		Path        types.String `tfsdk:"path"`
		Version     types.Int64  `tfsdk:"version"`
		Field       types.String `tfsdk:"field"`
	}

	ProjectKeyNone struct{}
)

const (
	ProjectKeyTypeLoginPassword string = "login_password"
	ProjectKeyTypeSSH           string = "ssh"
	ProjectKeyTypeString        string = "string"
	ProjectKeyTypeNone          string = "none"
)

func (model *ProjectKeyModel) Type() types.String {
	if model.LoginPassword != nil {
		return types.StringValue(ProjectKeyTypeLoginPassword)
	} else if model.SSH != nil {
		return types.StringValue(ProjectKeyTypeSSH)
	} else if model.String != nil {
		return types.StringValue(ProjectKeyTypeString)
	} else if model.None != nil {
		return types.StringValue(ProjectKeyTypeNone)
	}
	return types.StringUnknown()
}

func ProjectKeySchema() superschema.Schema {
	return superschema.Schema{
		Common: superschema.SchemaDetails{
			MarkdownDescription: "The project key",
		},
		Resource: superschema.SchemaDetails{
			MarkdownDescription: "resource allows you to define the credentials used throughout a project. Credentials can be Username/Password, SSH key or None. Project keys are used throughout SemaphoreUI, including Inventories, Repositories, Environments and Templates. When a resource doesn't need credentials, the None Key is used.",
		},
		DataSource: superschema.SchemaDetails{
			MarkdownDescription: "data source allows you to read the the credentials used throughout a project.",
		},
		Attributes: map[string]superschema.Attribute{
			"id": superschema.Int64Attribute{
				Common: &schemaR.Int64Attribute{
					MarkdownDescription: "The key ID.",
				},
				Resource: &schemaR.Int64Attribute{
					Computed:      true,
					PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
				},
				DataSource: &schemaD.Int64Attribute{
					Optional: true,
					Computed: true,
					Validators: []validator.Int64{
						int64validator.ExactlyOneOf(
							path.MatchRoot("id"),
							path.MatchRoot("name"),
						),
					},
				},
			},
			"project_id": superschema.Int64Attribute{
				Common: &schemaR.Int64Attribute{
					MarkdownDescription: "The project ID that the key belongs to.",
					Required:            true,
				},
				Resource: &schemaR.Int64Attribute{
					PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
				},
			},
			"name": superschema.StringAttribute{
				Common: &schemaR.StringAttribute{
					MarkdownDescription: "The display name of the key.",
				},
				Resource: &schemaR.StringAttribute{
					Required: true,
				},
				DataSource: &schemaD.StringAttribute{
					Optional: true,
					Computed: true,
					Validators: []validator.String{
						stringvalidator.ExactlyOneOf(
							path.MatchRoot("id"),
							path.MatchRoot("name"),
						),
					},
				},
			},
			ProjectKeyTypeLoginPassword: superschema.SingleNestedAttribute{
				Common: &schemaR.SingleNestedAttribute{
					MarkdownDescription: "A login password key.",
				},
				Resource: &schemaR.SingleNestedAttribute{
					Optional: true,
				},
				DataSource: &schemaD.SingleNestedAttribute{
					Computed: true,
				},
				Attributes: map[string]superschema.Attribute{
					"login": superschema.StringAttribute{
						Common: &schemaR.StringAttribute{
							MarkdownDescription: "The login username.",
						},
						Resource: &schemaR.StringAttribute{
							Optional: true,
						},
						DataSource: &schemaD.StringAttribute{
							Computed: true,
						},
					},
					"password": superschema.StringAttribute{
						Common: &schemaR.StringAttribute{
							MarkdownDescription: "The login password. Persisted to Terraform state. Set exactly one of `password` or `password_wo`.",
							Sensitive:           true,
						},
						Resource: &schemaR.StringAttribute{
							Optional: true,
						},
						DataSource: &schemaD.StringAttribute{
							Computed: true,
						},
					},
					"password_wo": superschema.StringAttribute{
						Resource: &schemaR.StringAttribute{
							MarkdownDescription: "Write-only variant of `password` — accepts ephemeral values (e.g. from `vault_kv_secret_v2`) and is never persisted to Terraform state. Mutually exclusive with `password`. Bump `password_wo_version` to push a new value to SemaphoreUI.",
							Optional:            true,
							Sensitive:           true,
							WriteOnly:           true,
						},
						DataSource: &schemaD.StringAttribute{
							Computed:  true,
							Sensitive: true,
						},
					},
					"password_wo_version": superschema.Int64Attribute{
						Resource: &schemaR.Int64Attribute{
							MarkdownDescription: "Version trigger for `password_wo`. Increment to instruct the provider to re-read the write-only value and push it to SemaphoreUI. Only meaningful when `password_wo` is set.",
							Optional:            true,
						},
						DataSource: &schemaD.Int64Attribute{
							Computed: true,
						},
					},
				},
			},
			ProjectKeyTypeSSH: superschema.SingleNestedAttribute{
				Common: &schemaR.SingleNestedAttribute{
					MarkdownDescription: "A SSH key.",
				},
				Resource: &schemaR.SingleNestedAttribute{
					Optional: true,
				},
				DataSource: &schemaD.SingleNestedAttribute{
					Computed: true,
				},
				Attributes: map[string]superschema.Attribute{
					"login": superschema.StringAttribute{
						Common: &schemaR.StringAttribute{
							MarkdownDescription: "The login username.",
						},
						Resource: &schemaR.StringAttribute{
							Optional: true,
						},
						DataSource: &schemaD.StringAttribute{
							Computed: true,
						},
					},
					"passphrase": superschema.StringAttribute{
						Common: &schemaR.StringAttribute{
							MarkdownDescription: "The SSH Key passphrase. Persisted to Terraform state. Set at most one of `passphrase` or `passphrase_wo`.",
							Sensitive:           true,
						},
						Resource: &schemaR.StringAttribute{
							Optional: true,
						},
						DataSource: &schemaD.StringAttribute{
							Computed: true,
						},
					},
					"passphrase_wo": superschema.StringAttribute{
						Resource: &schemaR.StringAttribute{
							MarkdownDescription: "Write-only variant of `passphrase` — accepts ephemeral values and is never persisted to Terraform state. Mutually exclusive with `passphrase`. Bump `passphrase_wo_version` to push a new value to SemaphoreUI.",
							Optional:            true,
							Sensitive:           true,
							WriteOnly:           true,
						},
						DataSource: &schemaD.StringAttribute{
							Computed:  true,
							Sensitive: true,
						},
					},
					"passphrase_wo_version": superschema.Int64Attribute{
						Resource: &schemaR.Int64Attribute{
							MarkdownDescription: "Version trigger for `passphrase_wo`. Increment to push a new value to SemaphoreUI.",
							Optional:            true,
						},
						DataSource: &schemaD.Int64Attribute{
							Computed: true,
						},
					},
					"private_key": superschema.StringAttribute{
						Common: &schemaR.StringAttribute{
							MarkdownDescription: "The SSH private key. Persisted to Terraform state. Set exactly one of `private_key` or `private_key_wo`.",
							Sensitive:           true,
						},
						Resource: &schemaR.StringAttribute{
							Optional: true,
						},
						DataSource: &schemaD.StringAttribute{
							Computed: true,
						},
					},
					"private_key_wo": superschema.StringAttribute{
						Resource: &schemaR.StringAttribute{
							MarkdownDescription: "Write-only variant of `private_key` — accepts ephemeral values (e.g. from `vault_kv_secret_v2`) and is never persisted to Terraform state. Mutually exclusive with `private_key`. Bump `private_key_wo_version` to push a new value to SemaphoreUI.",
							Optional:            true,
							Sensitive:           true,
							WriteOnly:           true,
						},
						DataSource: &schemaD.StringAttribute{
							Computed:  true,
							Sensitive: true,
						},
					},
					"private_key_wo_version": superschema.Int64Attribute{
						Resource: &schemaR.Int64Attribute{
							MarkdownDescription: "Version trigger for `private_key_wo`. Increment to instruct the provider to re-read the write-only value and push it to SemaphoreUI. Only meaningful when `private_key_wo` is set.",
							Optional:            true,
						},
						DataSource: &schemaD.Int64Attribute{
							Computed: true,
						},
					},
				},
			},
			ProjectKeyTypeString: superschema.SingleNestedAttribute{
				Common:     &schemaR.SingleNestedAttribute{MarkdownDescription: "A string secret key."},
				Resource:   &schemaR.SingleNestedAttribute{Optional: true},
				DataSource: &schemaD.SingleNestedAttribute{Computed: true},
				Attributes: map[string]superschema.Attribute{
					"value":            superschema.StringAttribute{Common: &schemaR.StringAttribute{MarkdownDescription: "The string secret. Persisted to Terraform state. Set exactly one of `value` or `value_wo`.", Sensitive: true}, Resource: &schemaR.StringAttribute{Optional: true}, DataSource: &schemaD.StringAttribute{Computed: true}},
					"value_wo":         superschema.StringAttribute{Resource: &schemaR.StringAttribute{MarkdownDescription: "Write-only variant of `value`. Bump `value_wo_version` to push a new value.", Optional: true, Sensitive: true, WriteOnly: true}, DataSource: &schemaD.StringAttribute{Computed: true, Sensitive: true}},
					"value_wo_version": superschema.Int64Attribute{Resource: &schemaR.Int64Attribute{MarkdownDescription: "Version trigger for `value_wo`.", Optional: true}, DataSource: &schemaD.Int64Attribute{Computed: true}},
				},
			},
			"remote_reference": superschema.SingleNestedAttribute{
				Common:     &schemaR.SingleNestedAttribute{MarkdownDescription: "External secret reference. Set this instead of literal password, SSH private-key, passphrase, or string value."},
				Resource:   &schemaR.SingleNestedAttribute{Optional: true},
				DataSource: &schemaD.SingleNestedAttribute{Computed: true},
				Attributes: map[string]superschema.Attribute{
					"storage_type": superschema.StringAttribute{Common: &schemaR.StringAttribute{MarkdownDescription: "External storage type.", Validators: []validator.String{stringvalidator.OneOf("vault", "env", "file")}}, Resource: &schemaR.StringAttribute{Required: true}, DataSource: &schemaD.StringAttribute{Computed: true}},
					"storage_id":   superschema.Int64Attribute{Common: &schemaR.Int64Attribute{MarkdownDescription: "Project secret-storage ID. Required for `vault`.", Validators: []validator.Int64{int64validator.AtLeast(1)}}, Resource: &schemaR.Int64Attribute{Optional: true}, DataSource: &schemaD.Int64Attribute{Computed: true}},
					"mount":        superschema.StringAttribute{Common: &schemaR.StringAttribute{MarkdownDescription: "Remote secret mount. Omit to use the storage default; set an empty value to clear an explicit mount."}, Resource: &schemaR.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}, DataSource: &schemaD.StringAttribute{Computed: true}},
					"path":         superschema.StringAttribute{Common: &schemaR.StringAttribute{MarkdownDescription: "Remote secret path or environment/file key."}, Resource: &schemaR.StringAttribute{Required: true}, DataSource: &schemaD.StringAttribute{Computed: true}},
					"version":      superschema.Int64Attribute{Common: &schemaR.Int64Attribute{MarkdownDescription: "Remote secret version; zero uses the storage default.", Validators: []validator.Int64{int64validator.AtLeast(0)}}, Resource: &schemaR.Int64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}}, DataSource: &schemaD.Int64Attribute{Computed: true}},
					"field":        superschema.StringAttribute{Common: &schemaR.StringAttribute{MarkdownDescription: "Field read from the remote secret."}, Resource: &schemaR.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}, DataSource: &schemaD.StringAttribute{Computed: true}},
				},
			},
			ProjectKeyTypeNone: superschema.SingleNestedAttribute{
				Common: &schemaR.SingleNestedAttribute{
					MarkdownDescription: "The special None key.",
				},
				Resource: &schemaR.SingleNestedAttribute{
					Optional: true,
				},
				DataSource: &schemaD.SingleNestedAttribute{
					Computed: true,
				},
			},
		},
	}
}
