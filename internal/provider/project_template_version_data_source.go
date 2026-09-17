package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type projectTemplateVersionDataSource struct{ client *apiclient.SemaphoreUI }
type projectTemplateVersionModel struct {
	ID                 types.Int64  `tfsdk:"id"`
	ProjectID          types.Int64  `tfsdk:"project_id"`
	TemplateID         types.Int64  `tfsdk:"template_id"`
	VersionNumber      types.Int64  `tfsdk:"version_number"`
	ContentFingerprint types.String `tfsdk:"content_fingerprint"`
	Created            types.String `tfsdk:"created"`
}

type templateVersionResponse struct {
	ID                 int64  `json:"id"`
	VersionNumber      int64  `json:"version_number"`
	ContentFingerprint string `json:"content_fingerprint"`
	Created            string `json:"created"`
}

func NewProjectTemplateVersionDataSource() datasource.DataSource {
	return &projectTemplateVersionDataSource{}
}
func (d *projectTemplateVersionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_template_version"
}
func (d *projectTemplateVersionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *projectTemplateVersionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Reads an immutable published template version and its content fingerprint. Omit version_number to select the latest published version. Reading never publishes a version.", Attributes: map[string]schema.Attribute{
		"id":                  schema.Int64Attribute{Computed: true, MarkdownDescription: "Server-assigned version record ID."},
		"project_id":          schema.Int64Attribute{Required: true, MarkdownDescription: "Project owning the template.", Validators: []validator.Int64{int64validator.AtLeast(1)}},
		"template_id":         schema.Int64Attribute{Required: true, MarkdownDescription: "Template whose versions to read.", Validators: []validator.Int64{int64validator.AtLeast(1)}},
		"version_number":      schema.Int64Attribute{Optional: true, Computed: true, MarkdownDescription: "Exact published version number, or latest when omitted.", Validators: []validator.Int64{int64validator.AtLeast(1)}},
		"content_fingerprint": schema.StringAttribute{Computed: true, MarkdownDescription: "Immutable SHA-256 content fingerprint."},
		"created":             schema.StringAttribute{Computed: true, MarkdownDescription: "Publication timestamp."},
	}}
}

func findTemplateVersion(ctx context.Context, client *apiclient.SemaphoreUI, projectID, templateID, number int64) (templateVersionResponse, error) {
	params := map[string]string{"project_id": strconv.FormatInt(projectID, 10), "template_id": strconv.FormatInt(templateID, 10)}
	query := map[string]string{"count": "100"}
	var before int64
	for {
		var versions []templateVersionResponse
		if err := exRequestWithOptions(ctx, client, http.MethodGet, "/project/{project_id}/templates/{template_id}/versions", exRequestOptions{PathParams: params, Query: query}, nil, &versions); err != nil {
			return templateVersionResponse{}, err
		}
		if len(versions) == 0 {
			return templateVersionResponse{}, fmt.Errorf("the requested published template version does not exist")
		}
		for _, version := range versions {
			if number == 0 || version.VersionNumber == number {
				return version, nil
			}
		}
		next := versions[len(versions)-1].ID
		if next <= 0 || (before > 0 && next >= before) {
			return templateVersionResponse{}, fmt.Errorf("template version pagination did not advance")
		}
		before = next
		query["before"] = strconv.FormatInt(before, 10)
	}
}

func (d *projectTemplateVersionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config projectTemplateVersionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	version, err := findTemplateVersion(ctx, d.client, config.ProjectID.ValueInt64(), config.TemplateID.ValueInt64(), config.VersionNumber.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Template Version", err.Error())
		return
	}
	config.ID = types.Int64Value(version.ID)
	config.VersionNumber = types.Int64Value(version.VersionNumber)
	config.ContentFingerprint = types.StringValue(version.ContentFingerprint)
	config.Created = types.StringValue(version.Created)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
