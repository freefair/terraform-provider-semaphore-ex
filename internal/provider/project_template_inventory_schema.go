package provider

import (
	schemaD "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	schemaR "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	superschema "github.com/orange-cloudavenue/terraform-plugin-framework-superschema"
)

type projectTemplateInventoryModel struct {
	ID          types.String `tfsdk:"id"`
	ProjectID   types.Int64  `tfsdk:"project_id"`
	TemplateID  types.Int64  `tfsdk:"template_id"`
	InventoryID types.Int64  `tfsdk:"inventory_id"`
}

func ProjectTemplateInventorySchema() superschema.Schema {
	return superschema.Schema{Common: superschema.SchemaDetails{MarkdownDescription: "Attaches an additional inventory to a template. The template's inventory_id selects its default inventory. Destroy detaches this association and preserves the inventory itself."}, Attributes: map[string]superschema.Attribute{
		"id":           superschema.StringAttribute{Common: &schemaR.StringAttribute{MarkdownDescription: "Compound association identity."}, Resource: &schemaR.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}, DataSource: &schemaD.StringAttribute{Computed: true}},
		"project_id":   exRecordParent("Project containing the template and inventory."),
		"template_id":  exRecordParent("Template receiving the inventory."),
		"inventory_id": exRecordParent("Inventory to attach. An inventory can belong to only one template."),
	}}
}
