action "semaphore_ex_global_policy_guardrail_rollback" "example" {
  config {
    revision                = 1
    expected_draft_revision = 2
    reason                  = "Restore reviewed policy"
  }
}

# terraform apply -invoke=action.semaphore_ex_global_policy_guardrail_rollback.example
