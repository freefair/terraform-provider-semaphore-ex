package provider

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	schemaD "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	schemaR "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	superschema "github.com/orange-cloudavenue/terraform-plugin-framework-superschema"
	"regexp"
)

func AppSchema() superschema.Schema {
	return superschema.Schema{Common: superschema.SchemaDetails{MarkdownDescription: "Manages a custom task application. The executable path and arguments describe the command installed on runners; configuring an app does not execute it."}, Attributes: map[string]superschema.Attribute{
		"id":         superschema.StringAttribute{Common: &schemaR.StringAttribute{Required: true, MarkdownDescription: "Stable custom application identifier.", Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[A-Za-z0-9_]+$`), "Use letters, digits and underscores, as required by the server option keys.")}}, Resource: &schemaR.StringAttribute{PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}}},
		"title":      exRecordString("Display title."),
		"path":       exRecordString("Executable path on the runner."),
		"icon":       exRecordDefaultString("Icon identifier.", ""),
		"color":      exRecordDefaultString("Light-theme icon color.", ""),
		"dark_color": exRecordDefaultString("Dark-theme icon color.", ""),
		"active":     superschema.BoolAttribute{Common: &schemaR.BoolAttribute{MarkdownDescription: "Makes the app available for task templates."}, Resource: &schemaR.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)}, DataSource: &schemaD.BoolAttribute{Computed: true}},
		"priority":   superschema.Int64Attribute{Common: &schemaR.Int64Attribute{MarkdownDescription: "Display ordering priority."}, Resource: &schemaR.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(0)}, DataSource: &schemaD.Int64Attribute{Computed: true}},
		"args":       superschema.ListAttribute{Common: &schemaR.ListAttribute{MarkdownDescription: "Command arguments supplied to the application.", ElementType: types.StringType}, Resource: &schemaR.ListAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}}, DataSource: &schemaD.ListAttribute{Computed: true}},
	}}
}
func appSpec() exRecordSpec {
	return exRecordSpec{name: "app", collection: "/apps/{app_id}", item: "/apps/{app_id}", idParameter: "app_id", scopes: map[string]string{"app_id": "id"}, bodyFields: []string{"title", "path", "icon", "color", "dark_color", "active", "priority", "args"}, importStringID: true, schema: AppSchema()}
}
