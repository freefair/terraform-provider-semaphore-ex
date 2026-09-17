package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type appResource struct{ exRecordResource }

func NewAppResource() resource.Resource {
	return &appResource{exRecordResource: exRecordResource{spec: appSpec()}}
}
func (r *appResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan types.Object
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.spec.read(ctx, r.client, plan)
	if err == nil {
		resp.Diagnostics.AddError("Application Already Exists", "Import the existing application before managing it, or choose an unused custom application ID.")
		return
	}
	if !exNotFound(err) {
		resp.Diagnostics.AddError("Error Checking Application", err.Error())
		return
	}
	r.exRecordResource.Create(ctx, req, resp)
}
