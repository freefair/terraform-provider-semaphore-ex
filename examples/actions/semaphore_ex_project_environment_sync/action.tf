action "semaphore_ex_project_environment_sync" "production" {
  config {
    project_id     = 1
    environment_id = 3
  }
}

# terraform apply -invoke=action.semaphore_ex_project_environment_sync.production
