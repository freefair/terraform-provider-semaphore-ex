package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type projectWorkflowVersionDataSource struct{ client *apiclient.SemaphoreUI }
type projectWorkflowVersionModel struct {
	ID                    types.Int64  `tfsdk:"id"`
	ProjectID             types.Int64  `tfsdk:"project_id"`
	WorkflowID            types.Int64  `tfsdk:"workflow_id"`
	VersionNumber         types.Int64  `tfsdk:"version_number"`
	ParentVersionID       types.Int64  `tfsdk:"parent_version_id"`
	RestoredFromVersionID types.Int64  `tfsdk:"restored_from_version_id"`
	AuthorUserID          types.Int64  `tfsdk:"author_user_id"`
	Message               types.String `tfsdk:"message"`
	ContentFingerprint    types.String `tfsdk:"content_fingerprint"`
	Created               types.String `tfsdk:"created"`
	Definition            types.Object `tfsdk:"definition"`
}
type workflowVersionResponse struct {
	ID                    int64          `json:"id"`
	VersionNumber         int64          `json:"version_number"`
	ParentVersionID       *int64         `json:"parent_version_id"`
	RestoredFromVersionID *int64         `json:"restored_from_version_id"`
	AuthorUserID          int64          `json:"author_user_id"`
	Message               string         `json:"message"`
	ContentFingerprint    string         `json:"content_fingerprint"`
	Created               string         `json:"created"`
	Definition            map[string]any `json:"definition"`
}

func NewProjectWorkflowVersionDataSource() datasource.DataSource {
	return &projectWorkflowVersionDataSource{}
}
func (d *projectWorkflowVersionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_workflow_version"
}
func (d *projectWorkflowVersionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Invalid Provider Client", "Expected the configured Semaphore EX client.")
		return
	}
	d.client = client
}
func workflowDefinitionAttributeTypes() map[string]attr.Type {
	result := make(map[string]attr.Type)
	for name, attribute := range workflowResourceSchema().Attributes {
		result[name] = attribute.GetType()
	}
	return result
}
func (d *projectWorkflowVersionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	positive := []validator.Int64{int64validator.AtLeast(1)}
	resp.Schema = schema.Schema{MarkdownDescription: "Reads an immutable workflow version with its complete definition and provenance. Omit version_number for the latest snapshot. Workflow edits create versions automatically; reading never restores or runs a workflow. Snapshot node keys use node-<server ID>.", Attributes: map[string]schema.Attribute{
		"id":                       schema.Int64Attribute{Computed: true},
		"project_id":               schema.Int64Attribute{Required: true, Validators: positive},
		"workflow_id":              schema.Int64Attribute{Required: true, Validators: positive},
		"version_number":           schema.Int64Attribute{Optional: true, Computed: true, Validators: positive},
		"parent_version_id":        schema.Int64Attribute{Computed: true},
		"restored_from_version_id": schema.Int64Attribute{Computed: true},
		"author_user_id":           schema.Int64Attribute{Computed: true},
		"message":                  schema.StringAttribute{Computed: true},
		"content_fingerprint":      schema.StringAttribute{Computed: true},
		"created":                  schema.StringAttribute{Computed: true},
		"definition":               schema.ObjectAttribute{Computed: true, AttributeTypes: workflowDefinitionAttributeTypes()},
	}}
}
func readWorkflowVersion(ctx context.Context, client *apiclient.SemaphoreUI, projectID, workflowID, version int64) (workflowVersionResponse, error) {
	params := map[string]string{"project_id": strconv.FormatInt(projectID, 10), "workflow_id": strconv.FormatInt(workflowID, 10)}
	if version == 0 {
		var versions []workflowVersionResponse
		if err := exRequestWithOptions(ctx, client, http.MethodGet, "/project/{project_id}/workflows/{workflow_id}/versions", exRequestOptions{PathParams: params, Query: map[string]string{"count": "1"}}, nil, &versions); err != nil {
			return workflowVersionResponse{}, err
		}
		if len(versions) == 0 || versions[0].VersionNumber <= 0 {
			return workflowVersionResponse{}, fmt.Errorf("workflow has no published definition version")
		}
		version = versions[0].VersionNumber
	}
	params["version_number"] = strconv.FormatInt(version, 10)
	var response workflowVersionResponse
	if err := exRequest(ctx, client, http.MethodGet, "/project/{project_id}/workflows/{workflow_id}/versions/{version_number}", params, nil, &response); err != nil {
		return response, err
	}
	if response.ID <= 0 || response.VersionNumber != version || response.Definition == nil {
		return response, fmt.Errorf("workflow API returned an invalid version snapshot")
	}
	return response, nil
}
func (d *projectWorkflowVersionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var model projectWorkflowVersionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &model)...)
	if resp.Diagnostics.HasError() {
		return
	}
	version, err := readWorkflowVersion(ctx, d.client, model.ProjectID.ValueInt64(), model.WorkflowID.ValueInt64(), model.VersionNumber.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Workflow Version", err.Error())
		return
	}
	definition, err := workflowDecodedDefinition(ctx, version.Definition)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Workflow Version Snapshot", err.Error())
		return
	}
	model.ID = types.Int64Value(version.ID)
	model.VersionNumber = types.Int64Value(version.VersionNumber)
	model.ParentVersionID = types.Int64PointerValue(version.ParentVersionID)
	model.RestoredFromVersionID = types.Int64PointerValue(version.RestoredFromVersionID)
	model.AuthorUserID = types.Int64Value(version.AuthorUserID)
	model.Message = types.StringValue(version.Message)
	model.ContentFingerprint = types.StringValue(version.ContentFingerprint)
	model.Created = types.StringValue(version.Created)
	value, diagnostics := types.ObjectValueFrom(ctx, workflowDefinitionAttributeTypes(), definition)
	resp.Diagnostics.Append(diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	model.Definition = value
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}
