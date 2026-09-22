action "semaphore_ex_project_policy_guardrail_validate" "example" {
  config {
    project_id  = 1
    source_yaml = yamlencode({ version = 1, rules = [] })
  }
}

# terraform apply -invoke=action.semaphore_ex_project_policy_guardrail_validate.example
