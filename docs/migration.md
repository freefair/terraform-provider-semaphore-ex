# Migrate to Semaphore EX

The provider source is `freefair/semaphore-ex`; resource and data-source types use `semaphore_ex_` instead of `semaphoreui_`.
Use local provider name `semaphore` in `required_providers` and `provider "semaphore"`, matching Terraform's inferred first underscore-delimited resource prefix.
Existing `SEMAPHOREUI_*` environment names remain supported.

## Existing state

A provider-source replacement alone does not rename resource types.
Changing resource types directly can plan replacement, so back up state through the backend's normal mechanism and record the current import IDs first.
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
