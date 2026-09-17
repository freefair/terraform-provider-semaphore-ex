action "semaphore_ex_project_workflow_start" "release" {
  config {
    project_id  = 1
    workflow_id = 4
    parameters  = { release = "2026.09.16", canary_percent = 10, verify = true }
    node_overrides = {
      "12" = { inventory_id = 3, git_branch = "main" }
    }
  }
}

# terraform apply -invoke=action.semaphore_ex_project_workflow_start.release
