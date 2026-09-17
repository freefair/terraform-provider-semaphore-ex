resource "semaphore_ex_global_notification_rule" "critical" {
  destination_id    = semaphore_ex_global_notification_destination.operations.id
  source_kinds      = ["task", "workflow"]
  lifecycle_actions = ["trigger", "update", "resolve"]
  minimum_severity  = "critical"
}
