action "semaphore_ex_project_secret_storage_sync" "outbound" {
  config {
    project_id           = 1
    storage_id           = 2
    request_id           = "terraform:secret-sync-20260916"
    resolve_operation_id = 17
  }
}

# terraform apply -invoke=action.semaphore_ex_project_secret_storage_sync.outbound
