data "semaphore_ex_project_inventories" "available" {
  project_id = 1
}

output "available_ids" {
  value = data.semaphore_ex_project_inventories.available.ids
}
