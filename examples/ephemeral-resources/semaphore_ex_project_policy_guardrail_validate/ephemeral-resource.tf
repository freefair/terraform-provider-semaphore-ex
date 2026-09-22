ephemeral "semaphore_ex_project_policy_guardrail_validate" "example" {
  project_id  = 1
  source_yaml = yamlencode({ version = 1, rules = [] })
}

# Reference ephemeral.semaphore_ex_project_policy_guardrail_validate.example.result in an ephemeral context.
