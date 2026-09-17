resource "semaphore_ex_global_notification_destination" "operations" {
  name                  = "operations"
  destination_type      = "pagerduty"
  environment           = "production"
  region                = "eu"
  credential_wo         = var.pagerduty_routing_key
  credential_wo_version = 1
}
