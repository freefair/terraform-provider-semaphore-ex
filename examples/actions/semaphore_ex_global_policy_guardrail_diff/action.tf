action "semaphore_ex_global_policy_guardrail_diff" "example" {
  config {
    from_revision = 1
    to_revision   = 2
  }
}

# terraform apply -invoke=action.semaphore_ex_global_policy_guardrail_diff.example
