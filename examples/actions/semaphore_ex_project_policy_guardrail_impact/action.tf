action "semaphore_ex_project_policy_guardrail_impact" "example" {
  config {
    project_id = 1
    inputs     = [{ project_id = 1, intent = "task", evaluated_at = "2026-09-21T12:00:00Z", template = { id = 2, application = "ansible", source = "task" }, executor = { type = "local", image_reference_kind = "none" } }]
  }
}

# terraform apply -invoke=action.semaphore_ex_project_policy_guardrail_impact.example
