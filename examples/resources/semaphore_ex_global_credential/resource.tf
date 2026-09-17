resource "semaphore_ex_global_credential" "deployment_token" {
  type             = "string"
  display_name     = "Deployment token"
  value_wo         = var.deployment_token
  value_wo_version = 1
}
