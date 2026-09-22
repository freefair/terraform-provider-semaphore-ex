action "semaphore_ex_global_notification_destination_test" "example" {
  config {
    destination_id = 2
  }
}

# terraform apply -invoke=action.semaphore_ex_global_notification_destination_test.example
