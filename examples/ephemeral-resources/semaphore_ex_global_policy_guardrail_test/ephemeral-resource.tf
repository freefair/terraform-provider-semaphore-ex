ephemeral "semaphore_ex_global_policy_guardrail_test" "example" {
  source_yaml = yamlencode({ version = 1, rules = [] })
  input       = { project_id = 1, intent = "task", evaluated_at = "2026-09-21T12:00:00Z", template = { id = 2, application = "ansible", source = "task" }, executor = { type = "local", image_reference_kind = "none" } }
}

# Reference ephemeral.semaphore_ex_global_policy_guardrail_test.example.result in an ephemeral context.
