action "semaphore_ex_global_notification_delivery_retry" "example" {
  config {
    delivery_id = 2
  }
}

# terraform apply -invoke=action.semaphore_ex_global_notification_delivery_retry.example
