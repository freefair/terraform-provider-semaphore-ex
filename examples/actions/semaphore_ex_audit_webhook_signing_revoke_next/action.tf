action "semaphore_ex_audit_webhook_signing_revoke_next" "cancel" {
  config {
    expected_signing_revision = semaphore_ex_audit_webhook.audit.signing_revision
  }
}
