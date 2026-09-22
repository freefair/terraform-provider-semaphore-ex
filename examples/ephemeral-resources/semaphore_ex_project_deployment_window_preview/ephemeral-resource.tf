ephemeral "semaphore_ex_project_deployment_window_preview" "example" {
  project_id = 1
  policy     = { revision = 1, timezone = "UTC", default = "allow", rules = [], template_id = 2 }
}

# Reference ephemeral.semaphore_ex_project_deployment_window_preview.example.result in an ephemeral context.
