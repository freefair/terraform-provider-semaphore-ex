variable "preflight_token" {
  type      = string
  sensitive = true
  ephemeral = true
}

# The fingerprint and token come from a separately performed preflight review.
# This action does not obtain preflight proof or wait for the task to complete.
action "semaphore_ex_project_task_start" "deploy" {
  config {
    project_id            = 1
    template_id           = 2
    environment           = { release = "2026.09.16", maintenance = false }
    arguments             = ["--limit", "web"]
    preflight_fingerprint = "reviewed-fingerprint"
    preflight_token       = var.preflight_token
  }
}

# terraform apply -invoke=action.semaphore_ex_project_task_start.deploy

action "semaphore_ex_project_task_stop" "stop_deploy" {
  config {
    project_id = 1
    task_id    = 101
    force      = false
  }
}

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

# A workflow stop is asynchronous: it requests a stop and returns immediately.
action "semaphore_ex_project_workflow_stop" "stop_release" {
  config {
    project_id  = 1
    workflow_id = 4
    run_id      = 88
  }
}

# The server authorizes this decision against the immutable approval snapshot.
action "semaphore_ex_project_workflow_approval" "approve_release" {
  config {
    project_id  = 1
    workflow_id = 4
    run_id      = 88
    node_id     = 19
    status      = "approved"
    comment     = "Reviewed in change window."
  }
}
