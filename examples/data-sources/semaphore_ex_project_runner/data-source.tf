# Look up a project runner by ID.
data "semaphore_ex_project_runner" "by_id" {
  project_id = 1
  id         = 2
}

# Look up a project runner by name.
data "semaphore_ex_project_runner" "by_name" {
  project_id = 1
  name       = "Example Runner"
}
