package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	apiclient "github.com/freefair/terraform-provider-semaphore-ex/semaphoreui/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const exUserAPITokenIDPrefix = "semaphore-token-sha256-"

var errEXUserAPITokenNotFound = errors.New("API token was not found for the configured user")

type exUserAPITokenAPIResponse struct {
	ID        string  `json:"id"`
	TokenRef  string  `json:"token_ref"`
	Name      string  `json:"name"`
	ExpiresAt *string `json:"expires_at"`
	Created   string  `json:"created"`
	Expired   bool    `json:"expired"`
	UserID    int64   `json:"user_id"`
}

type exUserAPITokenResource struct{ client *apiclient.SemaphoreUI }
type exUserAPITokenDataSource struct{ client *apiclient.SemaphoreUI }

var (
	_ resource.ResourceWithImportState   = &exUserAPITokenResource{}
	_ resource.ResourceWithConfigure     = &exUserAPITokenResource{}
	_ datasource.DataSourceWithConfigure = &exUserAPITokenDataSource{}
)

func NewUserAPITokenResource() resource.Resource       { return &exUserAPITokenResource{} }
func NewUserAPITokenDataSource() datasource.DataSource { return &exUserAPITokenDataSource{} }

func (r *exUserAPITokenResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_api_token"
}

func (d *exUserAPITokenDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_api_token"
}

func (r *exUserAPITokenResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", "Expected *client.SemaphoreUI.")
		return
	}
	r.client = client
}

func (d *exUserAPITokenDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*apiclient.SemaphoreUI)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", "Expected *client.SemaphoreUI.")
		return
	}
	d.client = client
}

func (r *exUserAPITokenResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = exUserAPITokenResourceSchema()
}

func (d *exUserAPITokenDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = exUserAPITokenDataSourceSchema()
}

func exUserAPITokenIDValid(value string) bool {
	if !strings.HasPrefix(value, exUserAPITokenIDPrefix) || len(value) != len(exUserAPITokenIDPrefix)+64 {
		return false
	}
	for _, character := range value[len(exUserAPITokenIDPrefix):] {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

// exListUserAPITokens is also the capability preflight. The configured bearer
// credential authenticates this request, so a non-empty list proves that the
// server exposes token metadata for that credential's owner before POST can
// create a new one-time credential.
func exListUserAPITokens(ctx context.Context, client *apiclient.SemaphoreUI) ([]exUserAPITokenAPIResponse, error) {
	var tokens []exUserAPITokenAPIResponse
	if err := exRequest(ctx, client, http.MethodGet, "/user/tokens", nil, nil, &tokens); err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("semaphore EX does not list the configured API token; stable token references are required before creating a token")
	}
	for _, token := range tokens {
		if !exUserAPITokenIDValid(token.TokenRef) {
			return nil, fmt.Errorf("semaphore EX does not support stable token references; upgrade the server before creating a token")
		}
	}
	return tokens, nil
}

func exFindUserAPIToken(ctx context.Context, client *apiclient.SemaphoreUI, tokenID string) (exUserAPITokenAPIResponse, error) {
	if !exUserAPITokenIDValid(tokenID) {
		return exUserAPITokenAPIResponse{}, fmt.Errorf("api token identifier is invalid")
	}
	tokens, err := exListUserAPITokens(ctx, client)
	if err != nil {
		return exUserAPITokenAPIResponse{}, err
	}
	var match *exUserAPITokenAPIResponse
	for index := range tokens {
		if tokens[index].TokenRef != tokenID {
			continue
		}
		if match != nil {
			return exUserAPITokenAPIResponse{}, fmt.Errorf("semaphore EX returned duplicate stable API token identifiers")
		}
		match = &tokens[index]
	}
	if match == nil {
		return exUserAPITokenAPIResponse{}, errEXUserAPITokenNotFound
	}
	return *match, nil
}

func exUserAPITokenState(previous exUserAPITokenModel, raw exUserAPITokenAPIResponse, credential types.String) (exUserAPITokenModel, error) {
	if !exUserAPITokenIDValid(raw.TokenRef) {
		return previous, fmt.Errorf("semaphore EX returned an invalid stable API token reference")
	}
	next := previous
	next.ID = types.StringValue(raw.TokenRef)
	next.Name = types.StringValue(raw.Name)
	next.Created = types.StringValue(raw.Created)
	next.Expired = types.BoolValue(raw.Expired)
	next.UserID = types.Int64Value(raw.UserID)
	next.Credential = credential
	if raw.ExpiresAt == nil {
		next.ExpiresAt = types.StringNull()
	} else {
		next.ExpiresAt = types.StringValue(*raw.ExpiresAt)
	}
	return next, nil
}

func exCreateUserAPIToken(ctx context.Context, client *apiclient.SemaphoreUI, name string, expiresAt *string) (exUserAPITokenAPIResponse, error) {
	if _, err := exListUserAPITokens(ctx, client); err != nil {
		return exUserAPITokenAPIResponse{}, err
	}
	body := map[string]any{"name": name}
	if expiresAt != nil {
		body["expires_at"] = *expiresAt
	}
	var created exUserAPITokenAPIResponse
	if err := exRequest(ctx, client, http.MethodPost, "/user/tokens", nil, body, &created); err != nil {
		return exUserAPITokenAPIResponse{}, err
	}
	if created.ID == "" {
		return exUserAPITokenAPIResponse{}, fmt.Errorf("semaphore EX did not return the one-time API credential")
	}
	if !exUserAPITokenIDValid(created.TokenRef) {
		return exUserAPITokenAPIResponse{}, fmt.Errorf("semaphore EX returned an invalid stable API token reference")
	}
	return created, nil
}

func (r *exUserAPITokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan exUserAPITokenModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var expiresAt *string
	if !plan.ExpiresAt.IsNull() && !plan.ExpiresAt.IsUnknown() {
		value := plan.ExpiresAt.ValueString()
		expiresAt = &value
	}
	created, err := exCreateUserAPIToken(ctx, r.client, plan.Name.ValueString(), expiresAt)
	if err != nil {
		resp.Diagnostics.AddError("Semaphore EX API Token Contract Is Unavailable", err.Error())
		return
	}
	state, err := exUserAPITokenState(plan, created, types.StringValue(created.ID))
	if err != nil {
		resp.Diagnostics.AddError("Invalid Semaphore EX API Token Response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *exUserAPITokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state exUserAPITokenModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	raw, err := exFindUserAPIToken(ctx, r.client, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, errEXUserAPITokenNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error Reading Semaphore EX API Token", err.Error())
		return
	}
	next, err := exUserAPITokenState(state, raw, state.Credential)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Semaphore EX API Token Response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &next)...)
}

// Update is unreachable because every configurable attribute requires replacement.
func (r *exUserAPITokenResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Immutable Semaphore EX API Token", "API tokens cannot be updated. Change keepers or use Terraform -replace to create a replacement token.")
}

func (r *exUserAPITokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state exUserAPITokenModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !exUserAPITokenIDValid(state.ID.ValueString()) {
		resp.Diagnostics.AddError("Invalid Semaphore EX API Token Identifier", "Terraform state does not contain a stable API token identifier.")
		return
	}
	if err := exRequest(ctx, r.client, http.MethodDelete, "/user/tokens/{token_id}", map[string]string{"token_id": state.ID.ValueString()}, nil, nil); err != nil {
		resp.Diagnostics.AddError("Error Deleting Semaphore EX API Token", err.Error())
	}
}

func (r *exUserAPITokenResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !exUserAPITokenIDValid(req.ID) {
		resp.Diagnostics.AddError("Invalid User API Token Import ID", "Use the stable token_ref returned by Semaphore EX; API credentials and legacy credential prefixes cannot be imported.")
		return
	}
	raw, err := exFindUserAPIToken(ctx, r.client, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error Importing Semaphore EX API Token", err.Error())
		return
	}
	state, err := exUserAPITokenState(exUserAPITokenModel{ID: types.StringValue(req.ID), Keepers: types.MapNull(types.StringType), Credential: types.StringNull()}, raw, types.StringNull())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Semaphore EX API Token Response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
