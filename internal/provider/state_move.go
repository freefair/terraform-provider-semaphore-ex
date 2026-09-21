package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// legacyStateMovers only migrates the same remote object from the published
// semaphoreui/semaphore schema. It performs no API requests or secret rotation.
func legacyStateMovers(target resource.Resource, suffix string) []resource.StateMover {
	return []resource.StateMover{{StateMover: func(ctx context.Context, req resource.MoveStateRequest, resp *resource.MoveStateResponse) {
		address := strings.Split(req.SourceProviderAddress, "/")
		if len(address) != 3 || address[1] != "semaphoreui" || address[2] != "semaphore" || req.SourceTypeName != "semaphoreui_"+suffix || req.SourceSchemaVersion != 0 {
			return
		}
		sourceType, ok := legacyResourceType(req.SourceTypeName)
		if !ok {
			return
		}
		if req.SourceRawState == nil || len(req.SourceRawState.JSON) == 0 {
			resp.Diagnostics.AddError("Invalid Legacy State", "The source JSON state is unavailable.")
			return
		}
		source, err := req.SourceRawState.Unmarshal(sourceType)
		if err != nil || source.IsNull() || !source.IsFullyKnown() {
			resp.Diagnostics.AddError("Invalid Legacy State", "The source must contain a complete, known state matching the published v0.3.9 schema.")
			return
		}
		var raw map[string]any
		decoder := json.NewDecoder(bytes.NewReader(req.SourceRawState.JSON))
		decoder.UseNumber()
		if err = decoder.Decode(&raw); err != nil {
			resp.Diagnostics.AddError("Invalid Legacy State", "The source state cannot be decoded.")
			return
		}
		var schema resource.SchemaResponse
		target.Schema(ctx, resource.SchemaRequest{}, &schema)
		resp.Diagnostics.Append(schema.Diagnostics...)
		if resp.Diagnostics.HasError() {
			return
		}
		typ, ok := schema.Schema.Type().(types.ObjectType)
		if !ok {
			resp.Diagnostics.AddError("Invalid Target Schema", "The target schema must be an object.")
			return
		}
		// EX intentionally has no durable runner credential outputs. These old
		// computed fields do not identify the runner or configure its behavior.
		if suffix == "runner" || suffix == "project_runner" {
			if raw["token"] != nil || raw["private_key"] != nil {
				resp.Diagnostics.AddWarning("Legacy Runner Credential Outputs Removed", "EX does not expose runner token or private_key outputs. The move preserves the runner and its registration; those obsolete outputs are omitted. Update any references to them before applying.")
			}
			delete(raw, "token")
			delete(raw, "private_key")
		}
		if err := checkLegacyFields(typ, raw); err != nil {
			resp.Diagnostics.AddError("Incompatible Legacy State", err.Error())
			return
		}
		value, err := exTypedValue(ctx, typ, raw)
		if err != nil {
			resp.Diagnostics.AddError("Incompatible Legacy State", "A source value cannot be represented by the EX schema. The source state is retained.")
			return
		}
		object, ok := value.(types.Object)
		if !ok {
			resp.Diagnostics.AddError("Invalid Target State", "The converted state must be an object.")
			return
		}
		identifiers := []string{"id"}
		if suffix == "runner_registration_token" {
			identifiers = []string{"id", "runner_id"}
			if !object.Attributes()["project_id"].IsNull() {
				identifiers = append(identifiers, "project_id")
			}
		} else if suffix == "project_user" {
			identifiers = []string{"project_id", "user_id"}
		} else if strings.HasPrefix(suffix, "project_") || suffix == "integration_alias" {
			identifiers = append(identifiers, "project_id")
		}
		for _, name := range identifiers {
			if _, err := exPathID(object.Attributes()[name]); err != nil {
				resp.Diagnostics.AddError("Invalid Legacy Identity", fmt.Sprintf("The source %s is missing or invalid.", name))
				return
			}
		}
		resp.Diagnostics.Append(resp.TargetState.Set(ctx, value)...)
		resp.TargetPrivate = req.SourcePrivate
	}}}
}

// Reject unknown fields recursively instead of silently discarding configured
// values when schemas evolve. Missing target fields become typed nulls and Read
// hydrates the EX-only settings from the existing object before planning.
func checkLegacyFields(target attr.Type, raw any) error {
	if raw == nil {
		return nil
	}
	switch typ := target.(type) {
	case types.ObjectType:
		fields, ok := raw.(map[string]any)
		if !ok {
			return nil
		}
		for name, value := range fields {
			child, ok := typ.AttrTypes[name]
			if !ok {
				return fmt.Errorf("source attribute %s has no EX equivalent", name)
			}
			if err := checkLegacyFields(child, value); err != nil {
				return err
			}
		}
	case types.ListType:
		if values, ok := raw.([]any); ok {
			for _, value := range values {
				if err := checkLegacyFields(typ.ElemType, value); err != nil {
					return err
				}
			}
		}
	case types.SetType:
		if values, ok := raw.([]any); ok {
			for _, value := range values {
				if err := checkLegacyFields(typ.ElemType, value); err != nil {
					return err
				}
			}
		}
	case types.MapType:
		if values, ok := raw.(map[string]any); ok {
			for _, value := range values {
				if err := checkLegacyFields(typ.ElemType, value); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (r *integrationAliasResource) MoveState(ctx context.Context) []resource.StateMover {
	return legacyStateMovers(r, "integration_alias")
}

func (r *projectResource) MoveState(ctx context.Context) []resource.StateMover {
	return legacyStateMovers(r, "project")
}

func (r *projectEnvironmentResource) MoveState(ctx context.Context) []resource.StateMover {
	return legacyStateMovers(r, "project_environment")
}

func (r *projectIntegrationResource) MoveState(ctx context.Context) []resource.StateMover {
	return legacyStateMovers(r, "project_integration")
}

func (r *projectInventoryResource) MoveState(ctx context.Context) []resource.StateMover {
	return legacyStateMovers(r, "project_inventory")
}

func (r *projectKeyResource) MoveState(ctx context.Context) []resource.StateMover {
	return legacyStateMovers(r, "project_key")
}

func (r *projectRepositoryResource) MoveState(ctx context.Context) []resource.StateMover {
	return legacyStateMovers(r, "project_repository")
}

func (r *projectRunnerResource) MoveState(ctx context.Context) []resource.StateMover {
	return legacyStateMovers(r, "project_runner")
}

func (r *projectScheduleResource) MoveState(ctx context.Context) []resource.StateMover {
	return legacyStateMovers(r, "project_schedule")
}

func (r *projectTemplateResource) MoveState(ctx context.Context) []resource.StateMover {
	return legacyStateMovers(r, "project_template")
}

func (r *projectUserResource) MoveState(ctx context.Context) []resource.StateMover {
	return legacyStateMovers(r, "project_user")
}

func (r *projectViewResource) MoveState(ctx context.Context) []resource.StateMover {
	return legacyStateMovers(r, "project_view")
}

func (r *runnerResource) MoveState(ctx context.Context) []resource.StateMover {
	return legacyStateMovers(r, "runner")
}

func (r *runnerRegistrationTokenResource) MoveState(ctx context.Context) []resource.StateMover {
	return legacyStateMovers(r, "runner_registration_token")
}

func (r *userResource) MoveState(ctx context.Context) []resource.StateMover {
	return legacyStateMovers(r, "user")
}
