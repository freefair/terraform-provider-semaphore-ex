action "semaphore_ex_project_notification_destination_test" "example" {
  config {
    project_id     = 1
    destination_id = 2
  }
}

# terraform apply -invoke=action.semaphore_ex_project_notification_destination_test.example
