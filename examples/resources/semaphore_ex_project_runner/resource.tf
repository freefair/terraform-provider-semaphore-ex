resource "semaphore_ex_project" "project" {
  name = "Example Project"
}

resource "semaphore_ex_project_runner" "runner" {
  project_id         = semaphore_ex_project.project.id
  name               = "Example Runner"
  max_parallel_tasks = 1
  active             = true
  tags               = ["linux", "production"]
}
