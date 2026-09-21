data "semaphore_ex_project_environment" "environment" {
  project_id = 1
  id         = 4
}

# JSON outputs retain numbers, booleans, nulls, lists and nested objects.
locals {
  environment_variables = jsondecode(data.semaphore_ex_project_environment.environment.variables_json)
}

# The name must be unique in this project. Configure name or id, not both.
data "semaphore_ex_project_environment" "by_name" {
  project_id = 1
  name       = "production"
}
