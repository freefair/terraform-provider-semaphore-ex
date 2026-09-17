package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// exWireValue converts already validated native HCL values into an API payload.
// It preserves numeric values instead of serializing every parameter as a string.
func exWireValue(ctx context.Context, value attr.Value) (any, error) {
	if value.IsUnknown() {
		return nil, fmt.Errorf("an API input is still unknown")
	}
	if value.IsNull() {
		return nil, nil
	}
	switch v := value.(type) {
	case types.String:
		return v.ValueString(), nil
	case types.Bool:
		return v.ValueBool(), nil
	case types.Int64:
		return v.ValueInt64(), nil
	case types.Float64:
		return v.ValueFloat64(), nil
	case types.Number:
		// Integer-only API fields use json.Number.Int64, which rejects exponent
		// notation even when its numeric value is integral.
		if integer, accuracy := v.ValueBigFloat().Int64(); accuracy == big.Exact {
			return json.Number(strconv.FormatInt(integer, 10)), nil
		}
		return json.Number(v.ValueBigFloat().Text('g', -1)), nil
	case types.Dynamic:
		return exWireValue(ctx, v.UnderlyingValue())
	case types.Object:
		return exWireAttributes(ctx, v.Attributes())
	case types.Map:
		return exWireAttributes(ctx, v.Elements())
	case types.List:
		return exWireElements(ctx, v.Elements())
	case types.Set:
		return exWireElements(ctx, v.Elements())
	case types.Tuple:
		return exWireElements(ctx, v.Elements())
	default:
		return nil, fmt.Errorf("unsupported Terraform API input type %T", value)
	}
}

func exWireAttributes(ctx context.Context, values map[string]attr.Value) (map[string]any, error) {
	result := make(map[string]any, len(values))
	for name, value := range values {
		converted, err := exWireValue(ctx, value)
		if err != nil {
			return nil, err
		}
		result[name] = converted
	}
	return result, nil
}

func exWireElements(ctx context.Context, values []attr.Value) ([]any, error) {
	result := make([]any, len(values))
	for i, value := range values {
		converted, err := exWireValue(ctx, value)
		if err != nil {
			return nil, err
		}
		result[i] = converted
	}
	return result, nil
}

// exTypedValue decodes API values against the resource's declared schema rather
// than inferring lossy string maps or replacing configured collection types.
func exTypedValue(ctx context.Context, target attr.Type, raw any) (attr.Value, error) {
	if raw == nil {
		return target.ValueFromTerraform(ctx, tftypes.NewValue(target.TerraformType(ctx), nil))
	}
	switch t := target.(type) {
	case basetypes.StringType:
		value, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("API returned a non-string value")
		}
		return types.StringValue(value), nil
	case basetypes.BoolType:
		value, ok := raw.(bool)
		if !ok {
			return nil, fmt.Errorf("API returned a non-boolean value")
		}
		return types.BoolValue(value), nil
	case basetypes.Int64Type:
		var value int64
		switch n := raw.(type) {
		case json.Number:
			parsed, err := n.Int64()
			if err != nil {
				return nil, fmt.Errorf("API returned a non-integer value")
			}
			value = parsed
		case int64:
			value = n
		case int:
			value = int64(n)
		default:
			return nil, fmt.Errorf("API returned a non-integer value")
		}
		return types.Int64Value(value), nil
	case basetypes.NumberType:
		var number string
		switch value := raw.(type) {
		case json.Number:
			number = value.String()
		case int64:
			number = strconv.FormatInt(value, 10)
		case int:
			number = strconv.Itoa(value)
		default:
			return nil, fmt.Errorf("API returned a non-numeric value")
		}
		value, _, err := big.ParseFloat(number, 10, 512, big.ToNearestEven)
		if err != nil {
			return nil, fmt.Errorf("API returned an invalid number")
		}
		return types.NumberValue(value), nil
	case types.ObjectType:
		object, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("API returned a non-object value")
		}
		values := make(map[string]attr.Value, len(t.AttrTypes))
		for name, fieldType := range t.AttrTypes {
			value, err := exTypedValue(ctx, fieldType, object[name])
			if err != nil {
				return nil, err
			}
			values[name] = value
		}
		value, diagnostics := types.ObjectValue(t.AttrTypes, values)
		if diagnostics.HasError() {
			return nil, fmt.Errorf("API object does not match the Terraform schema")
		}
		return value, nil
	case types.ListType:
		values, err := exTypedElements(ctx, t.ElemType, raw)
		if err != nil {
			return nil, err
		}
		value, diagnostics := types.ListValue(t.ElemType, values)
		if diagnostics.HasError() {
			return nil, fmt.Errorf("API list does not match the Terraform schema")
		}
		return value, nil
	case types.SetType:
		values, err := exTypedElements(ctx, t.ElemType, raw)
		if err != nil {
			return nil, err
		}
		value, diagnostics := types.SetValue(t.ElemType, values)
		if diagnostics.HasError() {
			return nil, fmt.Errorf("API set does not match the Terraform schema")
		}
		return value, nil
	case types.MapType:
		object, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("API returned a non-map value")
		}
		values := make(map[string]attr.Value, len(object))
		for name, rawValue := range object {
			value, err := exTypedValue(ctx, t.ElemType, rawValue)
			if err != nil {
				return nil, err
			}
			values[name] = value
		}
		value, diagnostics := types.MapValue(t.ElemType, values)
		if diagnostics.HasError() {
			return nil, fmt.Errorf("API map does not match the Terraform schema")
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported Terraform API response type %T", target)
	}
}

func exTypedElements(ctx context.Context, elementType attr.Type, raw any) ([]attr.Value, error) {
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("API returned a non-array value")
	}
	values := make([]attr.Value, len(items))
	for i, item := range items {
		value, err := exTypedValue(ctx, elementType, item)
		if err != nil {
			return nil, err
		}
		values[i] = value
	}
	return values, nil
}

func exPathID(value attr.Value) (string, error) {
	if value == nil || value.IsUnknown() || value.IsNull() {
		return "", fmt.Errorf("resource identity is unavailable")
	}
	switch id := value.(type) {
	case types.Int64:
		if id.ValueInt64() <= 0 {
			return "", fmt.Errorf("resource identity must be positive")
		}
		return strconv.FormatInt(id.ValueInt64(), 10), nil
	case types.String:
		if id.ValueString() == "" {
			return "", fmt.Errorf("resource identity must not be empty")
		}
		return id.ValueString(), nil
	default:
		return "", fmt.Errorf("unsupported resource identity type")
	}
}
