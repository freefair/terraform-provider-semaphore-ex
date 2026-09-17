data "semaphore_ex_project_terraform_backend_alias" "state" {
  project_id   = 1
  inventory_id = 2
  id           = "opaque-alias-id"
}
