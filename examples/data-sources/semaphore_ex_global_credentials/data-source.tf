data "semaphore_ex_global_credentials" "available" {
}

output "available_ids" {
  value = data.semaphore_ex_global_credentials.available.ids
}
