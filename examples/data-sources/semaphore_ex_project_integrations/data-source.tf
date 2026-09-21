data "semaphore_ex_project_integrations" "available" {
  project_id = 1
}

output "available_ids" {
  value = data.semaphore_ex_project_integrations.available.ids
}
