action "semaphore_ex_project_task_confirm" "example" {
  config {
    project_id = 1
    task_id    = 2
  }
}

# terraform apply -invoke=action.semaphore_ex_project_task_confirm.example
