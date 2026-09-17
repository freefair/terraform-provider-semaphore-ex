package provider

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	schemaD "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	schemaR "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	superschema "github.com/orange-cloudavenue/terraform-plugin-framework-superschema"
)

type ProjectSecretStorageModel struct {
	ID              types.Int64   `tfsdk:"id"`
	ProjectID       types.Int64   `tfsdk:"project_id"`
	Name            types.String  `tfsdk:"name"`
	Type            types.String  `tfsdk:"type"`
	Params          types.Dynamic `tfsdk:"params"`
	Secret          types.String  `tfsdk:"secret"`
	SecretWO        types.String  `tfsdk:"secret_wo"`
	SecretWOVersion types.Int64   `tfsdk:"secret_wo_version"`
	SourceType      types.String  `tfsdk:"source_type"`
	SourceKey       types.String  `tfsdk:"source_key"`
	ReadOnly        types.Bool    `tfsdk:"read_only"`
	SyncEnabled     types.Bool    `tfsdk:"sync_enabled"`
	SyncDirection   types.String  `tfsdk:"sync_direction"`
	SyncInterval    types.Int64   `tfsdk:"sync_interval"`
	SyncRevision    types.Int64   `tfsdk:"sync_revision"`
	SyncPaths       types.List    `tfsdk:"sync_paths"`
}
type ProjectSecretStorageSyncPathModel struct {
	ID            types.Int64  `tfsdk:"id"`
	AccessKeyID   types.Int64  `tfsdk:"access_key_id"`
	Mount         types.String `tfsdk:"mount"`
	Path          types.String `tfsdk:"path"`
	Field         types.String `tfsdk:"field"`
	RemoteVersion types.Int64  `tfsdk:"remote_version"`
}

func ProjectSecretStorageSchema() superschema.Schema {
	return superschema.Schema{Common: superschema.SchemaDetails{MarkdownDescription: "Manages project secret storage and synchronization metadata without returning stored credential material."}, Attributes: map[string]superschema.Attribute{
		"id":                superschema.Int64Attribute{Common: &schemaR.Int64Attribute{}, Resource: &schemaR.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}}, DataSource: &schemaD.Int64Attribute{Required: true}},
		"project_id":        superschema.Int64Attribute{Common: &schemaR.Int64Attribute{Required: true}, Resource: &schemaR.Int64Attribute{PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}}, DataSource: &schemaD.Int64Attribute{Required: true}},
		"name":              superschema.StringAttribute{Common: &schemaR.StringAttribute{}, Resource: &schemaR.StringAttribute{Required: true}, DataSource: &schemaD.StringAttribute{Computed: true}},
		"type":              superschema.StringAttribute{Common: &schemaR.StringAttribute{Validators: []validator.String{stringvalidator.OneOf("local", "vault", "openbao", "dvls", "aws_sm", "azure_kv")}}, Resource: &schemaR.StringAttribute{Required: true}, DataSource: &schemaD.StringAttribute{Computed: true}},
		"params":            superschema.MapAttribute{Common: &schemaR.MapAttribute{ElementType: types.StringType}, Resource: &schemaR.MapAttribute{Optional: true}, DataSource: &schemaD.MapAttribute{Computed: true}},
		"secret":            superschema.StringAttribute{Common: &schemaR.StringAttribute{Sensitive: true}, Resource: &schemaR.StringAttribute{Optional: true, Sensitive: true}, DataSource: &schemaD.StringAttribute{Computed: true, Sensitive: true}},
		"secret_wo":         superschema.StringAttribute{Resource: &schemaR.StringAttribute{Optional: true, Sensitive: true, WriteOnly: true}, DataSource: &schemaD.StringAttribute{Computed: true, Sensitive: true}},
		"secret_wo_version": superschema.Int64Attribute{Resource: &schemaR.Int64Attribute{Optional: true}, DataSource: &schemaD.Int64Attribute{Computed: true}},
		"source_type":       superschema.StringAttribute{Common: &schemaR.StringAttribute{Validators: []validator.String{stringvalidator.OneOf("env", "file")}}, Resource: &schemaR.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}, DataSource: &schemaD.StringAttribute{Computed: true}},
		"source_key":        superschema.StringAttribute{Common: &schemaR.StringAttribute{Sensitive: true}, Resource: &schemaR.StringAttribute{Optional: true, Sensitive: true}, DataSource: &schemaD.StringAttribute{Computed: true, Sensitive: true}},
		"read_only":         superschema.BoolAttribute{Common: &schemaR.BoolAttribute{Computed: true}, Resource: &schemaR.BoolAttribute{Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}}, DataSource: &schemaD.BoolAttribute{Computed: true}},
		"sync_enabled":      superschema.BoolAttribute{Common: &schemaR.BoolAttribute{}, Resource: &schemaR.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}}, DataSource: &schemaD.BoolAttribute{Computed: true}},
		"sync_direction":    superschema.StringAttribute{Common: &schemaR.StringAttribute{Validators: []validator.String{stringvalidator.OneOf("read_only", "outbound")}}, Resource: &schemaR.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}, DataSource: &schemaD.StringAttribute{Computed: true}},
		"sync_interval":     superschema.Int64Attribute{Common: &schemaR.Int64Attribute{Validators: []validator.Int64{int64validator.AtLeast(0)}}, Resource: &schemaR.Int64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}}, DataSource: &schemaD.Int64Attribute{Computed: true}},
		"sync_revision":     superschema.Int64Attribute{Common: &schemaR.Int64Attribute{Computed: true}, Resource: &schemaR.Int64Attribute{Computed: true}, DataSource: &schemaD.Int64Attribute{Computed: true}},
		"sync_paths": superschema.ListNestedAttribute{Common: &schemaR.ListNestedAttribute{}, Resource: &schemaR.ListNestedAttribute{Optional: true, Computed: true}, DataSource: &schemaD.ListNestedAttribute{Computed: true}, Attributes: map[string]superschema.Attribute{
			"id": superschema.Int64Attribute{Common: &schemaR.Int64Attribute{Computed: true}}, "access_key_id": superschema.Int64Attribute{Common: &schemaR.Int64Attribute{}, Resource: &schemaR.Int64Attribute{Optional: true, Computed: true, Validators: []validator.Int64{int64validator.AtLeast(1)}}, DataSource: &schemaD.Int64Attribute{Computed: true}}, "mount": superschema.StringAttribute{Common: &schemaR.StringAttribute{}, Resource: &schemaR.StringAttribute{Optional: true, Computed: true}, DataSource: &schemaD.StringAttribute{Computed: true}}, "path": superschema.StringAttribute{Common: &schemaR.StringAttribute{}, Resource: &schemaR.StringAttribute{Optional: true, Computed: true}, DataSource: &schemaD.StringAttribute{Computed: true}}, "field": superschema.StringAttribute{Common: &schemaR.StringAttribute{}, Resource: &schemaR.StringAttribute{Optional: true, Computed: true}, DataSource: &schemaD.StringAttribute{Computed: true}}, "remote_version": superschema.Int64Attribute{Common: &schemaR.Int64Attribute{Computed: true}},
		}},
	}}
}
