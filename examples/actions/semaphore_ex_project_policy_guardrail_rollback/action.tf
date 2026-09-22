action "semaphore_ex_project_policy_guardrail_rollback" "example" {
  config {
    project_id              = 1
    revision                = 1
    expected_draft_revision = 2
    reason                  = "Restore reviewed policy"
  }
}

# terraform apply -invoke=action.semaphore_ex_project_policy_guardrail_rollback.example
