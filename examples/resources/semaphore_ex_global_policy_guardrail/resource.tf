resource "semaphore_ex_global_policy_guardrail" "configuration" {
  source_yaml = yamlencode({
    version = 1
    rules   = []
  })
}
