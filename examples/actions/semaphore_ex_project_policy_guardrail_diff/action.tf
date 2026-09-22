action "semaphore_ex_project_policy_guardrail_diff" "example" {
  config {
    project_id    = 1
    from_revision = 1
    to_revision   = 2
  }
}

# terraform apply -invoke=action.semaphore_ex_project_policy_guardrail_diff.example
