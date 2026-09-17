data "semaphore_ex_project_role" "viewer" {
  project_id = semaphore_ex_project.example.id
  id         = semaphore_ex_project_role.viewer.id
}
