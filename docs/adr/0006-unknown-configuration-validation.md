# Preserve unknown values during configuration validation

Status: Accepted

## Context

Version 1.0.2 adds resource validation for project environments and keys and plan
validation for environment synchronization. The new callbacks decode the entire
configuration or plan into existing CRUD models. Native map pointers cannot
represent unknown maps or string elements, and struct pointers cannot represent
unknown objects. Consequently, valid references to resources created in the same
apply produce a framework Value Conversion Error on variables, environment, or
login_password before the API is involved.

Unknown and null are distinct: an unknown storage ID or secret is pending, whereas
null means absent. Source and synchronization checks must preserve that distinction.

## Decision

Read only the attributes needed by each validation or plan callback. Read nested
objects and list elements as framework values and decode only known, non-null
objects. Leaf fields retain their existing framework scalar types.

Evaluate a constraint only when its necessary inputs are known. Continue checking
independent known values and other list elements so an unrelated unknown does not
hide a definite conflict or missing required field. Environment planning reads
only sync_paths and secret_storage, defers unresolved storage IDs, and does not
change the planned values.

This follows the [HashiCorp validation contract](https://developer.hashicorp.com/terraform/plugin/framework/validation).
Keep the schemas, resolved CRUD models, API conversions, and write-only secret
handling unchanged.

## Alternatives and consequences

Migrating every CRUD field to framework collection/object types would remove the
representation difference, but would also change read/write conversion and secret
retention paths unrelated to the regression. Targeted reads isolate this fix to
the callbacks introduced in 1.0.2.

Skipping validation whenever any configuration value is unknown would be simpler,
but would conceal independent, already-provable errors. Substituting empty values
for unknowns would change their meaning and could generate incorrect plans.

## Verification

Regression tests call the registered provider's ValidateResourceConfig protocol
with unknown maps, map elements, credential objects, and remote-reference fields.
Known-invalid controls verify that source conflicts and missing fields still fail.
Direct environment callback tests cover unknown list elements and planned storage
bindings, including explicit null, empty lists, and destroy. Assertions verify that
planning retains the original raw value.

The framework's own required-child validation rejects a wholly unknown secrets
list element before calling resource validation. That provider decoding case is
therefore tested directly; no framework or schema behavior is changed here.
