package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	rs "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	superschema "github.com/orange-cloudavenue/terraform-plugin-framework-superschema"
	"strings"
)

type projectTaskGroupModel struct {
	ID               types.Int64  `tfsdk:"id"`
	ProjectID        types.Int64  `tfsdk:"project_id"`
	OwnerProjectID   types.Int64  `tfsdk:"owner_project_id"`
	Name             types.String `tfsdk:"name"`
	Description      types.String `tfsdk:"description"`
	MaxParallelTasks types.Int64  `tfsdk:"max_parallel_tasks"`
	RunnerIDs        types.Set    `tfsdk:"runner_ids"`
	SharedProjectIDs types.Set    `tfsdk:"shared_project_ids"`
	Revision         types.Int64  `tfsdk:"revision"`
}

func projectTaskGroupSchema() superschema.Schema {
	name := exRecordString("Display name.")
	name.Resource.Validators = []validator.String{taskGroupNameValidator{}}
	description := exRecordDefaultString("Description.", "")
	description.Resource.Default = nil
	description.Resource.PlanModifiers = []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	description.Resource.Validators = []validator.String{stringvalidator.LengthAtMost(2048)}
	ids := func(description string) superschema.SetAttribute {
		return superschema.SetAttribute{
			Common:     &rs.SetAttribute{MarkdownDescription: description, ElementType: types.Int64Type},
			Resource:   &rs.SetAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()}, Validators: []validator.Set{setvalidator.ValueInt64sAre(int64validator.AtLeast(1))}},
			DataSource: &ds.SetAttribute{Computed: true},
		}
	}
	return superschema.Schema{Common: superschema.SchemaDetails{MarkdownDescription: "Manages a project task group in Semaphore EX v2.20.0-ex.2.1.1 or later. All groups selected by a template apply; admission reserves all capacity atomically. Updates and deletes use the last observed revision and never retry concurrent edits."}, DataSource: superschema.SchemaDetails{MarkdownDescription: "Reads an owned or explicitly shared task group in Semaphore EX v2.20.0-ex.2.1.1 or later. The lookup project and owning project are reported separately; receiving projects do not see the full grant list."}, Attributes: map[string]superschema.Attribute{
		"id":               exRecordID("Task group ID."),
		"project_id":       exRecordParent("Owner project for a resource; project through which a data source reads the group (including an explicit project grant)."),
		"owner_project_id": superschema.Int64Attribute{Common: &rs.Int64Attribute{Computed: true, MarkdownDescription: "Project that owns the group and authorizes mutations."}},
		"name":             name, "description": description,
		"max_parallel_tasks": superschema.Int64Attribute{Common: &rs.Int64Attribute{MarkdownDescription: "Maximum concurrent executions across every template and project using this group. Defaults to 1 on create; omission preserves existing limits."}, Resource: &rs.Int64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}, Validators: []validator.Int64{int64validator.Between(1, 1000)}}, DataSource: &ds.Int64Attribute{Computed: true}},
		"runner_ids":         ids("Allowed runner IDs. Omission preserves existing restrictions. Empty adds no runner restriction. Several groups intersect their runner restrictions."),
		"shared_project_ids": ids("Projects allowed to select this group. Omission preserves existing grants; empty removes them. The owner project is implicit. A data source reading through a receiving project returns null because the API hides the full grant list."),
		"revision":           superschema.Int64Attribute{Common: &rs.Int64Attribute{Computed: true, MarkdownDescription: "Server revision used to reject concurrent modifications."}, Resource: &rs.Int64Attribute{PlanModifiers: []planmodifier.Int64{}}},
	}}
}

// Names must already be canonical because Terraform cannot accept the server
// trimming a configured value after apply.
type taskGroupNameValidator struct{}

func (taskGroupNameValidator) Description(context.Context) string {
	return "name must contain 1–128 bytes without surrounding whitespace, line breaks or NUL"
}
func (v taskGroupNameValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (v taskGroupNameValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	name := req.ConfigValue.ValueString()
	if name == "" || len(name) > 128 || strings.TrimSpace(name) != name || strings.ContainsAny(name, "\r\n\x00") {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Task Group Name", v.Description(ctx))
	}
}
