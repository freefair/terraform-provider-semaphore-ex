package provider

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	schemaD "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	schemaR "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	superschema "github.com/orange-cloudavenue/terraform-plugin-framework-superschema"
)

type (
	ProjectEnvironmentModel struct {
		VariablesJSON   types.String `tfsdk:"variables_json"`
		EnvironmentJSON types.String `tfsdk:"environment_json"`
		ID              types.Int64  `tfsdk:"id"`
		ProjectID       types.Int64  `tfsdk:"project_id"`
		Name            types.String `tfsdk:"name"`
		Variables       types.Map    `tfsdk:"variables"`
		Environment     types.Map    `tfsdk:"environment"`
		Secrets         types.List   `tfsdk:"secrets"`

		SecretStorage *ProjectEnvironmentSecretStorageModel `tfsdk:"secret_storage"`
		SyncEnabled   types.Bool                            `tfsdk:"sync_enabled"`
		SyncInterval  types.Int64                           `tfsdk:"sync_interval"`
		SyncPaths     types.List                            `tfsdk:"sync_paths"`
	}

	ProjectEnvironmentSecretModel struct {
		ID    types.Int64  `tfsdk:"id"`
		Type  types.String `tfsdk:"type"`
		Name  types.String `tfsdk:"name"`
		Value types.String `tfsdk:"value"`

		StorageID types.Int64  `tfsdk:"storage_id"`
		Mount     types.String `tfsdk:"mount"`
		Path      types.String `tfsdk:"path"`
		Version   types.Int64  `tfsdk:"version"`
		Field     types.String `tfsdk:"field"`
	}

	ProjectEnvironmentSecretStorageModel struct {
		ID        types.Int64  `tfsdk:"id"`
		KeyPrefix types.String `tfsdk:"key_prefix"`
	}

	ProjectEnvironmentSyncPathModel struct {
		ID            types.Int64  `tfsdk:"id"`
		Path          types.String `tfsdk:"path"`
		Prefix        types.String `tfsdk:"prefix"`
		Separator     types.String `tfsdk:"separator"`
		AccessKeyID   types.Int64  `tfsdk:"access_key_id"`
		Mount         types.String `tfsdk:"mount"`
		Field         types.String `tfsdk:"field"`
		RemoteVersion types.Int64  `tfsdk:"remote_version"`
	}
)

func ProjectEnvironmentSchema() superschema.Schema {
	return superschema.Schema{
		Common: superschema.SchemaDetails{
			MarkdownDescription: "The project environment (variable group)",
		},
		Resource: superschema.SchemaDetails{
			MarkdownDescription: "resource allows you to manage a list of extra and environment variables that can be used in a project's templates.",
		},
		DataSource: superschema.SchemaDetails{
			MarkdownDescription: "data source allows you to read project environment details.",
		},
		Attributes: map[string]superschema.Attribute{
			"variables_json":   environmentJSONAttribute("variables", false),
			"environment_json": environmentJSONAttribute("environment", true),
			"id": superschema.Int64Attribute{
				Common: &schemaR.Int64Attribute{
					MarkdownDescription: "The environment ID.",
				},
				Resource: &schemaR.Int64Attribute{
					Computed:      true,
					PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
				},
				DataSource: &schemaD.Int64Attribute{
					Required: true,
				},
			},
			"project_id": superschema.Int64Attribute{
				Common: &schemaR.Int64Attribute{
					MarkdownDescription: "The project ID that the environment belongs to.",
					Required:            true,
				},
				Resource: &schemaR.Int64Attribute{
					PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
				},
			},
			"name": superschema.StringAttribute{
				Common: &schemaR.StringAttribute{
					MarkdownDescription: "The display name of the environment.",
				},
				Resource: &schemaR.StringAttribute{
					Required: true,
				},
				DataSource: &schemaD.StringAttribute{
					Computed: true,
				},
			},
			"variables": superschema.MapAttribute{
				Common: &schemaR.MapAttribute{
					MarkdownDescription: "String-valued extra variables. Omission preserves existing values; configure {} to clear them. Use variables_json for typed or nested values. Passed to Ansible as extra variables (`--extra-vars`) and Terraform/OpenTofu as variables (`-var`).",
					ElementType:         types.StringType,
				},
				Resource: &schemaR.MapAttribute{
					Optional: true,
					Computed: true,
				},
				DataSource: &schemaD.MapAttribute{
					Computed: true,
				},
			},
			"environment": superschema.MapAttribute{
				Common: &schemaR.MapAttribute{
					MarkdownDescription: "String-valued environment variables. Omission preserves existing values; configure {} to clear them. Use environment_json to retain other scalar JSON types.",
					ElementType:         types.StringType,
				},
				Resource: &schemaR.MapAttribute{
					Optional: true,
					Computed: true,
				},
				DataSource: &schemaD.MapAttribute{
					Computed: true,
				},
			},
			"secret_storage": superschema.SingleNestedAttribute{
				Common:     &schemaR.SingleNestedAttribute{MarkdownDescription: "Secret storage used for environment-managed secrets. Omit this block to retain imported settings; configure an empty block to clear the binding."},
				Resource:   &schemaR.SingleNestedAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Object{preserveOptionalObject{}}},
				DataSource: &schemaD.SingleNestedAttribute{Computed: true},
				Attributes: map[string]superschema.Attribute{
					"id":         superschema.Int64Attribute{Common: &schemaR.Int64Attribute{MarkdownDescription: "Secret storage ID."}, Resource: &schemaR.Int64Attribute{Optional: true, Validators: []validator.Int64{int64validator.AtLeast(1)}}, DataSource: &schemaD.Int64Attribute{Computed: true}},
					"key_prefix": superschema.StringAttribute{Common: &schemaR.StringAttribute{MarkdownDescription: "Prefix for keys created in the storage."}, Resource: &schemaR.StringAttribute{Optional: true}, DataSource: &schemaD.StringAttribute{Computed: true}},
				},
			},
			"sync_enabled": superschema.BoolAttribute{
				Common: &schemaR.BoolAttribute{MarkdownDescription: "Whether automatic synchronization of managed secrets is enabled."},
				Resource: &schemaR.BoolAttribute{
					Optional:      true,
					Computed:      true,
					PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
				},
				DataSource: &schemaD.BoolAttribute{Computed: true},
			},
			"sync_interval": superschema.Int64Attribute{
				Common: &schemaR.Int64Attribute{MarkdownDescription: "Automatic synchronization interval in minutes. Set `0` to disable scheduling."},
				Resource: &schemaR.Int64Attribute{
					Optional:      true,
					Computed:      true,
					PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
					Validators:    []validator.Int64{int64validator.AtLeast(0)},
				},
				DataSource: &schemaD.Int64Attribute{Computed: true},
			},
			"sync_paths": superschema.ListNestedAttribute{
				Common: &schemaR.ListNestedAttribute{MarkdownDescription: "Mappings from environment access keys to remote secret-storage targets. Set `[]` to remove all mappings."},
				Resource: &schemaR.ListNestedAttribute{
					Optional:      true,
					Computed:      true,
					PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				},
				DataSource: &schemaD.ListNestedAttribute{Computed: true},
				Attributes: map[string]superschema.Attribute{
					"id":             superschema.Int64Attribute{Common: &schemaR.Int64Attribute{Computed: true}},
					"path":           superschema.StringAttribute{Common: &schemaR.StringAttribute{}, Resource: &schemaR.StringAttribute{Optional: true, Computed: true}, DataSource: &schemaD.StringAttribute{Computed: true}},
					"prefix":         superschema.StringAttribute{Common: &schemaR.StringAttribute{}, Resource: &schemaR.StringAttribute{Optional: true, Computed: true}, DataSource: &schemaD.StringAttribute{Computed: true}},
					"separator":      superschema.StringAttribute{Common: &schemaR.StringAttribute{}, Resource: &schemaR.StringAttribute{Optional: true, Computed: true}, DataSource: &schemaD.StringAttribute{Computed: true}},
					"access_key_id":  superschema.Int64Attribute{Common: &schemaR.Int64Attribute{}, Resource: &schemaR.Int64Attribute{Optional: true, Computed: true, Validators: []validator.Int64{int64validator.AtLeast(1)}}, DataSource: &schemaD.Int64Attribute{Computed: true}},
					"mount":          superschema.StringAttribute{Common: &schemaR.StringAttribute{}, Resource: &schemaR.StringAttribute{Optional: true, Computed: true}, DataSource: &schemaD.StringAttribute{Computed: true}},
					"field":          superschema.StringAttribute{Common: &schemaR.StringAttribute{}, Resource: &schemaR.StringAttribute{Optional: true, Computed: true}, DataSource: &schemaD.StringAttribute{Computed: true}},
					"remote_version": superschema.Int64Attribute{Common: &schemaR.Int64Attribute{Computed: true}},
				},
			},
			"secrets": superschema.ListNestedAttribute{
				Common: &schemaR.ListNestedAttribute{
					MarkdownDescription: "Secret variables of either `\"var\"` or `\"env\"` type. The `value` is encrypted and will be empty if imported.",
				},
				Resource: &schemaR.ListNestedAttribute{
					Optional: true,
				},
				DataSource: &schemaD.ListNestedAttribute{
					Computed: true,
				},
				Attributes: map[string]superschema.Attribute{
					"id": superschema.Int64Attribute{
						Common: &schemaR.Int64Attribute{
							MarkdownDescription: "The variable ID.",
							Computed:            true,
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
						},
					},
					"type": superschema.StringAttribute{
						Common: &schemaR.StringAttribute{
							MarkdownDescription: "The variable type.",
						},
						Resource: &schemaR.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.OneOf("env", "var"),
							},
						},
						DataSource: &schemaD.StringAttribute{
							Computed: true,
						},
					},
					"name": superschema.StringAttribute{
						Common: &schemaR.StringAttribute{
							MarkdownDescription: "The variable name.",
						},
						Resource: &schemaR.StringAttribute{
							Required: true,
						},
						DataSource: &schemaD.StringAttribute{
							Computed: true,
						},
					},
					"value": superschema.StringAttribute{
						Common: &schemaR.StringAttribute{
							MarkdownDescription: "The variable value.",
							Sensitive:           true,
						},
						Resource: &schemaR.StringAttribute{
							Optional: true,
							Computed: true,
						},
						DataSource: &schemaD.StringAttribute{
							Computed: true,
						},
					},
					"storage_id": superschema.Int64Attribute{
						Common:     &schemaR.Int64Attribute{MarkdownDescription: "Remote secret storage ID. Set this instead of `value` to bind an external runtime secret."},
						Resource:   &schemaR.Int64Attribute{Optional: true, Computed: true, Validators: []validator.Int64{int64validator.AtLeast(1)}},
						DataSource: &schemaD.Int64Attribute{Computed: true},
					},
					"mount":   superschema.StringAttribute{Common: &schemaR.StringAttribute{MarkdownDescription: "Remote secret mount; defaults to `secret` when omitted."}, Resource: &schemaR.StringAttribute{Optional: true, Computed: true}, DataSource: &schemaD.StringAttribute{Computed: true}},
					"path":    superschema.StringAttribute{Common: &schemaR.StringAttribute{MarkdownDescription: "Remote secret path."}, Resource: &schemaR.StringAttribute{Optional: true, Computed: true}, DataSource: &schemaD.StringAttribute{Computed: true}},
					"version": superschema.Int64Attribute{Common: &schemaR.Int64Attribute{MarkdownDescription: "Remote secret version; `0` selects the storage default."}, Resource: &schemaR.Int64Attribute{Optional: true, Computed: true, Validators: []validator.Int64{int64validator.AtLeast(0)}}, DataSource: &schemaD.Int64Attribute{Computed: true}},
					"field":   superschema.StringAttribute{Common: &schemaR.StringAttribute{MarkdownDescription: "Field to read from the remote secret."}, Resource: &schemaR.StringAttribute{Optional: true, Computed: true}, DataSource: &schemaD.StringAttribute{Computed: true}},
				},
			},
		},
	}
}
