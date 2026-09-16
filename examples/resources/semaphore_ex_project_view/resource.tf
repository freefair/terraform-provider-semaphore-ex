resource "semaphore_ex_project" "project" {
  name = "Example Project"
}

resource "semaphore_ex_project_view" "view" {
  project_id = semaphore_ex_project.project.id
  title      = "Section A"
  position   = 0
}
