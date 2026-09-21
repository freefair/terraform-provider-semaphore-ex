# Unknown configuration values regression

## Scope

Restore validation and planning for project environments and keys when Terraform
has not resolved their configured values. Keep resource schemas, CRUD transport,
write-only handling, and published versions unchanged.

## Evidence and approach

The v1.0.1..v1.0.2 diff introduces whole-model Config.Get calls in both resource
validators and Plan.Get in environment ModifyPlan. Native maps and struct pointers
cannot represent unknown collections or objects.

Read only validation inputs using framework values, then decode known objects.
Defer checks whose required inputs remain unknown. Keep independent known-invalid
inputs reportable. This follows HashiCorp's framework validation contract.

A full CRUD model migration would unify representations but changes unrelated
read/write mapping and secret retention. Targeted validation reads keep this fix
bounded and preserve existing lifecycle behavior.

## Steps

1. Reproduce through provider protocol validation and environment planning with
   tests first stored in /tmp and injected through a Go overlay.
2. Update internal/provider/project_environment_resource.go and
   internal/provider/project_key_resource.go with unknown-aware attribute reads
   and checks.
3. Add internal/provider/unknown_config_test.go for unknown maps, map elements,
   nested objects, remote references, secret/sync list elements, and known-invalid
   controls. Verify planned unknowns remain unchanged.
4. Record the boundary in docs/adr/0006-unknown-configuration-validation.md.
5. Run focused regressions, task test, task build, and task lint. Inspect the diff
   and commit verified work. Publication is a separate approval step.
