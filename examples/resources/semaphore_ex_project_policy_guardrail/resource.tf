resource "semaphore_ex_project_policy_guardrail" "configuration" {
  project_id = 1
  source_yaml = yamlencode({
    version = 1
    rules   = []
  })
}
