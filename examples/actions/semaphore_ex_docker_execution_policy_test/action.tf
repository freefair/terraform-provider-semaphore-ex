action "semaphore_ex_docker_execution_policy_test" "example" {
  config {
    request = { image = "example/image", privileged = false }
  }
}

# terraform apply -invoke=action.semaphore_ex_docker_execution_policy_test.example
