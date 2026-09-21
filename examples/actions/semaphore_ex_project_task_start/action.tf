variable "preflight_token" {
  type      = string
  sensitive = true
  ephemeral = true
}

variable "survey_secrets" {
  type      = map(string)
  sensitive = true
  ephemeral = true
  default   = {}
}

action "semaphore_ex_project_task_start" "deploy" {
  config {
    project_id  = 1
    template_id = 2
    # Select no non-always keys for this run; omit to inherit.
    ssh_keys    = []
    environment = { release = "2026.09.16", maintenance = false }
    # The template must allow a limit override.
    params                = { limit = ["web"] }
    secret                = var.survey_secrets
    preflight_fingerprint = "reviewed-fingerprint"
    preflight_token       = var.preflight_token
  }
}

# terraform apply -invoke=action.semaphore_ex_project_task_start.deploy
