action "semaphore_ex_global_notification_routing_preview" "example" {
  config {
    event = {
      source_revision  = 1
      source           = { kind = "task", id = "task:12" }
      lifecycle_id     = "template:7"
      severity         = "error"
      lifecycle_action = "trigger"
      details          = { task_id = 12, template_id = 7, status = "failed" }
    }
  }
}

# terraform apply -invoke=action.semaphore_ex_global_notification_routing_preview.example
