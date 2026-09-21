data "semaphore_ex_global_roles" "available" {
}

output "available_ids" {
  value = data.semaphore_ex_global_roles.available.ids
}
