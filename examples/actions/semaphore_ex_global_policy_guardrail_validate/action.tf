action "semaphore_ex_global_policy_guardrail_validate" "example" {
  config {
    source_yaml = yamlencode({ version = 1, rules = [] })
  }
}

# terraform apply -invoke=action.semaphore_ex_global_policy_guardrail_validate.example
