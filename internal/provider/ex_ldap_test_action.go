package provider

import (
	"context"
	"net/http"
	"strings"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type ldapTestAction struct{ client *apiclient.SemaphoreUI }

// NewLDAPTestAction verifies a configured provider only when an operator
// explicitly supplies disposable directory and local-recovery credentials.
func NewLDAPTestAction() action.Action { return &ldapTestAction{} }

func (a *ldapTestAction) Metadata(_ context.Context, q action.MetadataRequest, p *action.MetadataResponse) {
	p.TypeName = q.ProviderTypeName + "_ldap_test"
}
func (a *ldapTestAction) Configure(_ context.Context, q action.ConfigureRequest, p *action.ConfigureResponse) {
	if q.ProviderData == nil {
		return
	}
	c, ok := q.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		p.Diagnostics.AddError("Unexpected LDAP Test Action Configure Type", "Expected the configured Semaphore EX client.")
		return
	}
	a.client = c
}
func (a *ldapTestAction) Schema(_ context.Context, _ action.SchemaRequest, p *action.SchemaResponse) {
	requiredSecret := func(description string) schema.StringAttribute {
		return schema.StringAttribute{Required: true, WriteOnly: true, MarkdownDescription: description, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}}
	}
	p.Schema = schema.Schema{MarkdownDescription: "Explicitly tests an LDAP provider and records readiness. Supply short-lived test and local-recovery credentials; this action never exposes or retains them. Invoke it after configuration and before explicitly setting active or selected_users state." + actionWriteOnlyLimitation, Attributes: map[string]schema.Attribute{
		"provider_id": requiredSecret("Configured LDAP provider identifier."),
		"username":    requiredSecret("Directory test username."), "password": requiredSecret("Directory test password."),
		"recovery_admin_user_id": schema.Int64Attribute{Required: true}, "recovery_admin_password": requiredSecret("Local recovery-administrator password."),
	}}
}
func (a *ldapTestAction) Invoke(ctx context.Context, q action.InvokeRequest, p *action.InvokeResponse) {
	var c struct {
		ProviderID            types.String `tfsdk:"provider_id"`
		Username              types.String `tfsdk:"username"`
		Password              types.String `tfsdk:"password"`
		RecoveryAdminPassword types.String `tfsdk:"recovery_admin_password"`
		RecoveryAdminUserID   types.Int64  `tfsdk:"recovery_admin_user_id"`
	}
	p.Diagnostics.Append(q.Config.Get(ctx, &c)...)
	if p.Diagnostics.HasError() {
		return
	}
	for name, value := range map[string]types.String{"provider_id": c.ProviderID, "username": c.Username, "password": c.Password, "recovery_admin_password": c.RecoveryAdminPassword} {
		if value.IsNull() || value.IsUnknown() || strings.TrimSpace(value.ValueString()) == "" {
			p.Diagnostics.AddError("Invalid LDAP Test Input", name+" must be a known non-empty value.")
			return
		}
	}
	if c.RecoveryAdminUserID.IsNull() || c.RecoveryAdminUserID.IsUnknown() || c.RecoveryAdminUserID.ValueInt64() < 1 {
		p.Diagnostics.AddError("Invalid LDAP Test Input", "recovery_admin_user_id must be a known positive integer.")
		return
	}
	body := map[string]any{"provider_id": c.ProviderID.ValueString(), "username": c.Username.ValueString(), "password": c.Password.ValueString(), "recovery_admin_user_id": c.RecoveryAdminUserID.ValueInt64(), "recovery_admin_password": c.RecoveryAdminPassword.ValueString()}
	var readiness struct {
		Status     string `json:"status"`
		Connection bool   `json:"connection"`
		Search     bool   `json:"search"`
		Bind       bool   `json:"bind"`
		Recovery   bool   `json:"recovery"`
	}
	if err := exRequest(ctx, a.client, http.MethodPost, "/capabilities/ldap/test", nil, body, &readiness); err != nil {
		p.Diagnostics.AddError("Error Testing LDAP Provider", err.Error())
		return
	}
	if readiness.Status != "ready" || !readiness.Connection || !readiness.Search || !readiness.Bind || !readiness.Recovery {
		p.Diagnostics.AddError("LDAP Provider Is Not Ready", "The LDAP test did not establish directory connectivity, bind, search, and local recovery readiness. Check the server-side LDAP readiness result before enabling the provider.")
	}
}
