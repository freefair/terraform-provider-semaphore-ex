action "semaphore_ex_kubernetes_execution_policy_test" "example" {
  config {
    cluster_alias = "jobs"
    request       = { namespace = "jobs", task_image = "example/image", service_account = "runner", has_restricted_security_profile = true }
  }
}

# terraform apply -invoke=action.semaphore_ex_kubernetes_execution_policy_test.example
