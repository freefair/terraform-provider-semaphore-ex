package provider

import (
	"context"
	"fmt"
	"net/http"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Retention publication is append-only by design. The API has no destructive
// reset endpoint, so these data sources expose the effective, revision-proven
// retention contract without pretending Terraform can delete history.
type exWorkflowArtifactRetentionModel struct {
	ID               types.String `tfsdk:"id"`
	ProjectID        types.Int64  `tfsdk:"project_id"`
	GlobalPolicy     types.Object `tfsdk:"global_policy"`
	ProjectPolicy    types.Object `tfsdk:"project_policy"`
	GlobalRevision   types.Int64  `tfsdk:"global_revision"`
	ProjectRevision  types.Int64  `tfsdk:"project_revision"`
	RetentionSeconds types.Int64  `tfsdk:"retention_seconds"`
	MaxArtifactBytes types.Int64  `tfsdk:"max_artifact_bytes"`
	MaxRunBytes      types.Int64  `tfsdk:"max_run_bytes"`
}

type exWorkflowArtifactRetentionDataSource struct {
	client  *apiclient.SemaphoreUI
	project bool
}

var _ datasource.DataSourceWithConfigure = &exWorkflowArtifactRetentionDataSource{}

func NewGlobalWorkflowArtifactRetentionDataSource() datasource.DataSource {
	return &exWorkflowArtifactRetentionDataSource{}
}
func NewProjectWorkflowArtifactRetentionDataSource() datasource.DataSource {
	return &exWorkflowArtifactRetentionDataSource{project: true}
}

func (d *exWorkflowArtifactRetentionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	name := "global_workflow_artifact_retention"
	if d.project {
		name = "project_workflow_artifact_retention"
	}
	resp.TypeName = req.ProviderTypeName + "_" + name
}
func (d *exWorkflowArtifactRetentionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Workflow Artifact Retention Configure Type", "Expected the configured Semaphore EX client.")
		return
	}
	d.client = client
}
func (d *exWorkflowArtifactRetentionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = exWorkflowArtifactRetentionDataSourceSchema(d.project)
}

func exWorkflowArtifactRetentionPolicyTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id": types.Int64Type, "scope": types.StringType, "project_id": types.Int64Type, "revision": types.Int64Type,
		"retention_seconds": types.Int64Type, "max_artifact_bytes": types.Int64Type, "max_run_bytes": types.Int64Type,
		"created_by_user_id": types.Int64Type, "created_at": types.StringType,
	}
}
func exWorkflowArtifactRetentionPolicyAttributes() map[string]ds.Attribute {
	return map[string]ds.Attribute{
		"id": ds.Int64Attribute{Computed: true}, "scope": ds.StringAttribute{Computed: true}, "project_id": ds.Int64Attribute{Computed: true}, "revision": ds.Int64Attribute{Computed: true},
		"retention_seconds": ds.Int64Attribute{Computed: true}, "max_artifact_bytes": ds.Int64Attribute{Computed: true}, "max_run_bytes": ds.Int64Attribute{Computed: true},
		"created_by_user_id": ds.Int64Attribute{Computed: true}, "created_at": ds.StringAttribute{Computed: true},
	}
}
func exWorkflowArtifactRetentionDataSourceSchema(project bool) ds.Schema {
	attributes := map[string]ds.Attribute{
		"id": ds.StringAttribute{Computed: true}, "global_policy": ds.SingleNestedAttribute{Computed: true, Attributes: exWorkflowArtifactRetentionPolicyAttributes()},
		"project_policy":  ds.SingleNestedAttribute{Computed: true, Attributes: exWorkflowArtifactRetentionPolicyAttributes()},
		"global_revision": ds.Int64Attribute{Computed: true}, "project_revision": ds.Int64Attribute{Computed: true}, "retention_seconds": ds.Int64Attribute{Computed: true},
		"max_artifact_bytes": ds.Int64Attribute{Computed: true}, "max_run_bytes": ds.Int64Attribute{Computed: true},
	}
	if project {
		attributes["project_id"] = ds.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}}
	} else {
		attributes["project_id"] = ds.Int64Attribute{Computed: true}
	}
	return ds.Schema{MarkdownDescription: "Reads the effective, revision-proven Semaphore EX workflow artifact retention policy.", Attributes: attributes}
}
func exWorkflowArtifactRetentionRoute(project bool) string {
	if project {
		return "/project/{project_id}/workflow-artifact-retention"
	}
	return "/workflow-artifact-retention"
}
func exWorkflowArtifactRetentionFromResponse(ctx context.Context, old exWorkflowArtifactRetentionModel, raw map[string]any, project bool) (exWorkflowArtifactRetentionModel, error) {
	policy := types.ObjectType{AttrTypes: exWorkflowArtifactRetentionPolicyTypes()}
	value, err := exTypedValue(ctx, types.ObjectType{AttrTypes: map[string]attr.Type{
		"global_policy": policy, "project_policy": policy,
		"effective": types.ObjectType{AttrTypes: map[string]attr.Type{"global_revision": types.Int64Type, "project_revision": types.Int64Type, "retention_seconds": types.Int64Type, "max_artifact_bytes": types.Int64Type, "max_run_bytes": types.Int64Type}},
	}}, raw)
	if err != nil {
		return old, err
	}
	objectValue, ok := value.(types.Object)
	if !ok {
		return old, fmt.Errorf("API response does not match the artifact-retention schema")
	}
	object := objectValue.Attributes()
	globalPolicy, globalOK := object["global_policy"].(types.Object)
	projectPolicy, projectOK := object["project_policy"].(types.Object)
	effectiveObject, effectiveOK := object["effective"].(types.Object)
	if !globalOK || !projectOK || !effectiveOK {
		return old, fmt.Errorf("API response has invalid artifact-retention policy fields")
	}
	effective := effectiveObject.Attributes()
	globalRevision, globalRevisionOK := effective["global_revision"].(types.Int64)
	projectRevision, projectRevisionOK := effective["project_revision"].(types.Int64)
	retention, retentionOK := effective["retention_seconds"].(types.Int64)
	artifactBytes, artifactBytesOK := effective["max_artifact_bytes"].(types.Int64)
	runBytes, runBytesOK := effective["max_run_bytes"].(types.Int64)
	if !globalRevisionOK || !projectRevisionOK || !retentionOK || !artifactBytesOK || !runBytesOK {
		return old, fmt.Errorf("API response has invalid artifact-retention effective limits")
	}
	next := exWorkflowArtifactRetentionModel{GlobalPolicy: globalPolicy, ProjectPolicy: projectPolicy, GlobalRevision: globalRevision, ProjectRevision: projectRevision, RetentionSeconds: retention, MaxArtifactBytes: artifactBytes, MaxRunBytes: runBytes}
	if project {
		next.ProjectID = old.ProjectID
		next.ID = types.StringValue("project/" + fmt.Sprint(old.ProjectID.ValueInt64()))
	} else {
		next.ProjectID = types.Int64Null()
		next.ID = types.StringValue("global")
	}
	return next, nil
}
func (d *exWorkflowArtifactRetentionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config exWorkflowArtifactRetentionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	params := map[string]string(nil)
	if d.project {
		id, err := exPathID(config.ProjectID)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Workflow Artifact Retention Project", err.Error())
			return
		}
		params = map[string]string{"project_id": id}
	}
	var raw map[string]any
	if err := exRequest(ctx, d.client, http.MethodGet, exWorkflowArtifactRetentionRoute(d.project), params, nil, &raw); err != nil {
		resp.Diagnostics.AddError("Error Reading Semaphore EX Workflow Artifact Retention", err.Error())
		return
	}
	next, err := exWorkflowArtifactRetentionFromResponse(ctx, config, raw, d.project)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Workflow Artifact Retention Response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}
