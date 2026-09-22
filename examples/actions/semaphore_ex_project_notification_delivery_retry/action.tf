action "semaphore_ex_project_notification_delivery_retry" "example" {
  config {
    project_id  = 1
    delivery_id = 2
  }
}

# terraform apply -invoke=action.semaphore_ex_project_notification_delivery_retry.example
