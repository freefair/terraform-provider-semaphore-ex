package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// canonicalRunnerTagValidator prevents the server's normalization from changing
// a configured value after Terraform has committed to a plan.
type canonicalRunnerTagValidator struct{}

func (canonicalRunnerTagValidator) Description(context.Context) string {
	return "Runner tags must be nonempty, lowercase, trimmed, and at most 255 bytes."
}

func (v canonicalRunnerTagValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v canonicalRunnerTagValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	value := req.ConfigValue.ValueString()
	if value == "" || len(value) > 255 || value != strings.ToLower(strings.TrimSpace(value)) {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid runner tag", v.Description(ctx))
	}
}
