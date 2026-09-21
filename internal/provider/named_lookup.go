package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// A lookup resolves identity only. The existing reader always loads the detail
// endpoint, preserving its field conversions and credential redaction rules.
type namedLookupDataSource struct {
	datasource.DataSource
	spec   collectionSpec
	client *apiclient.SemaphoreUI
}

type collectionSpec struct {
	queryScopes                       []string
	opaqueID                          bool
	generatedOnly                     bool
	name, route, nameField, typeField string
	scopes                            []string
	paged                             bool
}

func lookupSpecs() map[string]collectionSpec {
	project := []string{"project_id"}
	return map[string]collectionSpec{
		"project":                          {name: "projects", route: "/projects", nameField: "name"},
		"project_environment":              {name: "project_environments", route: "/project/{project_id}/environment", nameField: "name", scopes: project},
		"project_template":                 {name: "project_templates", route: "/project/{project_id}/templates", nameField: "name", typeField: "app", scopes: project},
		"project_inventory":                {name: "project_inventories", route: "/project/{project_id}/inventory", nameField: "name", typeField: "type", scopes: project},
		"project_repository":               {name: "project_repositories", route: "/project/{project_id}/repositories", nameField: "name", scopes: project},
		"project_key":                      {name: "project_keys", route: "/project/{project_id}/keys", nameField: "name", typeField: "type", scopes: project},
		"project_generated_ssh_key":        {generatedOnly: true, route: "/project/{project_id}/keys", nameField: "name", typeField: "type", scopes: project},
		"project_integration":              {name: "project_integrations", route: "/project/{project_id}/integrations", nameField: "name", typeField: "auth_method", scopes: project},
		"project_schedule":                 {name: "project_schedules", route: "/project/{project_id}/schedules", nameField: "name", scopes: project},
		"project_runner":                   {name: "project_runners", route: "/project/{project_id}/runners", nameField: "name", scopes: project},
		"runner":                           {name: "runners", route: "/runners", nameField: "name"},
		"project_view":                     {name: "project_views", route: "/project/{project_id}/views", nameField: "title", scopes: project},
		"project_secret_storage":           {name: "project_secret_storages", route: "/project/{project_id}/secret_storages", nameField: "name", typeField: "type", scopes: project},
		"workflow_definition":              {name: "project_workflows", route: "/project/{project_id}/workflows", nameField: "name", scopes: project},
		"workflow_trigger":                 {name: "workflow_triggers", route: "/project/{project_id}/workflows/{workflow_id}/triggers", nameField: "name", typeField: "type", scopes: []string{"project_id", "workflow_id"}},
		"global_role":                      {name: "global_roles", opaqueID: true, route: "/roles", nameField: "name"},
		"project_role":                     {name: "project_roles", opaqueID: true, route: "/project/{project_id}/roles", nameField: "name", scopes: project},
		"global_credential":                {name: "global_credentials", route: "/global-credentials", nameField: "display_name", typeField: "type", paged: true},
		"global_notification_destination":  {name: "global_notification_destinations", route: "/notification-governance/destinations", nameField: "name", typeField: "provider", paged: true},
		"project_notification_destination": {name: "project_notification_destinations", route: "/project/{project_id}/notification-governance/destinations", nameField: "name", typeField: "provider", paged: true, scopes: project},
	}
}

func withNamedLookup(source datasource.DataSource, name string) datasource.DataSource {
	return &namedLookupDataSource{DataSource: withReadMetadata(source, name), spec: lookupSpecs()[name]}
}

func (d *namedLookupDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	d.DataSource.Schema(ctx, req, resp)
	switch id := resp.Schema.Attributes["id"].(type) {
	case ds.Int64Attribute:
		id.Required = false
		id.Optional = true
		id.Computed = true
		id.Validators = []validator.Int64{int64validator.AtLeast(1)}
		resp.Schema.Attributes["id"] = id
	case ds.StringAttribute:
		id.Required = false
		id.Optional = true
		id.Computed = true
		id.Validators = []validator.String{stringvalidator.LengthAtLeast(1)}
		resp.Schema.Attributes["id"] = id
	}
	resp.Schema.Attributes[d.spec.nameField] = ds.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Exact name within the selected scope. Configure either this attribute or id. Zero or multiple matches are errors.", Validators: []validator.String{stringvalidator.LengthAtLeast(1)}}
}
func (d *namedLookupDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{datasourcevalidator.ExactlyOneOf(path.MatchRoot("id"), path.MatchRoot(d.spec.nameField))}
}
func (d *namedLookupDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	var ok bool
	d.client, ok = req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Invalid Provider Client", "Expected the configured Semaphore EX client.")
		return
	}
	if configurable, ok := d.DataSource.(datasource.DataSourceWithConfigure); ok {
		configurable.Configure(ctx, req, resp)
	}
}
func (d *namedLookupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config types.Object
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	values := config.Attributes()
	id := values["id"]
	if id.IsNull() {
		name, ok := values[d.spec.nameField].(types.String)
		if !ok || name.IsNull() || name.IsUnknown() {
			resp.Diagnostics.AddError("Invalid Lookup", "A known id or exact name is required.")
			return
		}
		parameters, err := collectionParameters(d.spec, config)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Lookup Scope", err.Error())
			return
		}
		records, err := readCollection(ctx, d.client, d.spec, parameters)
		if err != nil {
			resp.Diagnostics.AddError("Error Reading Lookup Collection", err.Error())
			return
		}
		var match map[string]any
		for _, record := range records {
			if record[d.spec.nameField] != name.ValueString() {
				continue
			}
			if d.spec.generatedOnly {
				if _, generated := record["generated_ssh_key"].(map[string]any); !generated {
					continue
				}
			}
			if match != nil {
				resp.Diagnostics.AddError("Ambiguous Name", "Multiple records have the requested name in this scope; use an explicit id.")
				return
			}
			match = record
		}
		if match == nil {
			resp.Diagnostics.AddError("Name Not Found", "No accessible record has the requested exact name in this scope.")
			return
		}
		value, err := exTypedValue(ctx, config.AttributeTypes(ctx)["id"], match["id"])
		if err != nil || value.IsNull() || value.IsUnknown() {
			resp.Diagnostics.AddError("Invalid Lookup Response", "The matching record has no valid identity.")
			return
		}
		values["id"] = value
	} else if id.IsUnknown() {
		resp.Diagnostics.AddError("Invalid Lookup", "The id must be known before reading.")
		return
	}
	object, diagnostics := types.ObjectValue(config.AttributeTypes(ctx), values)
	resp.Diagnostics.Append(diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	raw, err := object.ToTerraformValue(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Lookup Configuration", err.Error())
		return
	}
	var base datasource.SchemaResponse
	d.DataSource.Schema(ctx, datasource.SchemaRequest{}, &base)
	resp.Diagnostics.Append(base.Diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	req.Config = tfsdk.Config{Schema: base.Schema, Raw: raw}
	d.DataSource.Read(ctx, req, resp)
}

func collectionParameters(spec collectionSpec, config types.Object) (map[string]string, error) {
	result := map[string]string{}
	for _, scope := range append(append([]string{}, spec.scopes...), spec.queryScopes...) {
		id, err := exPathID(config.Attributes()[scope])
		if err != nil {
			return nil, fmt.Errorf("%s must be a known positive identity", scope)
		}
		result[scope] = id
	}
	return result, nil
}

func readCollection(ctx context.Context, client *apiclient.SemaphoreUI, spec collectionSpec, parameters map[string]string) ([]map[string]any, error) {
	result := []map[string]any{}
	seen := map[string]bool{}
	const pageSize = 25
	const maximumPages = 4000
	for page := 0; page < maximumPages; page++ {
		options := exRequestOptions{PathParams: parameters, Query: map[string]string{}}
		for _, scope := range spec.queryScopes {
			options.Query[scope] = parameters[scope]
		}
		if spec.paged {
			options.Query["count"] = strconv.Itoa(pageSize)
			options.Query["offset"] = strconv.Itoa(page * pageSize)
		}
		var records []map[string]any
		if err := exRequestWithOptions(ctx, client, http.MethodGet, spec.route, options, nil, &records); err != nil {
			return nil, err
		}
		for _, record := range records {
			identity := fmt.Sprint(record["id"])
			if record["id"] == nil || identity == "" {
				return nil, fmt.Errorf("collection returned a record without an identity")
			}
			if seen[identity] {
				return nil, fmt.Errorf("collection repeated an identity; pagination is unstable, retry the read")
			}
			seen[identity] = true
			result = append(result, record)
		}
		if !spec.paged || len(records) < pageSize {
			return result, nil
		}
	}
	return nil, fmt.Errorf("collection exceeded the bounded pagination limit; refusing an incomplete result")
}
