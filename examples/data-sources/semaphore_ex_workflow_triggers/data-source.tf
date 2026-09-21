data "semaphore_ex_workflow_triggers" "available" {
  project_id  = 1
  workflow_id = 2
}

output "available_ids" {
  value = data.semaphore_ex_workflow_triggers.available.ids
}
