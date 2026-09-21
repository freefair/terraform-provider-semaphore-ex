data "semaphore_ex_project_notification_destinations" "available" {
  project_id = 1
}

output "available_ids" {
  value = data.semaphore_ex_project_notification_destinations.available.ids
}
