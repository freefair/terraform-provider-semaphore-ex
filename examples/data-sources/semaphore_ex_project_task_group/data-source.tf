data "semaphore_ex_project_task_group" "state" {
  project_id = 2 # A project with an explicit grant can read the shared group.
  id         = 7
}
