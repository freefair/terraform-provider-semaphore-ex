package provider

import (
	"github.com/freefair/terraform-provider-semaphore-ex/internal/stringvalidator"
	schemaD "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	schemaR "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	superschema "github.com/orange-cloudavenue/terraform-plugin-framework-superschema"
)

type (
	ProjectScheduleModel struct {
		Type           types.String     `tfsdk:"type"`
		RunAt          types.String     `tfsdk:"run_at"`
		DeleteAfterRun types.Bool       `tfsdk:"delete_after_run"`
		RepositoryID   types.Int64      `tfsdk:"repository_id"`
		TaskParams     *TaskParamsModel `tfsdk:"task_params"`
		ID             types.Int64      `tfsdk:"id"`
		ProjectID      types.Int64      `tfsdk:"project_id"`
		TemplateID     types.Int64      `tfsdk:"template_id"`
		Name           types.String     `tfsdk:"name"`
		CronFormat     types.String     `tfsdk:"cron_format"`
		Timezone       types.String     `tfsdk:"timezone"`
		Enabled        types.Bool       `tfsdk:"enabled"`
	}
)

func ProjectScheduleSchema() superschema.Schema {
	return superschema.Schema{
		Common: superschema.SchemaDetails{
			MarkdownDescription: "The project schedule",
		},
		Resource: superschema.SchemaDetails{
			MarkdownDescription: "resource allows you to schedule the execution of templates in a project.",
		},
		DataSource: superschema.SchemaDetails{
			MarkdownDescription: "data source allows you to read a project schedule",
		},
		Attributes: map[string]superschema.Attribute{
			"type": superschema.StringAttribute{
				Common: &schemaR.StringAttribute{MarkdownDescription: "Schedule kind: cron or run_at. Inferred from cron_format/run_at when omitted."}, Resource: &schemaR.StringAttribute{Optional: true, Computed: true}, DataSource: &schemaD.StringAttribute{Computed: true},
			},
			"run_at":           superschema.StringAttribute{Common: &schemaR.StringAttribute{MarkdownDescription: "One-off execution time in RFC3339 format."}, Resource: &schemaR.StringAttribute{Optional: true}, DataSource: &schemaD.StringAttribute{Computed: true}},
			"delete_after_run": preservedBoolAttribute("Remove a one-off schedule after it executes. Before the next apply, remove the completed schedule from configuration or choose a new future run_at; the server rejects creating a schedule in the past."),
			"repository_id":    superschema.Int64Attribute{Common: &schemaR.Int64Attribute{MarkdownDescription: "Optional repository used to detect source changes."}, Resource: &schemaR.Int64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}}, DataSource: &schemaD.Int64Attribute{Computed: true}},
			"task_params":      TaskParamsAttribute(),
			"id": superschema.Int64Attribute{
				Common: &schemaR.Int64Attribute{
					MarkdownDescription: "The schedule ID.",
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
					MarkdownDescription: "The project ID that the schedule belongs to.",
					Required:            true,
				},
				Resource: &schemaR.Int64Attribute{
					PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
				},
			},
			"template_id": superschema.Int64Attribute{
				Common: &schemaR.Int64Attribute{
					MarkdownDescription: "The template ID that the schedule executes.",
				},
				Resource: &schemaR.Int64Attribute{
					Required:      true,
					PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
				},
				DataSource: &schemaD.Int64Attribute{
					Computed: true,
				},
			},
			"name": superschema.StringAttribute{
				Common: &schemaR.StringAttribute{
					MarkdownDescription: "The display name of the schedule.",
				},
				Resource: &schemaR.StringAttribute{
					Required: true,
				},
				DataSource: &schemaD.StringAttribute{
					Computed: true,
				},
			},
			"cron_format": superschema.StringAttribute{
				Common: &schemaR.StringAttribute{
					MarkdownDescription: "The cron format of the schedule.",
				},
				Resource: &schemaR.StringAttribute{
					Optional: true,
					Validators: []validator.String{
						stringvalidator.CronFormat(),
					},
				},
				DataSource: &schemaD.StringAttribute{
					Computed: true,
				},
			},
			"timezone": superschema.StringAttribute{
				Common:     &schemaR.StringAttribute{MarkdownDescription: "IANA timezone. Empty uses the server default. Existing imported values are preserved when omitted."},
				Resource:   &schemaR.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
				DataSource: &schemaD.StringAttribute{Computed: true},
			},
			"enabled": superschema.BoolAttribute{
				Common: &schemaR.BoolAttribute{
					MarkdownDescription: "Whether the schedule is enabled.",
				},
				Resource: &schemaR.BoolAttribute{
					Required: true,
				},
				DataSource: &schemaD.BoolAttribute{
					Computed: true,
				},
			},
		},
	}
}
