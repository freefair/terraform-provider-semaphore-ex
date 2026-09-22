resource "semaphore_ex_project_task_group" "state" {
  project_id         = 1
  name               = "Production state"
  description        = "Serialize every template using the same Terraform state."
  max_parallel_tasks = 1
  runner_ids         = [4, 5]
  shared_project_ids = [2]
}

# Select the group on each participating template:
# task_groups = [semaphore_ex_project_task_group.state.id]
