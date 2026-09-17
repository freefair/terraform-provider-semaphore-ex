package provider

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"
)

func TestEXObjectPlanAndTypedValues(t *testing.T) {
	ctx := context.Background()
	s := schema.Schema{Attributes: map[string]schema.Attribute{"name": schema.StringAttribute{Required: true}, "id": schema.Int64Attribute{Computed: true}}}
	plan := tfsdk.Plan{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), map[string]tftypes.Value{"name": tftypes.NewValue(tftypes.String, "test"), "id": tftypes.NewValue(tftypes.Number, tftypes.UnknownValue)})}
	var object types.Object
	require.False(t, plan.Get(ctx, &object).HasError())
	require.True(t, object.Attributes()["id"].IsUnknown())
	_, err := exPathID(object.Attributes()["id"])
	require.Error(t, err)
	value, err := exTypedValue(ctx, types.Int64Type, json.Number("9007199254740993"))
	require.NoError(t, err)
	require.Equal(t, types.Int64Value(9007199254740993), value)
	objectValue := types.ObjectValueMust(map[string]attr.Type{"enabled": types.BoolType, "count": types.Int64Type}, map[string]attr.Value{"enabled": types.BoolValue(true), "count": types.Int64Value(3)})
	wire, err := exWireValue(ctx, objectValue)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"enabled": true, "count": int64(3)}, wire)
	_, err = exTypedValue(ctx, types.BoolType, "true")
	require.Error(t, err)
}

func TestEXNumberPreservesFractionAndLargeInteger(t *testing.T) {
	for _, number := range []string{"3.125", "9007199254740993.125", "1000000", "9007199254740993", "9223372036854775807"} {
		t.Run(number, func(t *testing.T) {
			value, err := exTypedValue(context.Background(), types.NumberType, json.Number(number))
			require.NoError(t, err)
			expected, _, err := big.ParseFloat(number, 10, 512, big.ToNearestEven)
			require.NoError(t, err)
			require.True(t, types.NumberValue(expected).Equal(value))
			wire, err := exWireValue(context.Background(), value)
			require.NoError(t, err)
			encoded, ok := wire.(json.Number)
			require.True(t, ok)
			if expected.IsInt() {
				_, err := encoded.Int64()
				require.NoError(t, err)
				require.Equal(t, number, encoded.String())
			}
			decoded, _, err := big.ParseFloat(encoded.String(), 10, 512, big.ToNearestEven)
			require.NoError(t, err)
			require.Zero(t, expected.Cmp(decoded))
		})
	}
	_, err := exTypedValue(context.Background(), types.NumberType, "3.125")
	require.Error(t, err)
}
