package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"math/big"
)

func secretStorageParams(value types.Dynamic) map[string]any {
	result, _ := secretStorageAttribute(value.UnderlyingValue()).(map[string]any)
	return result
}

// secretStorageDynamicFromAPI infers Terraform-native dynamic values for import
// and data-source reads. Configured state is retained elsewhere to preserve an
// author-supplied list-versus-tuple shape during normal refreshes.
func secretStorageDynamicFromAPI(value any) types.Dynamic {
	if value == nil {
		return types.DynamicNull()
	}
	return types.DynamicValue(secretStorageAPIAttribute(value))
}

func secretStorageAPIAttribute(value any) attr.Value {
	switch value := value.(type) {
	case string:
		return types.StringValue(value)
	case bool:
		return types.BoolValue(value)
	case json.Number:
		number, _, err := big.ParseFloat(value.String(), 10, 256, big.ToNearestEven)
		if err != nil {
			return types.StringValue(value.String())
		}
		return types.NumberValue(number)
	case float64:
		return types.NumberValue(big.NewFloat(value))
	case int64:
		return types.NumberValue(big.NewFloat(float64(value)))
	case nil:
		return types.DynamicNull()
	case map[string]any:
		typesByName := map[string]attr.Type{}
		values := map[string]attr.Value{}
		for key, item := range value {
			child := secretStorageAPIAttribute(item)
			typesByName[key] = child.Type(context.Background())
			values[key] = child
		}
		return types.ObjectValueMust(typesByName, values)
	case []any:
		values := make([]attr.Value, 0, len(value))
		typesByIndex := make([]attr.Type, 0, len(value))
		same := true
		var first attr.Type
		for _, item := range value {
			child := secretStorageAPIAttribute(item)
			values = append(values, child)
			childType := child.Type(context.Background())
			typesByIndex = append(typesByIndex, childType)
			if first == nil {
				first = childType
			} else if fmt.Sprintf("%T", first) != fmt.Sprintf("%T", childType) {
				same = false
			}
		}
		if same && first != nil {
			return types.ListValueMust(first, values)
		}
		return types.TupleValueMust(typesByIndex, values)
	default:
		return types.StringValue(fmt.Sprint(value))
	}
}

func secretStorageAttribute(value attr.Value) any {
	switch value := value.(type) {
	case types.String:
		return value.ValueString()
	case types.Bool:
		return value.ValueBool()
	case types.Int64:
		return value.ValueInt64()
	case types.Float64:
		return value.ValueFloat64()
	case types.Number:
		return value.ValueBigFloat()
	case types.Map:
		result := map[string]any{}
		for key, item := range value.Elements() {
			result[key] = secretStorageAttribute(item)
		}
		return result
	case types.Object:
		result := map[string]any{}
		for key, item := range value.Attributes() {
			result[key] = secretStorageAttribute(item)
		}
		return result
	case types.List:
		result := make([]any, 0, len(value.Elements()))
		for _, item := range value.Elements() {
			result = append(result, secretStorageAttribute(item))
		}
		return result
	case types.Tuple:
		result := make([]any, 0, len(value.Elements()))
		for _, item := range value.Elements() {
			result = append(result, secretStorageAttribute(item))
		}
		return result
	default:
		return nil
	}
}
