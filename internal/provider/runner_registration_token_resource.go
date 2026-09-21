package provider

import (
	"context"
	"fmt"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client/runner"
	"github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/models"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// runnerRegistrationTokenID builds the synthetic resource ID from the owning
// project (nil for global runners) and the runner ID.
func runnerRegistrationTokenID(projectID *int64, runnerID int64) string {
	if projectID != nil {
		return fmt.Sprintf("project/%d/runner/%d", *projectID, runnerID)
	}
	return fmt.Sprintf("runner/%d", runnerID)
}

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &runnerRegistrationTokenResource{}
	_ resource.ResourceWithConfigure = &runnerRegistrationTokenResource{}
)

func NewRunnerRegistrationTokenResource() resource.Resource {
	return &runnerRegistrationTokenResource{}
}

type runnerRegistrationTokenResource struct {
	client *apiclient.SemaphoreUI
}

func (r *runnerRegistrationTokenResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			"Expected *client.SemaphoreUI, got %T. Please report this issue to the provider developers.",
		)
		return
	}
	r.client = client
}

func (r *runnerRegistrationTokenResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runner_registration_token"
}

func (r *runnerRegistrationTokenResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = RunnerRegistrationTokenSchema().GetResource(ctx)
}

// isProject reports whether the model targets a project runner.
func (m RunnerRegistrationTokenModel) isProject() bool {
	return !m.ProjectID.IsNull() && !m.ProjectID.IsUnknown()
}

func (r *runnerRegistrationTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan RunnerRegistrationTokenModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var payload *models.RunnerRegistrationToken
	if plan.isProject() {
		response, err := r.client.Runner.PostProjectProjectIDRunnersRunnerIDRegistrationToken(&runner.PostProjectProjectIDRunnersRunnerIDRegistrationTokenParams{
			ProjectID: plan.ProjectID.ValueInt64(),
			RunnerID:  plan.RunnerID.ValueInt64(),
		}, nil)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Generating SemaphoreUI Runner Registration Token",
				"Could not generate registration token, unexpected error: "+err.Error(),
			)
			return
		}
		payload = response.Payload
	} else {
		response, err := r.client.Runner.PostRunnersRunnerIDRegistrationToken(&runner.PostRunnersRunnerIDRegistrationTokenParams{
			RunnerID: plan.RunnerID.ValueInt64(),
		}, nil)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Generating SemaphoreUI Runner Registration Token",
				"Could not generate registration token, unexpected error: "+err.Error(),
			)
			return
		}
		payload = response.Payload
	}

	// Populate identifiers from the response: runner_id is read-only and
	// project_id is null for global runners.
	plan.RunnerID = types.Int64Value(payload.RunnerID)
	plan.ProjectID = types.Int64PointerValue(payload.ProjectID)
	plan.RegistrationToken = types.StringValue(payload.RegistrationToken)
	plan.ID = types.StringValue(runnerRegistrationTokenID(payload.ProjectID, payload.RunnerID))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read keeps the stored token (the API never returns it again) but verifies the
// underlying runner still exists. If the runner was deleted out-of-band the
// token is meaningless, so the resource is removed from state.
func (r *runnerRegistrationTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state RunnerRegistrationTokenModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.isProject() {
		_, err := r.client.Runner.GetProjectProjectIDRunnersRunnerID(&runner.GetProjectProjectIDRunnersRunnerIDParams{
			ProjectID: state.ProjectID.ValueInt64(),
			RunnerID:  state.RunnerID.ValueInt64(),
		}, nil)
		if err != nil {
			if resourceNotFound(err) {
				resp.State.RemoveResource(ctx)
				return
			}
			resp.Diagnostics.AddError(
				"Error Reading SemaphoreUI Project Runner",
				"Could not read project runner, unexpected error: "+err.Error(),
			)
			return
		}
	} else {
		_, err := r.client.Runner.GetRunnersRunnerID(&runner.GetRunnersRunnerIDParams{
			RunnerID: state.RunnerID.ValueInt64(),
		}, nil)
		if err != nil {
			if resourceNotFound(err) {
				resp.State.RemoveResource(ctx)
				return
			}
			resp.Diagnostics.AddError(
				"Error Reading SemaphoreUI Runner",
				"Could not read runner, unexpected error: "+err.Error(),
			)
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is never reached: every settable attribute forces replacement, so a
// new token is always generated via Create. It is implemented defensively to
// preserve the computed values if it ever runs.
func (r *runnerRegistrationTokenResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state RunnerRegistrationTokenModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	plan.RegistrationToken = state.RegistrationToken
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is a no-op: the registration token is one-time and short-lived, and
// the API has no endpoint to revoke it. Removing the resource simply drops the
// token from Terraform state.
func (r *runnerRegistrationTokenResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// ImportState adopts the runner association without issuing or rotating a token.
// The one-time secret is unavailable after creation and remains null on import.
func (r *runnerRegistrationTokenResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	fields, err := parseImportFields(req.ID, []string{"runner"}, []string{"project", "runner"})
	if err != nil {
		resp.Diagnostics.AddError("Invalid Runner Registration Token Import ID", err.Error())
		return
	}
	state := RunnerRegistrationTokenModel{RunnerID: types.Int64Value(fields["runner"]), ProjectID: types.Int64Null(), Keepers: types.MapNull(types.StringType), RegistrationToken: types.StringNull()}
	var projectID *int64
	if id, ok := fields["project"]; ok {
		projectID = &id
		state.ProjectID = types.Int64Value(id)
	}
	state.ID = types.StringValue(runnerRegistrationTokenID(projectID, fields["runner"]))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	resp.Diagnostics.AddWarning("One-Time Registration Token Unavailable", "Import adopts the runner association only. The API cannot return the original registration token; registration_token remains null. Import does not generate or invalidate a token. Use an explicit replacement to request a new token for an unregistered runner.")
}
