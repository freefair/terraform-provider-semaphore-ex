resource "semaphore_ex_project_notification_rule" "critical" {
  project_id        = semaphore_ex_project.example.id
  destination_id    = semaphore_ex_project_notification_destination.operations.id
  source_kinds      = ["task", "workflow"]
  lifecycle_actions = ["trigger", "update", "resolve"]
  minimum_severity  = "critical"
}
