# Operational Actions and ephemeral previews

> **Known Action limitation:** WriteOnly Action inputs are currently rejected by
> the official framework, including LDAP apply tokens and execution preflight
> tokens. See [affected Actions and behavior](known-limitations.md#write-only-action-inputs).


Use Actions for explicit operations. Several preview Actions also have a matching
ephemeral resource when their structured result must be consumed by Terraform.
Ephemeral results are evaluated when Terraform opens the resource and are not saved
in state. They can be evaluated again across plan/apply; do not assume one invocation.

```hcl
ephemeral "semaphore_ex_global_policy_guardrail_validate" "candidate" {
  source_yaml = yamlencode({ version = 1, rules = [] })
  lifecycle {
    postcondition {
      condition     = self.result.valid
      error_message = "The candidate guardrail is invalid."
    }
  }
}
```

An evaluation returning `valid = false` or `allowed = false` is not a transport
failure. Use a postcondition when that outcome must block the plan/apply. Results
are sensitive because identity, routing and execution metadata may be private.

LDAP and OIDC previews write reconciliation history on the server. They do not apply
role assignments. LDAP exposes a separate sensitive `preview_token`; LDAP apply
rechecks freshness and fails on changed mappings/directory data. The token is not a
one-time credential, and the provider neither renews it nor deletes server history.
OIDC has no public apply endpoint.

```hcl
ephemeral "semaphore_ex_ldap_group_preview" "review" {
  provider_id = "directory"
}

action "semaphore_ex_ldap_group_apply" "reviewed" {
  config {
    provider_id   = "directory"
    preview_token = ephemeral.semaphore_ex_ldap_group_preview.review.preview_token
  }
}

# Explicitly invokes apply using a freshly evaluated preview:
# terraform apply -invoke=action.semaphore_ex_ldap_group_apply.reviewed
```

The LDAP apply example above documents the intended wiring but is currently blocked
by the known framework defect. Neither an ephemeral token nor a separately supplied
token can make that Action execute with this framework version.

This example intentionally wires the current preview into an explicit apply. When
human review must happen separately, inspect the preview through an approved
operational workflow and pass its reviewed token as a sensitive ephemeral variable.
The provider never fetches execution preflight proof or approves it automatically.

Notification destination tests and retries can send real notifications. Audit
webhook tests attempt outbound HTTP. Workflow-trigger tests start real workflow
runs. LDAP reconcile immediately changes role assignments. These mutating operations
are Actions only; naming an endpoint `test` does not make it a read-only preview.

Docker/Kubernetes policy previews evaluate native request summaries without creating
containers or Jobs. Guardrail validate/test/impact and routing previews do not publish
or save configuration. Guardrail rollback is an explicit revision-fenced Action that
publishes a new revision; configure its reason and expected draft revision yourself.

Consult the [coverage matrix](provider-coverage.md#operational-api-mapping) and
individual generated Action/ephemeral reference pages for required inputs and scope.
