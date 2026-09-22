package provider

import (
	"context"
	"fmt"
	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"net/http"
	"strconv"
)

type projectTaskGroupResource struct{ client *apiclient.SemaphoreUI }

func NewProjectTaskGroupResource() resource.Resource { return &projectTaskGroupResource{} }
func (r *projectTaskGroupResource) Metadata(_ context.Context, q resource.MetadataRequest, p *resource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_project_task_group"
}
func (r *projectTaskGroupResource) Schema(ctx context.Context, _ resource.SchemaRequest, p *resource.SchemaResponse) {
	p.Schema = projectTaskGroupSchema().GetResource(ctx)
}
func (r *projectTaskGroupResource) Configure(_ context.Context, q resource.ConfigureRequest, p *resource.ConfigureResponse) {
	if q.ProviderData == nil {
		return
	}
	client, ok := q.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		p.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.SemaphoreUI, got %T", q.ProviderData))
		return
	}
	r.client = client
}
func taskGroupParams(m projectTaskGroupModel) map[string]string {
	return map[string]string{"project_id": strconv.FormatInt(m.ProjectID.ValueInt64(), 10), "group_id": strconv.FormatInt(m.ID.ValueInt64(), 10)}
}
func taskGroupBody(ctx context.Context, m projectTaskGroupModel, revision bool) (map[string]any, error) {
	runners := []int64{}
	projects := []int64{}
	for _, selection := range []struct {
		value  types.Set
		target *[]int64
	}{{m.RunnerIDs, &runners}, {m.SharedProjectIDs, &projects}} {
		if !selection.value.IsNull() && !selection.value.IsUnknown() {
			if d := selection.value.ElementsAs(ctx, selection.target, false); d.HasError() {
				return nil, fmt.Errorf("invalid group identifiers: %s", d)
			}
		} else if revision {
			return nil, fmt.Errorf("refresh the group before updating unavailable policy selections")
		}
	}
	limit := m.MaxParallelTasks.ValueInt64()
	if m.MaxParallelTasks.IsNull() || m.MaxParallelTasks.IsUnknown() {
		if revision {
			return nil, fmt.Errorf("refresh the group before updating its unavailable limit")
		}
		limit = 1
	}
	body := map[string]any{"name": m.Name.ValueString(), "description": m.Description.ValueString(), "max_parallel_tasks": limit, "runner_ids": runners, "shared_project_ids": projects}
	if revision {
		if err := taskGroupRevision(m); err != nil {
			return nil, err
		}
		body["revision"] = m.Revision.ValueInt64()
	}
	return body, nil
}
func taskGroupRevision(m projectTaskGroupModel) error {
	if m.Revision.IsNull() || m.Revision.IsUnknown() || m.Revision.ValueInt64() < 1 {
		return fmt.Errorf("refresh the group before modifying it; its revision is unavailable")
	}
	return nil
}

type taskGroupResponse struct {
	ID               int64   `json:"id"`
	ProjectID        int64   `json:"project_id"`
	Name             string  `json:"name"`
	Description      string  `json:"description"`
	MaxParallelTasks int64   `json:"max_parallel_tasks"`
	RunnerIDs        []int64 `json:"runner_ids"`
	SharedProjectIDs []int64 `json:"shared_project_ids"`
	Revision         int64   `json:"revision"`
}

func taskGroupFromResponse(ctx context.Context, old projectTaskGroupModel, raw taskGroupResponse) (projectTaskGroupModel, error) {
	if raw.ID < 1 || raw.ProjectID < 1 || raw.Revision < 1 {
		return old, fmt.Errorf("server returned an invalid task group identity or revision")
	}
	if !old.ID.IsNull() && !old.ID.IsUnknown() && old.ID.ValueInt64() != raw.ID {
		return old, fmt.Errorf("server returned a different task group")
	}
	next := old
	next.ID = types.Int64Value(raw.ID)
	next.OwnerProjectID = types.Int64Value(raw.ProjectID)
	next.Name = types.StringValue(raw.Name)
	next.Description = types.StringValue(raw.Description)
	next.MaxParallelTasks = types.Int64Value(raw.MaxParallelTasks)
	next.Revision = types.Int64Value(raw.Revision)
	runners, d := types.SetValueFrom(ctx, types.Int64Type, raw.RunnerIDs)
	if d.HasError() {
		return old, fmt.Errorf("invalid runner response: %s", d)
	}
	projects, d := types.SetValueFrom(ctx, types.Int64Type, raw.SharedProjectIDs)
	if d.HasError() {
		return old, fmt.Errorf("invalid grant response: %s", d)
	}
	if runners.IsNull() {
		runners = types.SetValueMust(types.Int64Type, nil)
	}
	if projects.IsNull() {
		projects = types.SetValueMust(types.Int64Type, nil)
	}
	next.RunnerIDs = runners
	next.SharedProjectIDs = projects
	return next, nil
}
func readTaskGroup(ctx context.Context, client *apiclient.SemaphoreUI, m projectTaskGroupModel) (projectTaskGroupModel, error) {
	var raw taskGroupResponse
	if err := exRequest(ctx, client, http.MethodGet, "/project/{project_id}/task_groups/{group_id}", taskGroupParams(m), nil, &raw); err != nil {
		return m, err
	}
	return taskGroupFromResponse(ctx, m, raw)
}
func (r *projectTaskGroupResource) Create(ctx context.Context, q resource.CreateRequest, p *resource.CreateResponse) {
	var m projectTaskGroupModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &m)...)
	if p.Diagnostics.HasError() {
		return
	}
	body, err := taskGroupBody(ctx, m, false)
	var raw taskGroupResponse
	if err == nil {
		err = exRequest(ctx, r.client, http.MethodPost, "/project/{project_id}/task_groups", taskGroupParams(m), body, &raw)
	}
	if err == nil {
		m, err = taskGroupFromResponse(ctx, m, raw)
	}
	if err != nil {
		p.Diagnostics.AddError("Error Creating Task Group", err.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &m)...)
}
func (r *projectTaskGroupResource) Read(ctx context.Context, q resource.ReadRequest, p *resource.ReadResponse) {
	var m projectTaskGroupModel
	p.Diagnostics.Append(q.State.Get(ctx, &m)...)
	if p.Diagnostics.HasError() {
		return
	}
	m, err := readTaskGroup(ctx, r.client, m)
	if resourceNotFound(err) {
		p.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		p.Diagnostics.AddError("Error Reading Task Group", err.Error())
		return
	}
	if !m.ProjectID.Equal(m.OwnerProjectID) {
		p.Diagnostics.AddError("Task Group Ownership Mismatch", "Manage or import this group through its owner project.")
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &m)...)
}
func (r *projectTaskGroupResource) Update(ctx context.Context, q resource.UpdateRequest, p *resource.UpdateResponse) {
	var m, old projectTaskGroupModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &m)...)
	p.Diagnostics.Append(q.State.Get(ctx, &old)...)
	if p.Diagnostics.HasError() {
		return
	}
	m.ID = old.ID
	m.Revision = old.Revision
	body, err := taskGroupBody(ctx, m, true)
	if err == nil {
		err = exRequest(ctx, r.client, http.MethodPut, "/project/{project_id}/task_groups/{group_id}", taskGroupParams(m), body, nil)
	}
	if err == nil {
		m, err = readTaskGroup(ctx, r.client, m)
	}
	if err != nil {
		p.Diagnostics.AddError("Error Updating Task Group", err.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &m)...)
}
func (r *projectTaskGroupResource) Delete(ctx context.Context, q resource.DeleteRequest, p *resource.DeleteResponse) {
	var m projectTaskGroupModel
	p.Diagnostics.Append(q.State.Get(ctx, &m)...)
	if p.Diagnostics.HasError() {
		return
	}
	err := taskGroupRevision(m)
	if err == nil {
		err = exRequest(ctx, r.client, http.MethodDelete, "/project/{project_id}/task_groups/{group_id}", taskGroupParams(m), map[string]any{"revision": m.Revision.ValueInt64()}, nil)
	}
	if err != nil && !resourceNotFound(err) {
		p.Diagnostics.AddError("Error Deleting Task Group", err.Error())
	}
}
func (r *projectTaskGroupResource) ImportState(ctx context.Context, q resource.ImportStateRequest, p *resource.ImportStateResponse) {
	fields, err := parseImportFields(q.ID, []string{"project", "task_group"})
	if err != nil {
		p.Diagnostics.AddError("Invalid Import ID", err.Error())
		return
	}
	p.Diagnostics.Append(p.State.SetAttribute(ctx, path.Root("id"), fields["task_group"])...)
	p.Diagnostics.Append(p.State.SetAttribute(ctx, path.Root("project_id"), fields["project"])...)
}

var _ resource.ResourceWithImportState = &projectTaskGroupResource{}
