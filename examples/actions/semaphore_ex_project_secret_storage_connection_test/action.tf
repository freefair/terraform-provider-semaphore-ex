action "semaphore_ex_project_secret_storage_connection_test" "vault" {
  config {
    project_id = 1
    storage_id = 2
  }
}

# terraform apply -invoke=action.semaphore_ex_project_secret_storage_connection_test.vault
