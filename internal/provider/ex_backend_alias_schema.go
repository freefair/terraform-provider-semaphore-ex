package provider

import (
	schemaD "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	schemaR "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	superschema "github.com/orange-cloudavenue/terraform-plugin-framework-superschema"
)

func ProjectTerraformBackendAliasSchema() superschema.Schema {
	return superschema.Schema{Common: superschema.SchemaDetails{MarkdownDescription: "Manage an alias and Basic-auth credential binding for one encrypted Terraform HTTP backend inventory."}, Attributes: map[string]superschema.Attribute{
		"id":           superschema.StringAttribute{Common: &schemaR.StringAttribute{MarkdownDescription: "Opaque alias identifier assigned by Semaphore EX."}, Resource: &schemaR.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}, DataSource: &schemaD.StringAttribute{Required: true}},
		"project_id":   exRecordParent("Project containing the Terraform workspace."),
		"inventory_id": exRecordParent("Terraform, OpenTofu, or Terragrunt workspace inventory."),
		"auth_key_id":  superschema.Int64Attribute{Common: &schemaR.Int64Attribute{MarkdownDescription: "Project login-password key used as the HTTP backend Basic-auth credential."}, Resource: &schemaR.Int64Attribute{Required: true}, DataSource: &schemaD.Int64Attribute{Computed: true}},
		"url":          superschema.StringAttribute{Common: &schemaR.StringAttribute{MarkdownDescription: "HTTP backend address for this alias. Configure it as TF_HTTP_ADDRESS, TF_HTTP_LOCK_ADDRESS, and TF_HTTP_UNLOCK_ADDRESS."}, Resource: &schemaR.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}, DataSource: &schemaD.StringAttribute{Computed: true}},
	}}
}

func projectTerraformBackendAliasSpec() exRecordSpec {
	return exRecordSpec{name: "project_terraform_backend_alias", collection: "/project/{project_id}/inventory/{inventory_id}/terraform/aliases", item: "/project/{project_id}/inventory/{inventory_id}/terraform/aliases/{alias_id}", idParameter: "alias_id", scopes: map[string]string{"project_id": "project_id", "inventory_id": "inventory_id"}, bodyFields: []string{"auth_key_id"}, importLabels: []string{"project", "inventory", "alias"}, importAttributes: []string{"project_id", "inventory_id", "id"}, schema: ProjectTerraformBackendAliasSchema()}
}
