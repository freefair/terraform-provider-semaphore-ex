action "semaphore_ex_workflow_trigger_test" "example" {
  config {
    project_id  = 1
    workflow_id = 2
    trigger_id  = 3
    inputs      = { release = "example" }
  }
}

# terraform apply -invoke=action.semaphore_ex_workflow_trigger_test.example
