data "semaphore_ex_project_keys" "available" {
  project_id = 1
}

output "available_ids" {
  value = data.semaphore_ex_project_keys.available.ids
}
