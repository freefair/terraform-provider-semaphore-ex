variable "preflight_token" {
  type      = string
  sensitive = true
  ephemeral = true
}

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
