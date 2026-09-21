data "semaphore_ex_global_notification_destinations" "available" {
}

output "available_ids" {
  value = data.semaphore_ex_global_notification_destinations.available.ids
}
