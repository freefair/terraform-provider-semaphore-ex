data "semaphore_ex_project_secret_storages" "available" {
  project_id = 1
}

output "available_ids" {
  value = data.semaphore_ex_project_secret_storages.available.ids
}
