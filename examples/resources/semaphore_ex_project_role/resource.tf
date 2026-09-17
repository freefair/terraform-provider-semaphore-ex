resource "semaphore_ex_project" "example" {
  name = "Role example"
}

resource "semaphore_ex_project_role" "viewer" {
  project_id          = semaphore_ex_project.example.id
  name                = "Viewer"
  project_permissions = ["project.resources.view"]
}
