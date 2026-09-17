resource "semaphore_ex_project_notification_destination" "operations" {
  project_id            = semaphore_ex_project.example.id
  name                  = "operations"
  destination_type      = "pagerduty"
  environment           = "production"
  region                = "eu"
  credential_wo         = var.pagerduty_routing_key
  credential_wo_version = 1
}
