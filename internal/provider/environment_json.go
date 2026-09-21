package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	sd "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	sr "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	superschema "github.com/orange-cloudavenue/terraform-plugin-framework-superschema"
)

type environmentJSONValidator struct{ scalarOnly bool }

func (v environmentJSONValidator) Description(context.Context) string {
	return "A JSON object; environment values must be scalar."
}
func (v environmentJSONValidator) MarkdownDescription(c context.Context) string {
	return v.Description(c)
}
func (v environmentJSONValidator) ValidateString(_ context.Context, q validator.StringRequest, p *validator.StringResponse) {
	if q.ConfigValue.IsNull() || q.ConfigValue.IsUnknown() {
		return
	}
	if _, err := environmentJSONObject(q.ConfigValue.ValueString(), v.scalarOnly); err != nil {
		p.Diagnostics.AddAttributeError(q.Path, "Invalid environment JSON", err.Error())
	}
}
func environmentJSONAttribute(mapName string, scalarOnly bool) superschema.StringAttribute {
	return superschema.StringAttribute{
		Common:     &sr.StringAttribute{MarkdownDescription: "Lossless JSON representation of " + mapName + ". Use jsonencode() for typed values; mutually exclusive with " + mapName + ". Omission preserves existing values; configure jsonencode({}) to clear them."},
		Resource:   &sr.StringAttribute{Optional: true, Computed: true, Validators: []validator.String{environmentJSONValidator{scalarOnly: scalarOnly}, stringvalidator.ConflictsWith(path.MatchRoot(mapName))}, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		DataSource: &sd.StringAttribute{Computed: true},
	}
}
func environmentJSONObject(raw string, scalarOnly bool) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		raw = "{}"
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil {
		return nil, fmt.Errorf("expected a valid JSON object")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("expected exactly one JSON object")
	}
	if object == nil {
		object = map[string]any{}
	}
	for name, value := range object {
		if name == "" {
			return nil, fmt.Errorf("JSON object keys must not be empty")
		}
		if scalarOnly {
			switch value.(type) {
			case []any, map[string]any:
				return nil, fmt.Errorf("environment variable values must be scalar")
			}
		}
	}
	return object, nil
}
func canonicalEnvironmentJSON(raw string, scalarOnly bool) (string, error) {
	object, err := environmentJSONObject(raw, scalarOnly)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(object)
	return string(encoded), err
}
func environmentJSONEqual(left, right string) bool {
	a, e := canonicalEnvironmentJSON(left, false)
	if e != nil {
		return false
	}
	b, e := canonicalEnvironmentJSON(right, false)
	return e == nil && bytes.Equal([]byte(a), []byte(b))
}
func environmentValuesFromAPI(raw string, previous types.Map, previousJSON types.String, scalarOnly bool) (types.Map, types.String, error) {
	object, err := environmentJSONObject(raw, scalarOnly)
	if err != nil {
		return types.MapNull(types.StringType), types.StringNull(), err
	}
	stringsOnly := true
	values := map[string]attr.Value{}
	for name, value := range object {
		switch v := value.(type) {
		case string:
			values[name] = types.StringValue(v)
		case nil:
			values[name] = types.StringNull()
		default:
			stringsOnly = false
		}
	}
	mapped := types.MapNull(types.StringType)
	if stringsOnly && (len(values) > 0 || (!previous.IsNull() && !previous.IsUnknown() && len(previous.Elements()) == 0)) {
		mapped = types.MapValueMust(types.StringType, values)
	}
	encoded, err := json.Marshal(object)
	if err != nil {
		return mapped, types.StringNull(), err
	}
	result := types.StringValue(string(encoded))
	if !previousJSON.IsNull() && !previousJSON.IsUnknown() && environmentJSONEqual(previousJSON.ValueString(), result.ValueString()) {
		result = previousJSON
	}
	return mapped, result, nil
}

// Both representations are computed so import can recover either authoring form.
// The explicitly configured form drives the request; the other is derived here.
func planEnvironmentJSON(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Config.Schema == nil {
		return
	}
	for _, name := range []string{"variables", "environment"} {
		var configuredMap types.Map
		var configuredJSON types.String
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(name), &configuredMap)...)
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(name+"_json"), &configuredJSON)...)
		if resp.Diagnostics.HasError() {
			return
		}
		previousMap := types.MapNull(types.StringType)
		previousJSON := types.StringNull()
		if req.State.Schema != nil && !req.State.Raw.IsNull() {
			resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(name), &previousMap)...)
			resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(name+"_json"), &previousJSON)...)
		}
		if !configuredMap.IsNull() {
			if !configuredJSON.IsNull() {
				continue
			}
			unknown := configuredMap.IsUnknown()
			for _, value := range configuredMap.Elements() {
				unknown = unknown || value.IsUnknown()
			}
			derived := types.StringUnknown()
			if !unknown {
				wire, err := exWireValue(ctx, configuredMap)
				if err != nil {
					resp.Diagnostics.AddError("Invalid Environment Variables", err.Error())
					return
				}
				encoded, err := json.Marshal(wire)
				if err != nil {
					resp.Diagnostics.AddError("Invalid Environment Variables", "Could not encode variables.")
					return
				}
				derived = types.StringValue(string(encoded))
				if !previousJSON.IsNull() && !previousJSON.IsUnknown() && environmentJSONEqual(previousJSON.ValueString(), derived.ValueString()) {
					derived = previousJSON
				}
			}
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root(name), configuredMap)...)
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root(name+"_json"), derived)...)
			continue
		}
		if !configuredJSON.IsNull() {
			mapped := types.MapUnknown(types.StringType)
			if !configuredJSON.IsUnknown() {
				var err error
				mapped, _, err = environmentValuesFromAPI(configuredJSON.ValueString(), previousMap, configuredJSON, name == "environment")
				if err != nil {
					resp.Diagnostics.AddAttributeError(path.Root(name+"_json"), "Invalid Environment JSON", err.Error())
					return
				}
			}
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root(name), mapped)...)
			continue
		}
		if previousJSON.IsNull() {
			previousJSON = types.StringValue("{}")
		}
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root(name), previousMap)...)
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root(name+"_json"), previousJSON)...)
	}
}

func environmentRequestJSON(ctx context.Context, mapped types.Map, encoded types.String, scalarOnly bool) (string, error) {
	if !encoded.IsNull() && !encoded.IsUnknown() {
		if _, err := environmentJSONObject(encoded.ValueString(), scalarOnly); err != nil {
			return "", err
		}
		return encoded.ValueString(), nil
	}
	if mapped.IsNull() {
		if encoded.IsUnknown() {
			return "", fmt.Errorf("JSON input must be known before applying")
		}
		return "{}", nil
	}
	value, err := exWireValue(ctx, mapped)
	if err != nil {
		return "", err
	}
	wire, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("could not encode variables")
	}
	return string(wire), nil
}
