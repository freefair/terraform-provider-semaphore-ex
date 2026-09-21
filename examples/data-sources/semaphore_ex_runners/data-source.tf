data "semaphore_ex_runners" "available" {
}

output "available_ids" {
  value = data.semaphore_ex_runners.available.ids
}
