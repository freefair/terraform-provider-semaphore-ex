# A workflow stop is asynchronous: it requests a stop and returns immediately.
action "semaphore_ex_project_workflow_stop" "stop_release" {
  config {
    project_id  = 1
    workflow_id = 4
    run_id      = 88
  }
}

# terraform apply -invoke=action.semaphore_ex_project_workflow_stop.stop_release
