# Migrate to Semaphore EX

The provider source is `freefair/semaphore-ex`; resource and data-source types use `semaphore_ex_` instead of `semaphoreui_`.
Use local provider name `semaphore` in `required_providers` and `provider "semaphore"`, matching Terraform's inferred first underscore-delimited resource prefix.
Existing `SEMAPHOREUI_*` environment names remain supported.

## Existing state

A provider-source replacement alone does not rename resource types.
Provider v1.0.4 and newer support direct `moved` migration from `semaphoreui/semaphore` v0.3.9, schema version 0, for all 15 resource types published by that provider.
Use EX provider v1.0.4 or newer; versions through 1.0.3 do not contain these state movers.
Keep the same API server and existing resource settings, replace the type prefix in resource blocks and references, and add one move per managed resource:

```hcl
moved {
  from = semaphoreui_project.example
  to   = semaphore_ex_project.example
}

resource "semaphore_ex_project" "example" {
  name = "Existing project"
}
```

Terraform 1.8 introduced cross-type moves; this provider's overall minimum remains Terraform 1.15.2.
Keep the old provider configuration available through the handover and select `freefair/semaphore-ex` for the destination resources.
Back up state through the backend's normal mechanism, inspect the plan for moves without create, update or destroy, then apply that plan.
IDs, scope and retained secret inputs are transferred without API writes.
Refresh fills in EX-only settings from the existing object.
The old runner `token` and `private_key` outputs have no EX equivalents and are omitted with a warning; update dependent output references first.
This does not revoke or re-register the runner.
A missing, incompatible or unknown source identity is rejected without changing the original state.
See the [lifecycle matrix](lifecycle.md) for import forms and per-resource behavior.

### Handover using remove and import

For Terraform 1.7 or newer, use a non-destructive handover with `removed` blocks and imports, keeping the old provider configuration available until the handover completes:

```hcl
removed {
  from = semaphoreui_project.example
  lifecycle {
    destroy = false
  }
}

resource "semaphore_ex_project" "example" {
  name = "Existing project"
}

import {
  to = semaphore_ex_project.example
  id = "42"
}
```

Replace the illustrative ID with the actual object's ID and preserve existing settings in configuration.
Nested resource IDs use formats such as `project/42/template/7`; consult the generated resource pages or example `import.sh` files.
Review the plan for imports without remote destruction or replacement before applying.
Retain secret inputs that the API cannot return.

## Variable groups and EX settings

Use `environment_ids = [group.id, another.id]` for multiple variable groups, or `[]` for none.
The singular `environment_id` remains deprecated; configure exactly one form.
EX returns unique group IDs in ascending ID order, so configuration order does not determine variable precedence.
Imports and data sources expose the full set.

Working directories, executor images, error-alert flags, runner-tag settings, schedule timezones and runner registration policies are optional/computed.
Omitting them after import preserves the server's values during unrelated updates.
Use an explicit empty string to clear working directory, executor image or timezone, and an empty set to clear runner tags.

## Memberships and runners

Membership `revision` is server-computed state, not a configurable input.
After a concurrency conflict, refresh and review the current membership before applying again.

Runner `token` and `private_key` outputs are removed: EX does not provide these values on detail GETs.
Create a runner inactive, obtain its one-time token through `semaphore_ex_runner_registration_token`, complete registration, then enable it.
`registration_policy` accepts `standard` or `secure` and survives import and updates.
Policy changes on registered runners remain subject to EX's lifecycle restrictions.
