action "semaphore_ex_audit_webhook_test" "example" {
  config {
    key = "current"
  }
}

# terraform apply -invoke=action.semaphore_ex_audit_webhook_test.example
