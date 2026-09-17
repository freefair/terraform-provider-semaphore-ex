action "semaphore_ex_project_task_stop" "stop_deploy" {
  config {
    project_id = 1
    task_id    = 101
    force      = false
  }
}

# terraform apply -invoke=action.semaphore_ex_project_task_stop.stop_deploy
