action "semaphore_ex_project_workflow_retry_reconcile" "example" {
  config {
    project_id  = 1
    workflow_id = 2
    run_id      = 3
  }
}

# terraform apply -invoke=action.semaphore_ex_project_workflow_retry_reconcile.example
