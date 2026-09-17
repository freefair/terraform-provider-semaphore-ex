package provider

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	superschema "github.com/orange-cloudavenue/terraform-plugin-framework-superschema"
)

// exRecordSpec shares the mechanical lifecycle of ordinary records with explicit identities.
// Each resource still declares its own native schema, fixed routes and writable
// fields. Revisioned policies and operational transitions have separate handlers.
type exRecordSpec struct {
	name, collection, item, idParameter string
	scopes                              map[string]string // API parameter to Terraform attribute
	bodyFields                          []string
	importLabels                        []string
	importAttributes                    []string
	importStringID                      bool
	schema                              superschema.Schema
}

type exRecordResource struct {
	spec   exRecordSpec
	client *apiclient.SemaphoreUI
}
type exRecordDataSource struct {
	spec   exRecordSpec
	client *apiclient.SemaphoreUI
}

func (r *exRecordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.spec.name
}
func (r *exRecordResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = r.spec.schema.GetResource(ctx)
}
func (d *exRecordDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + d.spec.name
}
func (d *exRecordDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = d.spec.schema.GetDataSource(ctx)
}
func (r *exRecordResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Invalid Provider Client", "Expected the configured Semaphore EX client.")
		return
	}
	r.client = client
}
func (d *exRecordDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (s exRecordSpec) parameters(state types.Object, includeID bool) (map[string]string, error) {
	values := state.Attributes()
	result := map[string]string{}
	for parameter, attribute := range s.scopes {
		id, err := exPathID(values[attribute])
		if err != nil {
			return nil, err
		}
		result[parameter] = id
	}
	if includeID {
		id, err := exPathID(values["id"])
		if err != nil {
			return nil, err
		}
		result[s.idParameter] = id
	}
	return result, nil
}

func (s exRecordSpec) body(ctx context.Context, state types.Object) (map[string]any, error) {
	values := state.Attributes()
	result := map[string]any{}
	for _, name := range s.bodyFields {
		value := values[name]
		if value == nil || value.IsNull() || value.IsUnknown() {
			continue
		}
		wire, err := exWireValue(ctx, value)
		if err != nil {
			return nil, err
		}
		result[name] = wire
	}
	return result, nil
}

func (s exRecordSpec) state(ctx context.Context, previous types.Object, response map[string]any) (types.Object, error) {
	values := previous.Attributes()
	for name, target := range previous.AttributeTypes(ctx) {
		if raw, present := response[name]; present {
			value, err := exTypedValue(ctx, target, raw)
			if err != nil {
				return types.Object{}, fmt.Errorf("invalid %s response field %s: %w", s.name, name, err)
			}
			for _, scope := range s.scopes {
				if scope == name && !previous.Attributes()[name].Equal(value) {
					return types.Object{}, fmt.Errorf("API returned a different parent identity")
				}
			}
			values[name] = value
		} else if values[name].IsUnknown() {
			value, err := exTypedValue(ctx, target, nil)
			if err != nil {
				return types.Object{}, err
			}
			values[name] = value
		}
	}
	state, diagnostics := types.ObjectValue(previous.AttributeTypes(ctx), values)
	if diagnostics.HasError() {
		return types.Object{}, fmt.Errorf("invalid resource state returned by the API")
	}
	return state, nil
}

func (s exRecordSpec) read(ctx context.Context, client *apiclient.SemaphoreUI, state types.Object) (types.Object, error) {
	parameters, err := s.parameters(state, true)
	if err != nil {
		return types.Object{}, err
	}
	var response map[string]any
	if err := exRequest(ctx, client, http.MethodGet, s.item, parameters, nil, &response); err != nil {
		return types.Object{}, err
	}
	return s.state(ctx, state, response)
}

// createdIdentity retains a recoverable identity if decoding non-identity fields
// or the follow-up read fails after the server has created the record.
func (s exRecordSpec) createdIdentity(ctx context.Context, plan types.Object, response map[string]any) (types.Object, error) {
	identity := make(map[string]any)
	if id, present := response["id"]; present {
		identity["id"] = id
	}
	for _, scope := range s.scopes {
		if value, present := response[scope]; present {
			identity[scope] = value
		}
	}
	state, err := s.state(ctx, plan, identity)
	if err != nil {
		return types.Object{}, err
	}
	if _, err := s.parameters(state, true); err != nil {
		return types.Object{}, err
	}
	before, after := plan.Attributes()["id"], state.Attributes()["id"]
	if !before.IsNull() && !before.IsUnknown() && !before.Equal(after) {
		return types.Object{}, fmt.Errorf("API returned a different created resource identity")
	}
	return state, nil
}

func (r *exRecordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan types.Object
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	parameters, err := r.spec.parameters(plan, false)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Resource Identity", err.Error())
		return
	}
	body, err := r.spec.body(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Resource Configuration", err.Error())
		return
	}
	var response map[string]any
	if err = exRequest(ctx, r.client, http.MethodPost, r.spec.collection, parameters, body, &response); err != nil {
		resp.Diagnostics.AddError("Error Creating Semaphore EX Resource", err.Error())
		return
	}
	identity, err := r.spec.createdIdentity(ctx, plan, response)
	if err != nil {
		resp.Diagnostics.AddError("Missing Created Resource Identity", err.Error()+". The create request succeeded; inspect the server and import the record before retrying.")
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, identity)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, err := r.spec.state(ctx, identity, response)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Create Response", err.Error())
		return
	}
	if len(response) == 0 {
		state, err = r.spec.read(ctx, r.client, state)
		if err != nil {
			resp.Diagnostics.AddError("Error Reading Created Resource", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *exRecordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state types.Object
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	next, err := r.spec.read(ctx, r.client, state)
	if exNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Semaphore EX Resource", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, next)...)
}

func (r *exRecordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan types.Object
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	parameters, err := r.spec.parameters(plan, true)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Resource Identity", err.Error())
		return
	}
	body, err := r.spec.body(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Resource Configuration", err.Error())
		return
	}
	if err = exRequest(ctx, r.client, http.MethodPut, r.spec.item, parameters, body, nil); err != nil {
		resp.Diagnostics.AddError("Error Updating Semaphore EX Resource", err.Error())
		return
	}
	state, err := r.spec.read(ctx, r.client, plan)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Updated Resource", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *exRecordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state types.Object
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	parameters, err := r.spec.parameters(state, true)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Resource Identity", err.Error())
		return
	}
	if err = exRequest(ctx, r.client, http.MethodDelete, r.spec.item, parameters, nil, nil); err != nil && !exNotFound(err) {
		resp.Diagnostics.AddError("Error Deleting Semaphore EX Resource", err.Error())
	}
}

func (r *exRecordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if !r.spec.importStringID && len(parts) != 2*len(r.spec.importLabels) {
		resp.Diagnostics.AddError("Invalid Import ID", "Expected "+strings.Join(r.spec.importLabels, "/<id>/")+"/<id>.")
		return
	}
	objectType, ok := r.spec.schema.GetResource(ctx).Type().(types.ObjectType)
	if !ok {
		resp.Diagnostics.AddError("Invalid Import Schema", "Expected an object schema.")
		return
	}
	attributeTypes := objectType.AttrTypes
	values := map[string]attr.Value{}
	for name, target := range attributeTypes {
		value, err := exTypedValue(ctx, target, nil)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Import Schema", err.Error())
			return
		}
		values[name] = value
	}
	for i, label := range r.spec.importLabels {
		value := parts[2*i+1]
		if parts[2*i] != label || value == "" {
			resp.Diagnostics.AddError("Invalid Import ID", "Import labels and identifiers must match the documented format.")
			return
		}
		attribute := r.spec.importAttributes[i]
		if attributeTypes[attribute] == types.StringType {
			values[attribute] = types.StringValue(value)
			continue
		}
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			resp.Diagnostics.AddError("Invalid Import ID", "Numeric identifiers must be positive integers.")
			return
		}
		values[attribute] = types.Int64Value(id)
	}
	if r.spec.importStringID {
		if strings.TrimSpace(req.ID) == "" || strings.Contains(req.ID, "/") {
			resp.Diagnostics.AddError("Invalid Import ID", "Use the nonempty application ID.")
			return
		}
		values["id"] = types.StringValue(req.ID)
	}
	state, diagnostics := types.ObjectValue(attributeTypes, values)
	resp.Diagnostics.Append(diagnostics...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, err := r.spec.read(ctx, r.client, state)
	if err != nil {
		resp.Diagnostics.AddError("Error Importing Semaphore EX Resource", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (d *exRecordDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config types.Object
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state, err := d.spec.read(ctx, d.client, config)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Semaphore EX Data Source", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
