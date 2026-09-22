action "semaphore_ex_project_deployment_window_preview" "example" {
  config {
    project_id = 1
    policy     = { revision = 1, timezone = "UTC", default = "allow", rules = [], template_id = 2 }
  }
}

# terraform apply -invoke=action.semaphore_ex_project_deployment_window_preview.example
