package provider

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type collectionDataSource struct {
	spec   collectionSpec
	client *apiclient.SemaphoreUI
}

func collectionDataSources() []func() datasource.DataSource {
	specs := lookupSpecs()
	specs["ldap_group_mapping"] = collectionSpec{name: "ldap_group_mappings", route: identityRoute(false), nameField: "group_external_id", opaqueID: true, queryScopes: []string{"provider_id"}}
	specs["oidc_group_mapping"] = collectionSpec{name: "oidc_group_mappings", route: identityRoute(true), nameField: "claim_value", opaqueID: true, queryScopes: []string{"provider_id"}}
	keys := make([]string, 0, len(specs))
	for key, spec := range specs {
		if spec.name != "" && spec.name != "projects" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	result := make([]func() datasource.DataSource, 0, len(keys))
	for _, key := range keys {
		spec := specs[key]
		result = append(result, func() datasource.DataSource { return &collectionDataSource{spec: spec} })
	}
	return result
}
func (d *collectionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + d.spec.name
}
func (d *collectionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	idType := attr.Type(types.Int64Type)
	idAttribute := ds.Attribute(ds.Int64Attribute{Computed: true})
	if d.spec.opaqueID {
		idType = types.StringType
		idAttribute = ds.StringAttribute{Computed: true}
	}
	item := map[string]ds.Attribute{"id": idAttribute, "revision": ds.Int64Attribute{Computed: true}, "enabled": ds.BoolAttribute{Computed: true}}
	if d.spec.nameField != "" {
		item[d.spec.nameField] = ds.StringAttribute{Computed: true}
	}
	if d.spec.typeField != "" {
		item[d.spec.typeField] = ds.StringAttribute{Computed: true}
	}
	attributes := map[string]ds.Attribute{
		"id":          ds.StringAttribute{Computed: true, MarkdownDescription: "Stable identity derived from the collection scope and filters."},
		"name_filter": ds.StringAttribute{Optional: true, MarkdownDescription: "Exact, case-sensitive match on " + d.spec.nameField + ". Omit to include every accessible record."},
		"ids":         ds.ListAttribute{Computed: true, ElementType: idType, MarkdownDescription: "Matching IDs in deterministic ascending order."},
		"items":       ds.ListNestedAttribute{Computed: true, NestedObject: ds.NestedAttributeObject{Attributes: item}, MarkdownDescription: "Identity and non-secret summary fields. Use the singular data source for complete details. Missing metadata remains null."},
	}
	if d.spec.typeField != "" {
		attributes["type_filter"] = ds.StringAttribute{Optional: true, MarkdownDescription: "Exact match on the API's " + d.spec.typeField + " field."}
	}
	for _, scope := range d.spec.scopes {
		attributes[scope] = ds.Int64Attribute{Required: true}
		item[scope] = ds.Int64Attribute{Computed: true}
	}
	for _, scope := range d.spec.queryScopes {
		attributes[scope] = ds.StringAttribute{Required: true}
		item[scope] = ds.StringAttribute{Computed: true}
	}
	resp.Schema = ds.Schema{MarkdownDescription: "Lists accessible " + strings.ReplaceAll(d.spec.name, "_", " ") + " without modifying them. Pagination is completed before filtering, and failures never produce partial results.", Attributes: attributes}
}
func (d *collectionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	var ok bool
	d.client, ok = req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Invalid Provider Client", "Expected the configured Semaphore EX client.")
	}
}
func (d *collectionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config types.Object
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	parameters, err := collectionParameters(d.spec, config)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Collection Scope", err.Error())
		return
	}
	records, err := readCollection(ctx, d.client, d.spec, parameters)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Collection", err.Error())
		return
	}
	values := config.Attributes()
	// Select only schema-declared summary fields. Credential material and backend
	// fields newly introduced in future releases never flow into collection state.
	var schema datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schema)
	itemListType, ok := schema.Schema.Attributes["items"].GetType().(types.ListType)
	if !ok {
		resp.Diagnostics.AddError("Invalid Collection Schema", "Expected a list of records.")
		return
	}
	itemType, ok := itemListType.ElemType.(types.ObjectType)
	if !ok {
		resp.Diagnostics.AddError("Invalid Collection Schema", "Expected typed record summaries.")
		return
	}
	filtered := make([]map[string]any, 0, len(records))
	for _, record := range records {
		include := true
		for filter, field := range map[string]string{"name_filter": d.spec.nameField, "type_filter": d.spec.typeField} {
			value, present := values[filter]
			if !present || value.IsNull() {
				continue
			}
			typed, ok := value.(types.String)
			if !ok || typed.IsUnknown() {
				resp.Diagnostics.AddError("Unknown Collection Filter", "Filters must be known when the data source is read.")
				return
			}
			if record[field] != typed.ValueString() {
				include = false
			}
		}
		if include {
			filtered = append(filtered, record)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if d.spec.opaqueID {
			return fmt.Sprint(filtered[i]["id"]) < fmt.Sprint(filtered[j]["id"])
		}
		left, _ := identityNumber(filtered[i]["id"])
		right, _ := identityNumber(filtered[j]["id"])
		return left < right
	})
	items := make([]attr.Value, 0, len(filtered))
	ids := make([]attr.Value, 0, len(filtered))
	for _, record := range filtered {
		summary := map[string]any{}
		for field := range itemType.AttrTypes {
			summary[field] = record[field]
		}
		for _, scope := range d.spec.scopes {
			value, err := exWireValue(ctx, config.Attributes()[scope])
			if err != nil {
				resp.Diagnostics.AddError("Invalid Collection Scope", err.Error())
				return
			}
			summary[scope] = value
		}
		for _, scope := range d.spec.queryScopes {
			summary[scope] = parameters[scope]
		}
		item, err := exTypedValue(ctx, itemType, summary)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Collection Response", err.Error())
			return
		}
		object, ok := item.(types.Object)
		if !ok {
			resp.Diagnostics.AddError("Invalid Collection Response", "Expected a record object.")
			return
		}
		items = append(items, object)
		ids = append(ids, object.Attributes()["id"])
	}
	itemsValue, diags := types.ListValue(itemType, items)
	resp.Diagnostics.Append(diags...)
	idsValue, diags := types.ListValue(itemType.AttrTypes["id"], ids)
	resp.Diagnostics.Append(diags...)
	values["items"], values["ids"] = itemsValue, idsValue
	identity := []string{d.spec.name}
	for _, scope := range append(append([]string{}, d.spec.scopes...), d.spec.queryScopes...) {
		identity = append(identity, scope, parameters[scope])
	}
	for _, name := range []string{"name_filter", "type_filter"} {
		if value, ok := values[name]; ok && !value.IsNull() {
			identity = append(identity, name, value.String())
		}
	}
	for i, part := range identity {
		identity[i] = url.PathEscape(part)
	}
	values["id"] = types.StringValue(strings.Join(identity, "/"))
	state, diags := types.ObjectValue(config.AttributeTypes(ctx), values)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
