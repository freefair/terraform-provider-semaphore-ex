package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/attr"

	schemaD "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	schemaR "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	superschema "github.com/orange-cloudavenue/terraform-plugin-framework-superschema"
)

// Preserve a configured empty list, but never hide remote deletion of entries.
func emptyListAfterRead(previous types.List, elementType attr.Type) types.List {
	if !previous.IsNull() && !previous.IsUnknown() && len(previous.Elements()) == 0 {
		return types.ListValueMust(elementType, []attr.Value{})
	}
	return types.ListNull(elementType)
}

func knownInt64Pointer(value types.Int64) *int64 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	return value.ValueInt64Pointer()
}

func preservedBoolAttribute(description string) superschema.BoolAttribute {
	return superschema.BoolAttribute{
		Common:     &schemaR.BoolAttribute{MarkdownDescription: description + " Omitted configuration preserves the server value."},
		Resource:   &schemaR.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}},
		DataSource: &schemaD.BoolAttribute{Computed: true},
	}
}

// preserveOptionalObject keeps imported objects and leaves an omitted new object
// null, so a Go pointer model never needs to represent an unknown whole object.
type preserveOptionalObject struct{}

func (preserveOptionalObject) Description(context.Context) string {
	return "Preserve an omitted object from prior state."
}
func (m preserveOptionalObject) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}
func (preserveOptionalObject) PlanModifyObject(_ context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if req.ConfigValue.IsNull() && req.PlanValue.IsUnknown() {
		resp.PlanValue = req.StateValue
	}
}
