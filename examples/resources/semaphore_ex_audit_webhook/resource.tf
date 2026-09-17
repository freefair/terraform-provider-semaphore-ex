resource "semaphore_ex_audit_webhook" "audit" {
  endpoint = "https://audit.example.com/events"
  paused   = true

  # Each changed value explicitly requests one new one-time signing secret.
  signing_bootstrap_version = 1
  signing_stage_version     = 1
}

output "current_audit_webhook_signing_secret" {
  value     = semaphore_ex_audit_webhook.audit.current_signing_secret
  sensitive = true
}
