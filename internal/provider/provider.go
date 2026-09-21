package provider

import (
	"context"
	"crypto/tls"
	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"net/http"
	"net/url"
	"os"
	"strconv"

	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &SemaphoreUIProvider{}
var _ provider.ProviderWithFunctions = &SemaphoreUIProvider{}
var _ provider.ProviderWithActions = &SemaphoreUIProvider{}

// SemaphoreUIProvider defines the provider implementation.
type SemaphoreUIProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// SemaphoreUIProviderModel describes the provider data model.
type SemaphoreUIProviderModel struct {
	ApiToken      types.String `tfsdk:"api_token"`
	TlsSkipVerify types.Bool   `tfsdk:"tls_skip_verify"`
	ApiBaseUrl    types.String `tfsdk:"api_base_url"`
}

func (p *SemaphoreUIProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "semaphore_ex"
	resp.Version = p.version
}

func (p *SemaphoreUIProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `
Use the Semaphore EX provider to interact with Semaphore EX via the API. You must configure the provider with the proper credentials before you can use it.

## API Token
You can generate a Semaphore API token by logging into Semaphore, opening the browser Developer Tools console, and running the following command:
` + "```javascript" + `
fetch("/api/user/tokens", {
  method: "POST",
  headers: {'Content-Type': 'application/json'},
  body: JSON.stringify({})
}).then(res => res.json()).then(data => console.log("api_token = " + data.id));
` + "```" + `
The token will be printed in the console. This token will grant the same level of access as the logged in user. Copy the token value and use it to configure the provider. The token is sensitive and should be treated as a secret. It is recommended to use the ` + "`SEMAPHOREUI_API_TOKEN`" + ` environment variable to configure the provider.
`,
		Attributes: map[string]schema.Attribute{
			"api_token": schema.StringAttribute{
				MarkdownDescription: "SemaphoreUI API token. This can also be defined by the `SEMAPHOREUI_API_TOKEN` environment variable.",
				Sensitive:           true,
				Optional:            true,
			},
			"api_base_url": schema.StringAttribute{
				MarkdownDescription: "SemaphoreUI API base URL. This can also be defined by the `SEMAPHOREUI_API_BASE_URL` environment variable. An explicit URL is required in configuration or the environment; there is no implicit localhost target.",
				Optional:            true,
			},
			"tls_skip_verify": schema.BoolAttribute{
				MarkdownDescription: "Skip TLS verification for the SemaphoreUI API when using https. This can also be defined by the `SEMAPHOREUI_TLS_SKIP_VERIFY` environment variable.  Default: `false`.",
				Optional:            true,
			},
		},
	}
}

func (p *SemaphoreUIProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config SemaphoreUIProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if config.ApiBaseUrl.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("api_base_url"), "Unknown SemaphoreUI API base URL", "The API endpoint must be known before configuring the provider. Supply an explicit URL in api_base_url or SEMAPHOREUI_API_BASE_URL.")
	}

	if config.ApiToken.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_token"),
			"Unknown SemaphoreUI API Token",
			"The provider cannot create the SemaphoreUI API client as there is an unknown configuration value for the SemaphoreUI API token. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the SEMAPHOREUI_API_TOKEN environment variable.",
		)
	}

	if config.TlsSkipVerify.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("tls_skip_verify"),
			"Unknown SemaphoreUI TLS Skip Verify",
			"The provider cannot create the SemaphoreUI API client as there is an unknown configuration value for the SemaphoreUI TLS Skip Verify. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the SEMAPHOREUI_TLS_SKIP_VERIFY environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Default values to environment variables, but override
	// with Terraform configuration value if set.
	apiToken := os.Getenv("SEMAPHOREUI_API_TOKEN")
	apiBaseUrl := os.Getenv("SEMAPHOREUI_API_BASE_URL")
	tlsSkipVerify := os.Getenv("SEMAPHOREUI_TLS_SKIP_VERIFY")

	if !config.ApiBaseUrl.IsNull() {
		apiBaseUrl = config.ApiBaseUrl.ValueString()
	}
	if !config.ApiToken.IsNull() {
		apiToken = config.ApiToken.ValueString()
	}
	if !config.TlsSkipVerify.IsNull() {
		tlsSkipVerify = strconv.FormatBool(config.TlsSkipVerify.ValueBool())
	}

	// If any of the expected configurations are missing, use defaults or return
	// errors with provider-specific guidance.
	if apiBaseUrl == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_base_url"),
			"Missing SemaphoreUI API base URL",
			"Set api_base_url in the configuration or use the SEMAPHOREUI_API_BASE_URL environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if apiToken == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_token"),
			"Missing SemaphoreUI API Token",
			"Set the API Token value in the configuration or use the SEMAPHOREUI_API_TOKEN environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if tlsSkipVerify == "" {
		tlsSkipVerify = "false" // Default
	}

	if resp.Diagnostics.HasError() {
		return
	}

	u, err := url.Parse(apiBaseUrl)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_base_url"),
			"Invalid SemaphoreUI API base URL",
			"The provider cannot create the SemaphoreUI API client as the API base URL must be an absolute HTTP(S) URL with a host. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the SEMAPHOREUI_API_BASE_URL environment variable.",
		)
		return
	}

	rt := httptransport.New(u.Host, u.Path, []string{u.Scheme})
	if tlsSkipVerify == "true" {
		rt.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}
	rt.Transport = &projectAbsenceTransport{next: rt.Transport, basePath: rt.BasePath}
	rt.DefaultAuthentication = httptransport.BearerToken(apiToken)

	client := apiclient.New(rt, strfmt.Default)
	resp.DataSourceData = client
	resp.ResourceData = client
	resp.ActionData = client
}

func (p *SemaphoreUIProvider) Actions(context.Context) []func() action.Action {
	return []func() action.Action{
		NewLDAPTestAction,
		NewProjectTemplatePublishAction,
		NewProjectWorkflowRestoreAction,
		NewProjectSecretStorageSyncAction,
		NewProjectSecretStorageConnectionTestAction,
		NewProjectEnvironmentSyncAction,
		NewProjectGeneratedSSHKeyRotateAction,
		NewWorkflowTriggerSigningPromoteAction,
		NewWorkflowTriggerSigningRevokeAction,
		NewAuditWebhookSigningPromoteAction,
		NewAuditWebhookSigningRevokeNextAction,
		NewOptionSetAction,
		NewProjectTaskStartAction,
		NewProjectTaskStopAction,
		NewProjectWorkflowStartAction,
		NewProjectWorkflowStopAction,
		NewProjectWorkflowApprovalAction,
		NewGlobalPolicyGuardrailDraftSaveAction,
		NewProjectPolicyGuardrailDraftSaveAction,
		NewGlobalPolicyGuardrailPublishAction,
		NewProjectPolicyGuardrailPublishAction,
		NewGlobalWorkflowArtifactRetentionPublishAction,
		NewProjectWorkflowArtifactRetentionPublishAction,
	}
}

func (p *SemaphoreUIProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewGlobalPolicyGuardrailResource,
		NewProjectPolicyGuardrailResource,
		NewGlobalWorkflowArtifactRetentionResource,
		NewProjectWorkflowArtifactRetentionResource,
		NewRuntimeSecretsResource,
		NewIntegrationAliasResource,
		NewAppResource,
		NewUserAPITokenResource,
		NewWorkflowDefinitionResource,
		NewWorkflowTriggerResource,
		NewCrossProjectGrantResource,
		NewDockerExecutionPolicyResource,
		NewKubernetesExecutionPolicyResource,
		NewGlobalNotificationDestinationResource,
		NewProjectNotificationDestinationResource,
		NewGlobalNotificationRuleResource,
		NewProjectNotificationRuleResource,
		NewAuditWebhookResource,
		NewProjectDeploymentWindowResource,
		NewGlobalRoleResource,
		NewGlobalRoleAssignmentResource,
		NewGlobalCredentialResource,
		NewGlobalCredentialGrantResource,
		NewTemplateACLResource,
		NewLDAPGroupMappingResource,
		NewOIDCGroupMappingResource,
		NewTOTPPolicyResource,
		NewLDAPConfigurationResource,
		NewProjectRoleResource,
		NewProjectSecretStorageResource,
		NewProjectIntegrationMatcherResource,
		NewProjectTemplateInventoryResource,
		NewProjectIntegrationExtractValueResource,
		NewProjectEnvironmentResource,
		NewProjectIntegrationResource,
		NewProjectInventoryResource,
		NewProjectKeyResource,
		NewProjectGeneratedSSHKeyResource,
		NewProjectTerraformBackendAliasResource,
		NewProjectRepositoryResource,
		NewProjectResource,
		NewProjectSSHKeyPolicyResource,
		NewProjectRunnerResource,
		NewProjectScheduleResource,
		NewProjectTemplateResource,
		NewProjectUserResource,
		NewProjectViewResource,
		NewRunnerRegistrationTokenResource,
		NewRunnerResource,
		NewUserResource,
	}
}

func (p *SemaphoreUIProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	sources := []func() datasource.DataSource{
		NewRuntimeSecretsDataSource,
		NewIntegrationAliasDataSource,
		NewExternalUserDataSource,
		NewAppDataSource,
		NewUserAPITokenDataSource,
		NewOptionDataSource,
		NewWorkflowDefinitionDataSource,
		NewWorkflowTriggerDataSource,
		NewCrossProjectGrantDataSource,
		NewDockerExecutionPolicyDataSource,
		NewKubernetesExecutionPolicyDataSource,
		NewGlobalNotificationDestinationDataSource,
		NewProjectNotificationDestinationDataSource,
		NewGlobalNotificationRuleDataSource,
		NewProjectNotificationRuleDataSource,
		NewAuditWebhookDataSource,
		NewProjectDeploymentWindowDataSource,
		NewGlobalWorkflowArtifactRetentionDataSource,
		NewProjectWorkflowArtifactRetentionDataSource,
		NewGlobalPolicyGuardrailDataSource,
		NewProjectPolicyGuardrailDataSource,
		NewGlobalRoleDataSource,
		NewGlobalRoleAssignmentDataSource,
		NewGlobalCredentialDataSource,
		NewGlobalCredentialGrantDataSource,
		NewTemplateACLDataSource,
		NewProjectTemplateVersionDataSource,
		NewProjectWorkflowVersionDataSource,
		NewProjectTemplateInventoryDataSource,
		NewLDAPGroupMappingDataSource,
		NewOIDCGroupMappingDataSource,
		NewTOTPPolicyDataSource,
		NewLDAPConfigurationDataSource,
		NewProjectRoleDataSource,
		NewProjectSecretStorageDataSource,
		NewProjectIntegrationMatcherDataSource,
		NewProjectIntegrationExtractValueDataSource,
		NewProjectDataSource,
		NewProjectEnvironmentDataSource,
		NewProjectIntegrationDataSource,
		NewProjectInventoryDataSource,
		NewProjectKeyDataSource,
		NewProjectGeneratedSSHKeyDataSource,
		NewProjectTerraformBackendAliasDataSource,
		NewProjectRepositoryDataSource,
		NewProjectSSHKeyPolicyDataSource,
		NewProjectRunnerDataSource,
		NewProjectScheduleDataSource,
		NewProjectsDataSource,
		NewProjectTemplateDataSource,
		NewProjectUserDataSource,
		NewProjectViewDataSource,
		NewRunnerDataSource,
		NewUserDataSource,
	}

	return append(sources, collectionDataSources()...)
}

func (p *SemaphoreUIProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &SemaphoreUIProvider{
			version: version,
		}
	}
}
