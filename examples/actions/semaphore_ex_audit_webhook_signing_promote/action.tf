action "semaphore_ex_audit_webhook_signing_promote" "rotate" {
  config {
    expected_signing_revision = semaphore_ex_audit_webhook.audit.signing_revision
  }
}
