ephemeral "semaphore_ex_kubernetes_execution_policy_test" "example" {
  cluster_alias = "jobs"
  request       = { namespace = "jobs", task_image = "example/image", service_account = "runner", has_restricted_security_profile = true }
}

# Reference ephemeral.semaphore_ex_kubernetes_execution_policy_test.example.result in an ephemeral context.
