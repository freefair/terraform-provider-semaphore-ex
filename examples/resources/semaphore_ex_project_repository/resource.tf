resource "semaphore_ex_project" "project" {
  name = "Example Project"
}

resource "semaphore_ex_project_key" "none" {
  project_id = semaphore_ex_project.project.id
  name       = "None"
  none       = {}
}

resource "semaphore_ex_project_repository" "repository" {
  project_id = semaphore_ex_project.project.id
  name       = "Example Repository"
  url        = "https://github.com/semaphoreui/semaphore.git"
  branch     = "develop"
  ssh_key_id = semaphore_ex_project_key.none.id
}
