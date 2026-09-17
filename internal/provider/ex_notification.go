package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	ds "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rs "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Notification destinations keep provider credentials write-only. The API
// returns only credential_configured, which is enough to detect a missing
// credential without ever copying its material into Terraform state.
type exNotificationDestinationModel struct {
	ID                   types.Int64  `tfsdk:"id"`
	ProjectID            types.Int64  `tfsdk:"project_id"`
	Name                 types.String `tfsdk:"name"`
	DestinationType      types.String `tfsdk:"destination_type"`
	Environment          types.String `tfsdk:"environment"`
	Region               types.String `tfsdk:"region"`
	CredentialWO         types.String `tfsdk:"credential_wo"`
	CredentialWOVersion  types.Int64  `tfsdk:"credential_wo_version"`
	CredentialConfigured types.Bool   `tfsdk:"credential_configured"`
	Opsgenie             types.Object `tfsdk:"opsgenie"`
	ServiceNow           types.Object `tfsdk:"servicenow"`
	Enabled              types.Bool   `tfsdk:"enabled"`
	Paused               types.Bool   `tfsdk:"paused"`
	Revision             types.Int64  `tfsdk:"revision"`
}
type exNotificationDestinationDataSourceModel struct {
	ID                   types.Int64  `tfsdk:"id"`
	ProjectID            types.Int64  `tfsdk:"project_id"`
	Name                 types.String `tfsdk:"name"`
	DestinationType      types.String `tfsdk:"destination_type"`
	Environment          types.String `tfsdk:"environment"`
	Region               types.String `tfsdk:"region"`
	CredentialConfigured types.Bool   `tfsdk:"credential_configured"`
	Opsgenie             types.Object `tfsdk:"opsgenie"`
	ServiceNow           types.Object `tfsdk:"servicenow"`
	Enabled              types.Bool   `tfsdk:"enabled"`
	Paused               types.Bool   `tfsdk:"paused"`
	Revision             types.Int64  `tfsdk:"revision"`
}
type exNotificationRuleModel struct {
	ID               types.Int64  `tfsdk:"id"`
	ProjectID        types.Int64  `tfsdk:"project_id"`
	DestinationID    types.Int64  `tfsdk:"destination_id"`
	SourceKinds      types.Set    `tfsdk:"source_kinds"`
	LifecycleActions types.Set    `tfsdk:"lifecycle_actions"`
	MinimumSeverity  types.String `tfsdk:"minimum_severity"`
	Enabled          types.Bool   `tfsdk:"enabled"`
	Revision         types.Int64  `tfsdk:"revision"`
}
type exNotificationDestinationResource struct {
	client  *apiclient.SemaphoreUI
	project bool
}
type exNotificationDestinationDataSource struct {
	client  *apiclient.SemaphoreUI
	project bool
}
type exNotificationRuleResource struct {
	client  *apiclient.SemaphoreUI
	project bool
}
type exNotificationRuleDataSource struct {
	client  *apiclient.SemaphoreUI
	project bool
}

func NewGlobalNotificationDestinationResource() resource.Resource {
	return &exNotificationDestinationResource{}
}
func NewProjectNotificationDestinationResource() resource.Resource {
	return &exNotificationDestinationResource{project: true}
}
func NewGlobalNotificationDestinationDataSource() datasource.DataSource {
	return &exNotificationDestinationDataSource{}
}
func NewProjectNotificationDestinationDataSource() datasource.DataSource {
	return &exNotificationDestinationDataSource{project: true}
}
func NewGlobalNotificationRuleResource() resource.Resource { return &exNotificationRuleResource{} }
func NewProjectNotificationRuleResource() resource.Resource {
	return &exNotificationRuleResource{project: true}
}
func NewGlobalNotificationRuleDataSource() datasource.DataSource {
	return &exNotificationRuleDataSource{}
}
func NewProjectNotificationRuleDataSource() datasource.DataSource {
	return &exNotificationRuleDataSource{project: true}
}

func notificationScopeName(project bool) string {
	if project {
		return "project_notification"
	}
	return "global_notification"
}
func notificationDestinationOpsgenieType() map[string]attr.Type {
	return map[string]attr.Type{"priority": types.StringType, "responders": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"type": types.StringType, "id": types.StringType, "name": types.StringType, "username": types.StringType}}}}
}
func notificationDestinationServiceNowType() map[string]attr.Type {
	return map[string]attr.Type{"instance_origin": types.StringType, "auth_mode": types.StringType, "client_id": types.StringType, "basic_username": types.StringType, "scope": types.StringType, "field_mappings": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"incident_field": types.StringType, "source_field": types.StringType}}}}
}
func notificationOpsgenieResourceAttribute() rs.Attribute {
	return rs.SingleNestedAttribute{Optional: true, Attributes: map[string]rs.Attribute{"priority": rs.StringAttribute{Optional: true, Validators: []validator.String{stringvalidator.OneOf("P1", "P2", "P3", "P4", "P5")}}, "responders": rs.ListNestedAttribute{Optional: true, NestedObject: rs.NestedAttributeObject{Attributes: map[string]rs.Attribute{"type": rs.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("team", "user", "escalation", "schedule")}}, "id": rs.StringAttribute{Optional: true}, "name": rs.StringAttribute{Optional: true}, "username": rs.StringAttribute{Optional: true}}}}}}
}
func notificationServiceNowResourceAttribute() rs.Attribute {
	return rs.SingleNestedAttribute{Optional: true, Attributes: map[string]rs.Attribute{"instance_origin": rs.StringAttribute{Required: true}, "auth_mode": rs.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("oauth_client_credentials", "basic")}}, "client_id": rs.StringAttribute{Optional: true}, "basic_username": rs.StringAttribute{Optional: true}, "scope": rs.StringAttribute{Optional: true}, "field_mappings": rs.ListNestedAttribute{Required: true, NestedObject: rs.NestedAttributeObject{Attributes: map[string]rs.Attribute{"incident_field": rs.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("short_description", "description", "impact", "urgency")}}, "source_field": rs.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("summary", "severity", "lifecycle_action", "status")}}}}}}}
}
func notificationOpsgenieDataSourceAttribute() ds.Attribute {
	return ds.SingleNestedAttribute{Computed: true, Attributes: map[string]ds.Attribute{"priority": ds.StringAttribute{Computed: true}, "responders": ds.ListNestedAttribute{Computed: true, NestedObject: ds.NestedAttributeObject{Attributes: map[string]ds.Attribute{"type": ds.StringAttribute{Computed: true}, "id": ds.StringAttribute{Computed: true}, "name": ds.StringAttribute{Computed: true}, "username": ds.StringAttribute{Computed: true}}}}}}
}
func notificationServiceNowDataSourceAttribute() ds.Attribute {
	return ds.SingleNestedAttribute{Computed: true, Attributes: map[string]ds.Attribute{"instance_origin": ds.StringAttribute{Computed: true}, "auth_mode": ds.StringAttribute{Computed: true}, "client_id": ds.StringAttribute{Computed: true}, "basic_username": ds.StringAttribute{Computed: true}, "scope": ds.StringAttribute{Computed: true}, "field_mappings": ds.ListNestedAttribute{Computed: true, NestedObject: ds.NestedAttributeObject{Attributes: map[string]ds.Attribute{"incident_field": ds.StringAttribute{Computed: true}, "source_field": ds.StringAttribute{Computed: true}}}}}}
}

func notificationDestinationResourceSchema(project bool) rs.Schema {
	a := map[string]rs.Attribute{
		"id": rs.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}}, "name": rs.StringAttribute{Required: true}, "destination_type": rs.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}}, "environment": rs.StringAttribute{Required: true}, "region": rs.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Required as us or eu for PagerDuty and Opsgenie; omit for other destination types.", Validators: []validator.String{stringvalidator.OneOf("", "us", "eu")}},
		"credential_wo": rs.StringAttribute{Optional: true, Sensitive: true, WriteOnly: true}, "credential_wo_version": rs.Int64Attribute{Optional: true}, "credential_configured": rs.BoolAttribute{Computed: true}, "opsgenie": notificationOpsgenieResourceAttribute(), "servicenow": notificationServiceNowResourceAttribute(), "enabled": rs.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)}, "paused": rs.BoolAttribute{Computed: true}, "revision": rs.Int64Attribute{Computed: true}}
	if project {
		a["project_id"] = rs.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}}
	} else {
		a["project_id"] = rs.Int64Attribute{Computed: true}
	}
	return rs.Schema{MarkdownDescription: "Manages a Semaphore EX notification destination without persisting provider credentials.", Attributes: a}
}
func notificationDestinationDataSourceSchema(project bool) ds.Schema {
	a := map[string]ds.Attribute{"id": ds.Int64Attribute{Required: true}, "name": ds.StringAttribute{Computed: true}, "destination_type": ds.StringAttribute{Computed: true}, "environment": ds.StringAttribute{Computed: true}, "region": ds.StringAttribute{Computed: true}, "credential_configured": ds.BoolAttribute{Computed: true}, "opsgenie": notificationOpsgenieDataSourceAttribute(), "servicenow": notificationServiceNowDataSourceAttribute(), "enabled": ds.BoolAttribute{Computed: true}, "paused": ds.BoolAttribute{Computed: true}, "revision": ds.Int64Attribute{Computed: true}}
	if project {
		a["project_id"] = ds.Int64Attribute{Required: true}
	} else {
		a["project_id"] = ds.Int64Attribute{Computed: true}
	}
	return ds.Schema{MarkdownDescription: "Reads a Semaphore EX notification destination without credential material.", Attributes: a}
}
func notificationRuleResourceSchema(project bool) rs.Schema {
	a := map[string]rs.Attribute{"id": rs.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}}, "destination_id": rs.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}}, "source_kinds": rs.SetAttribute{Required: true, ElementType: types.StringType, Validators: []validator.Set{ /* server validates semantic combinations */ }}, "lifecycle_actions": rs.SetAttribute{Required: true, ElementType: types.StringType}, "minimum_severity": rs.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("info", "warning", "error", "critical")}}, "enabled": rs.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)}, "revision": rs.Int64Attribute{Computed: true}}
	if project {
		a["project_id"] = rs.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.AtLeast(1)}, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}}
	} else {
		a["project_id"] = rs.Int64Attribute{Computed: true}
	}
	return rs.Schema{MarkdownDescription: "Manages a revisioned Semaphore EX notification routing rule.", Attributes: a}
}
func notificationRuleDataSourceSchema(project bool) ds.Schema {
	a := map[string]ds.Attribute{"id": ds.Int64Attribute{Required: true}, "destination_id": ds.Int64Attribute{Computed: true}, "source_kinds": ds.SetAttribute{Computed: true, ElementType: types.StringType}, "lifecycle_actions": ds.SetAttribute{Computed: true, ElementType: types.StringType}, "minimum_severity": ds.StringAttribute{Computed: true}, "enabled": ds.BoolAttribute{Computed: true}, "revision": ds.Int64Attribute{Computed: true}}
	if project {
		a["project_id"] = ds.Int64Attribute{Required: true}
	} else {
		a["project_id"] = ds.Int64Attribute{Computed: true}
	}
	return ds.Schema{MarkdownDescription: "Reads a Semaphore EX notification routing rule.", Attributes: a}
}

func (r *exNotificationDestinationResource) Metadata(_ context.Context, q resource.MetadataRequest, p *resource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_" + notificationScopeName(r.project) + "_destination"
}
func (d *exNotificationDestinationDataSource) Metadata(_ context.Context, q datasource.MetadataRequest, p *datasource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_" + notificationScopeName(d.project) + "_destination"
}
func (r *exNotificationRuleResource) Metadata(_ context.Context, q resource.MetadataRequest, p *resource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_" + notificationScopeName(r.project) + "_rule"
}
func (d *exNotificationRuleDataSource) Metadata(_ context.Context, q datasource.MetadataRequest, p *datasource.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_" + notificationScopeName(d.project) + "_rule"
}
func (r *exNotificationDestinationResource) Schema(_ context.Context, _ resource.SchemaRequest, p *resource.SchemaResponse) {
	p.Schema = notificationDestinationResourceSchema(r.project)
}
func (d *exNotificationDestinationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, p *datasource.SchemaResponse) {
	p.Schema = notificationDestinationDataSourceSchema(d.project)
}
func (r *exNotificationRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, p *resource.SchemaResponse) {
	p.Schema = notificationRuleResourceSchema(r.project)
}
func (d *exNotificationRuleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, p *datasource.SchemaResponse) {
	p.Schema = notificationRuleDataSourceSchema(d.project)
}
func exNotificationClient(v any) (*apiclient.SemaphoreUI, bool) {
	c, ok := v.(*apiclient.SemaphoreUI)
	return c, ok
}
func (r *exNotificationDestinationResource) Configure(_ context.Context, q resource.ConfigureRequest, p *resource.ConfigureResponse) {
	if q.ProviderData == nil {
		return
	}
	if c, ok := exNotificationClient(q.ProviderData); ok {
		r.client = c
	} else {
		p.Diagnostics.AddError("Unexpected Resource Configure Type", "Expected *client.SemaphoreUI.")
	}
}
func (d *exNotificationDestinationDataSource) Configure(_ context.Context, q datasource.ConfigureRequest, p *datasource.ConfigureResponse) {
	if q.ProviderData == nil {
		return
	}
	if c, ok := exNotificationClient(q.ProviderData); ok {
		d.client = c
	} else {
		p.Diagnostics.AddError("Unexpected Data Source Configure Type", "Expected *client.SemaphoreUI.")
	}
}
func (r *exNotificationRuleResource) Configure(_ context.Context, q resource.ConfigureRequest, p *resource.ConfigureResponse) {
	if q.ProviderData == nil {
		return
	}
	if c, ok := exNotificationClient(q.ProviderData); ok {
		r.client = c
	} else {
		p.Diagnostics.AddError("Unexpected Resource Configure Type", "Expected *client.SemaphoreUI.")
	}
}
func (d *exNotificationRuleDataSource) Configure(_ context.Context, q datasource.ConfigureRequest, p *datasource.ConfigureResponse) {
	if q.ProviderData == nil {
		return
	}
	if c, ok := exNotificationClient(q.ProviderData); ok {
		d.client = c
	} else {
		p.Diagnostics.AddError("Unexpected Data Source Configure Type", "Expected *client.SemaphoreUI.")
	}
}

func notificationRoute(project bool, item string) string {
	if project {
		return "/project/{project_id}/notification-governance/" + item
	}
	return "/notification-governance/" + item
}
func notificationParams(project bool, projectID types.Int64, id types.Int64, key string) (map[string]string, error) {
	p := map[string]string{}
	if project {
		if projectID.IsNull() || projectID.IsUnknown() || projectID.ValueInt64() < 1 {
			return nil, fmt.Errorf("project_id is required")
		}
		p["project_id"] = strconv.FormatInt(projectID.ValueInt64(), 10)
	}
	if key != "" {
		if id.IsNull() || id.IsUnknown() || id.ValueInt64() < 1 {
			return nil, fmt.Errorf("id is required")
		}
		p[key] = strconv.FormatInt(id.ValueInt64(), 10)
	}
	return p, nil
}
func notificationDestinationBody(ctx context.Context, m exNotificationDestinationModel, credential *string, revision bool) (map[string]any, error) {
	b := map[string]any{"name": m.Name.ValueString(), "provider": m.DestinationType.ValueString(), "environment": m.Environment.ValueString(), "region": m.Region.ValueString(), "enabled": m.Enabled.ValueBool()}
	if revision {
		b["revision"] = m.Revision.ValueInt64()
	}
	for _, v := range []struct {
		name  string
		value types.Object
	}{{"opsgenie", m.Opsgenie}, {"servicenow", m.ServiceNow}} {
		if !v.value.IsNull() {
			x, e := exWireValue(ctx, v.value)
			if e != nil {
				return nil, e
			}
			b[v.name] = x
		}
	}
	if credential != nil {
		b["credential"] = *credential
	}
	return b, nil
}
func notificationDestinationFromResponse(ctx context.Context, old exNotificationDestinationModel, raw map[string]any) (exNotificationDestinationModel, error) {
	next := old
	if next.ProjectID.IsUnknown() {
		next.ProjectID = types.Int64Null()
	}
	for name, target := range map[string]attr.Type{"id": types.Int64Type, "name": types.StringType, "provider": types.StringType, "environment": types.StringType, "region": types.StringType, "credential_configured": types.BoolType, "enabled": types.BoolType, "paused": types.BoolType, "revision": types.Int64Type, "opsgenie": types.ObjectType{AttrTypes: notificationDestinationOpsgenieType()}, "servicenow": types.ObjectType{AttrTypes: notificationDestinationServiceNowType()}} {
		value, present := raw[name]
		if !present && name != "opsgenie" && name != "servicenow" {
			continue
		}
		v, e := exTypedValue(ctx, target, value)
		if e != nil {
			return old, fmt.Errorf("invalid notification destination response %s: %w", name, e)
		}
		switch name {
		case "id":
			next.ID, e = notificationTyped[types.Int64](v, name)
		case "name":
			next.Name, e = notificationTyped[types.String](v, name)
		case "provider":
			next.DestinationType, e = notificationTyped[types.String](v, name)
		case "environment":
			next.Environment, e = notificationTyped[types.String](v, name)
		case "region":
			next.Region, e = notificationTyped[types.String](v, name)
		case "credential_configured":
			next.CredentialConfigured, e = notificationTyped[types.Bool](v, name)
		case "enabled":
			next.Enabled, e = notificationTyped[types.Bool](v, name)
		case "paused":
			next.Paused, e = notificationTyped[types.Bool](v, name)
		case "revision":
			next.Revision, e = notificationTyped[types.Int64](v, name)
		case "opsgenie":
			next.Opsgenie, e = notificationTyped[types.Object](v, name)
		case "servicenow":
			next.ServiceNow, e = notificationTyped[types.Object](v, name)
		}
		if e != nil {
			return old, e
		}
	}
	if !next.Opsgenie.IsNull() && !next.Opsgenie.IsUnknown() && !old.Opsgenie.IsNull() && !old.Opsgenie.IsUnknown() {
		values := next.Opsgenie.Attributes()
		priorResponders, ok := old.Opsgenie.Attributes()["responders"].(types.List)
		if ok && values["responders"].IsNull() && !priorResponders.IsNull() && !priorResponders.IsUnknown() && len(priorResponders.Elements()) == 0 {
			values["responders"] = priorResponders
			next.Opsgenie, _ = types.ObjectValue(notificationDestinationOpsgenieType(), values)
		}
	}
	return next, nil
}
func notificationDestinationDataSourceFromResponse(ctx context.Context, old exNotificationDestinationDataSourceModel, raw map[string]any) (exNotificationDestinationDataSourceModel, error) {
	resourceState, err := notificationDestinationFromResponse(ctx, exNotificationDestinationModel{ID: old.ID, ProjectID: old.ProjectID, Name: old.Name, DestinationType: old.DestinationType, Environment: old.Environment, Region: old.Region, CredentialConfigured: old.CredentialConfigured, Opsgenie: old.Opsgenie, ServiceNow: old.ServiceNow, Enabled: old.Enabled, Paused: old.Paused, Revision: old.Revision}, raw)
	if err != nil {
		return old, err
	}
	return exNotificationDestinationDataSourceModel{ID: resourceState.ID, ProjectID: resourceState.ProjectID, Name: resourceState.Name, DestinationType: resourceState.DestinationType, Environment: resourceState.Environment, Region: resourceState.Region, CredentialConfigured: resourceState.CredentialConfigured, Opsgenie: resourceState.Opsgenie, ServiceNow: resourceState.ServiceNow, Enabled: resourceState.Enabled, Paused: resourceState.Paused, Revision: resourceState.Revision}, nil
}
func notificationRuleBody(ctx context.Context, m exNotificationRuleModel, revision bool) (map[string]any, error) {
	sk, e := exWireValue(ctx, m.SourceKinds)
	if e != nil {
		return nil, e
	}
	la, e := exWireValue(ctx, m.LifecycleActions)
	if e != nil {
		return nil, e
	}
	b := map[string]any{"destination_id": m.DestinationID.ValueInt64(), "source_kinds": sk, "lifecycle_actions": la, "minimum_severity": m.MinimumSeverity.ValueString(), "enabled": m.Enabled.ValueBool()}
	if revision {
		b["revision"] = m.Revision.ValueInt64()
	}
	return b, nil
}
func notificationRuleFromResponse(ctx context.Context, old exNotificationRuleModel, raw map[string]any) (exNotificationRuleModel, error) {
	next := old
	if next.ProjectID.IsUnknown() {
		next.ProjectID = types.Int64Null()
	}
	for name, target := range map[string]attr.Type{"id": types.Int64Type, "destination_id": types.Int64Type, "source_kinds": types.SetType{ElemType: types.StringType}, "lifecycle_actions": types.SetType{ElemType: types.StringType}, "minimum_severity": types.StringType, "enabled": types.BoolType, "revision": types.Int64Type} {
		v, e := exTypedValue(ctx, target, raw[name])
		if e != nil {
			return old, e
		}
		switch name {
		case "id":
			next.ID, e = notificationTyped[types.Int64](v, name)
		case "destination_id":
			next.DestinationID, e = notificationTyped[types.Int64](v, name)
		case "source_kinds":
			next.SourceKinds, e = notificationTyped[types.Set](v, name)
		case "lifecycle_actions":
			next.LifecycleActions, e = notificationTyped[types.Set](v, name)
		case "minimum_severity":
			next.MinimumSeverity, e = notificationTyped[types.String](v, name)
		case "enabled":
			next.Enabled, e = notificationTyped[types.Bool](v, name)
		case "revision":
			next.Revision, e = notificationTyped[types.Int64](v, name)
		}
		if e != nil {
			return old, e
		}
	}
	return next, nil
}

// Rules intentionally have no item GET endpoint in the current EX contract.
// Traverse the bounded server pages rather than inferring state from a stale
// mutation response. This keeps imports and refreshes correct for scopes with
// more than one page of rules.
func notificationReadRule(ctx context.Context, client *apiclient.SemaphoreUI, project bool, old exNotificationRuleModel) (exNotificationRuleModel, error) {
	params, err := notificationParams(project, old.ProjectID, types.Int64Null(), "")
	if err != nil {
		return old, err
	}
	for offset := 0; ; offset += 100 {
		options := exRequestOptions{PathParams: params, Query: map[string]string{"count": "100", "offset": strconv.Itoa(offset)}}
		var page []map[string]any
		if err := exRequestWithOptions(ctx, client, http.MethodGet, notificationRoute(project, "rules"), options, nil, &page); err != nil {
			return old, err
		}
		for _, raw := range page {
			id, err := exTypedValue(ctx, types.Int64Type, raw["id"])
			if err != nil {
				return old, fmt.Errorf("invalid notification rule list response: %w", err)
			}
			idValue, err := notificationTyped[types.Int64](id, "id")
			if err != nil {
				return old, err
			}
			if idValue.ValueInt64() == old.ID.ValueInt64() {
				return notificationRuleFromResponse(ctx, old, raw)
			}
		}
		if len(page) < 100 {
			return old, &exAPIError{StatusCode: http.StatusNotFound, method: http.MethodGet, route: notificationRoute(project, "rules")}
		}
	}
}

func notificationTyped[T attr.Value](value attr.Value, field string) (T, error) {
	converted, ok := value.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("invalid notification response field %s", field)
	}
	return converted, nil
}

func (r *exNotificationDestinationResource) Create(ctx context.Context, q resource.CreateRequest, p *resource.CreateResponse) {
	var plan, config exNotificationDestinationModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &plan)...)
	p.Diagnostics.Append(q.Config.Get(ctx, &config)...)
	if p.Diagnostics.HasError() {
		return
	}
	params, e := notificationParams(r.project, plan.ProjectID, types.Int64Null(), "")
	if e != nil {
		p.Diagnostics.AddError("Invalid Notification Destination", e.Error())
		return
	}
	var credential *string
	if !config.CredentialWO.IsNull() && !config.CredentialWO.IsUnknown() {
		v := config.CredentialWO.ValueString()
		credential = &v
	}
	body, e := notificationDestinationBody(ctx, plan, credential, false)
	if e != nil {
		p.Diagnostics.AddError("Invalid Notification Destination", e.Error())
		return
	}
	var raw map[string]any
	e = exRequest(ctx, r.client, http.MethodPost, notificationRoute(r.project, "destinations"), params, body, &raw)
	if e != nil {
		p.Diagnostics.AddError("Error Creating Semaphore EX Notification Destination", e.Error())
		return
	}
	state, e := notificationDestinationFromResponse(ctx, plan, raw)
	if e != nil {
		p.Diagnostics.AddError("Invalid Notification Destination Response", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &state)...)
}
func (r *exNotificationDestinationResource) Read(ctx context.Context, q resource.ReadRequest, p *resource.ReadResponse) {
	var state exNotificationDestinationModel
	p.Diagnostics.Append(q.State.Get(ctx, &state)...)
	if p.Diagnostics.HasError() {
		return
	}
	params, e := notificationParams(r.project, state.ProjectID, state.ID, "destination_id")
	if e == nil {
		var raw map[string]any
		e = exRequest(ctx, r.client, http.MethodGet, notificationRoute(r.project, "destinations/{destination_id}"), params, nil, &raw)
		if e == nil {
			state, e = notificationDestinationFromResponse(ctx, state, raw)
		}
	}
	if exNotFound(e) {
		p.State.RemoveResource(ctx)
		return
	}
	if e != nil {
		p.Diagnostics.AddError("Error Reading Semaphore EX Notification Destination", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &state)...)
}
func (r *exNotificationDestinationResource) Update(ctx context.Context, q resource.UpdateRequest, p *resource.UpdateResponse) {
	var plan, state, config exNotificationDestinationModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &plan)...)
	p.Diagnostics.Append(q.State.Get(ctx, &state)...)
	p.Diagnostics.Append(q.Config.Get(ctx, &config)...)
	if p.Diagnostics.HasError() {
		return
	}
	if state.Revision.IsNull() || state.Revision.IsUnknown() || state.Revision.ValueInt64() < 1 {
		p.Diagnostics.AddError("Notification Destination Revision Unavailable", "Refresh the destination and retry; the provider will not overwrite a concurrent change.")
		return
	}
	plan.ID = state.ID
	plan.Revision = state.Revision
	if notificationDestinationTypeChangeRequiresCredential(plan, state, config) {
		p.Diagnostics.AddError("Notification Destination Type Change Requires Credential", "Changing destination_type requires a new credential_wo value and a positive credential_wo_version.")
		return
	}
	var credential *string
	if !config.CredentialWO.IsNull() && !config.CredentialWO.IsUnknown() {
		v := config.CredentialWO.ValueString()
		credential = &v
	}
	params, e := notificationParams(r.project, plan.ProjectID, plan.ID, "destination_id")
	if e == nil {
		var body map[string]any
		body, e = notificationDestinationBody(ctx, plan, credential, true)
		if e == nil {
			var raw map[string]any
			e = exRequest(ctx, r.client, http.MethodPut, notificationRoute(r.project, "destinations/{destination_id}"), params, body, &raw)
			if e == nil {
				plan, e = notificationDestinationFromResponse(ctx, plan, raw)
			}
		}
	}
	if e != nil {
		p.Diagnostics.AddError("Error Updating Semaphore EX Notification Destination", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &plan)...)
}

func notificationDestinationTypeChangeRequiresCredential(plan, state, config exNotificationDestinationModel) bool {
	if plan.DestinationType.IsNull() || plan.DestinationType.IsUnknown() || state.DestinationType.IsNull() || state.DestinationType.IsUnknown() || plan.DestinationType.ValueString() == state.DestinationType.ValueString() {
		return false
	}
	return config.CredentialWO.IsNull() || config.CredentialWO.IsUnknown() || config.CredentialWO.ValueString() == "" || config.CredentialWOVersion.IsNull() || config.CredentialWOVersion.IsUnknown() || config.CredentialWOVersion.ValueInt64() < 1 || state.CredentialWOVersion.IsNull() || state.CredentialWOVersion.IsUnknown() || config.CredentialWOVersion.ValueInt64() == state.CredentialWOVersion.ValueInt64()
}
func (r *exNotificationDestinationResource) Delete(ctx context.Context, q resource.DeleteRequest, p *resource.DeleteResponse) {
	var state exNotificationDestinationModel
	p.Diagnostics.Append(q.State.Get(ctx, &state)...)
	if p.Diagnostics.HasError() {
		return
	}
	params, e := notificationParams(r.project, state.ProjectID, state.ID, "destination_id")
	if e == nil {
		e = exRequest(ctx, r.client, http.MethodDelete, notificationRoute(r.project, "destinations/{destination_id}"), params, map[string]any{"revision": state.Revision.ValueInt64()}, nil)
	}
	if e != nil && !exNotFound(e) {
		p.Diagnostics.AddError("Error Deleting Semaphore EX Notification Destination", e.Error())
	}
}
func (d *exNotificationDestinationDataSource) Read(ctx context.Context, q datasource.ReadRequest, p *datasource.ReadResponse) {
	var state exNotificationDestinationDataSourceModel
	p.Diagnostics.Append(q.Config.Get(ctx, &state)...)
	if p.Diagnostics.HasError() {
		return
	}
	params, e := notificationParams(d.project, state.ProjectID, state.ID, "destination_id")
	if e == nil {
		var raw map[string]any
		e = exRequest(ctx, d.client, http.MethodGet, notificationRoute(d.project, "destinations/{destination_id}"), params, nil, &raw)
		if e == nil {
			state, e = notificationDestinationDataSourceFromResponse(ctx, state, raw)
		}
	}
	if e != nil {
		p.Diagnostics.AddError("Error Reading Semaphore EX Notification Destination", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &state)...)
}
func (r *exNotificationDestinationResource) ImportState(ctx context.Context, q resource.ImportStateRequest, p *resource.ImportStateResponse) {
	parts := strings.Split(q.ID, "/")
	var projectID, id int64
	var e error
	if r.project {
		if len(parts) != 4 || parts[0] != "project" || parts[2] != "destination" {
			e = fmt.Errorf("use project/<project_id>/destination/<destination_id>")
		} else {
			projectID, e = strconv.ParseInt(parts[1], 10, 64)
			if e == nil {
				id, e = strconv.ParseInt(parts[3], 10, 64)
			}
		}
	} else {
		if len(parts) != 2 || parts[0] != "destination" {
			e = fmt.Errorf("use destination/<destination_id>")
		} else {
			id, e = strconv.ParseInt(parts[1], 10, 64)
		}
	}
	if e != nil || id < 1 || r.project && projectID < 1 {
		p.Diagnostics.AddError("Invalid Import ID", "Use the documented positive numeric import ID.")
		return
	}
	state := exNotificationDestinationModel{ID: types.Int64Value(id), ProjectID: types.Int64Value(projectID)}
	if !r.project {
		state.ProjectID = types.Int64Null()
	}
	params, _ := notificationParams(r.project, state.ProjectID, state.ID, "destination_id")
	var raw map[string]any
	e = exRequest(ctx, r.client, http.MethodGet, notificationRoute(r.project, "destinations/{destination_id}"), params, nil, &raw)
	if e == nil {
		state, e = notificationDestinationFromResponse(ctx, state, raw)
	}
	if e != nil {
		p.Diagnostics.AddError("Error Importing Semaphore EX Notification Destination", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &state)...)
}

func (r *exNotificationRuleResource) Create(ctx context.Context, q resource.CreateRequest, p *resource.CreateResponse) {
	var plan exNotificationRuleModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &plan)...)
	if p.Diagnostics.HasError() {
		return
	}
	params, e := notificationParams(r.project, plan.ProjectID, types.Int64Null(), "")
	body, e2 := notificationRuleBody(ctx, plan, false)
	if e == nil {
		e = e2
	}
	var raw map[string]any
	if e == nil {
		e = exRequest(ctx, r.client, http.MethodPost, notificationRoute(r.project, "rules"), params, body, &raw)
	}
	if e == nil {
		plan, e = notificationRuleFromResponse(ctx, plan, raw)
	}
	if e != nil {
		p.Diagnostics.AddError("Error Creating Semaphore EX Notification Rule", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &plan)...)
}
func (r *exNotificationRuleResource) Read(ctx context.Context, q resource.ReadRequest, p *resource.ReadResponse) {
	var state exNotificationRuleModel
	p.Diagnostics.Append(q.State.Get(ctx, &state)...)
	if p.Diagnostics.HasError() {
		return
	}
	state, e := notificationReadRule(ctx, r.client, r.project, state)
	if exNotFound(e) {
		p.State.RemoveResource(ctx)
		return
	}
	if e != nil {
		p.Diagnostics.AddError("Error Reading Semaphore EX Notification Rule", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &state)...)
}
func (r *exNotificationRuleResource) Update(ctx context.Context, q resource.UpdateRequest, p *resource.UpdateResponse) {
	var plan, state exNotificationRuleModel
	p.Diagnostics.Append(q.Plan.Get(ctx, &plan)...)
	p.Diagnostics.Append(q.State.Get(ctx, &state)...)
	if p.Diagnostics.HasError() {
		return
	}
	if state.Revision.IsNull() || state.Revision.IsUnknown() || state.Revision.ValueInt64() < 1 {
		p.Diagnostics.AddError("Notification Rule Revision Unavailable", "Refresh the rule and retry; the provider will not overwrite a concurrent change.")
		return
	}
	plan.ID = state.ID
	plan.Revision = state.Revision
	params, e := notificationParams(r.project, plan.ProjectID, plan.ID, "rule_id")
	body, e2 := notificationRuleBody(ctx, plan, true)
	if e == nil {
		e = e2
	}
	var raw map[string]any
	if e == nil {
		e = exRequest(ctx, r.client, http.MethodPut, notificationRoute(r.project, "rules/{rule_id}"), params, body, &raw)
		if e == nil {
			plan, e = notificationRuleFromResponse(ctx, plan, raw)
		}
	}
	if e != nil {
		p.Diagnostics.AddError("Error Updating Semaphore EX Notification Rule", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &plan)...)
}
func (r *exNotificationRuleResource) Delete(ctx context.Context, q resource.DeleteRequest, p *resource.DeleteResponse) {
	var state exNotificationRuleModel
	p.Diagnostics.Append(q.State.Get(ctx, &state)...)
	if p.Diagnostics.HasError() {
		return
	}
	params, e := notificationParams(r.project, state.ProjectID, state.ID, "rule_id")
	if e == nil {
		e = exRequest(ctx, r.client, http.MethodDelete, notificationRoute(r.project, "rules/{rule_id}"), params, map[string]any{"revision": state.Revision.ValueInt64()}, nil)
	}
	if e != nil && !exNotFound(e) {
		p.Diagnostics.AddError("Error Deleting Semaphore EX Notification Rule", e.Error())
	}
}
func (d *exNotificationRuleDataSource) Read(ctx context.Context, q datasource.ReadRequest, p *datasource.ReadResponse) {
	var state exNotificationRuleModel
	p.Diagnostics.Append(q.Config.Get(ctx, &state)...)
	if p.Diagnostics.HasError() {
		return
	}
	state, e := notificationReadRule(ctx, d.client, d.project, state)
	if e != nil {
		p.Diagnostics.AddError("Error Reading Semaphore EX Notification Rule", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &state)...)
}
func (r *exNotificationRuleResource) ImportState(ctx context.Context, q resource.ImportStateRequest, p *resource.ImportStateResponse) {
	parts := strings.Split(q.ID, "/")
	var projectID, id int64
	var e error
	if r.project {
		if len(parts) != 4 || parts[0] != "project" || parts[2] != "rule" {
			e = fmt.Errorf("use project/<project_id>/rule/<rule_id>")
		} else {
			projectID, e = strconv.ParseInt(parts[1], 10, 64)
			if e == nil {
				id, e = strconv.ParseInt(parts[3], 10, 64)
			}
		}
	} else {
		if len(parts) != 2 || parts[0] != "rule" {
			e = fmt.Errorf("use rule/<rule_id>")
		} else {
			id, e = strconv.ParseInt(parts[1], 10, 64)
		}
	}
	if e != nil || id < 1 || r.project && projectID < 1 {
		p.Diagnostics.AddError("Invalid Import ID", "Use the documented positive numeric import ID.")
		return
	}
	state := exNotificationRuleModel{ID: types.Int64Value(id), ProjectID: types.Int64Value(projectID)}
	if !r.project {
		state.ProjectID = types.Int64Null()
	}
	state, e = notificationReadRule(ctx, r.client, r.project, state)
	if e != nil {
		p.Diagnostics.AddError("Error Importing Semaphore EX Notification Rule", e.Error())
		return
	}
	p.Diagnostics.Append(p.State.Set(ctx, &state)...)
}

var _ resource.ResourceWithImportState = &exNotificationDestinationResource{}
var _ resource.ResourceWithImportState = &exNotificationRuleResource{}
var _ = path.Root // retained only while framework validator imports remain compatible
