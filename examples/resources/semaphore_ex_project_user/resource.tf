resource "semaphore_ex_project" "project" {
  name = "Example Project"
}

resource "semaphore_ex_user" "user" {
  name     = "Example User"
  username = "example"
  email    = "user@example.com"
}

resource "semaphore_ex_project_user" "project_user" {
  project_id = semaphore_ex_project.project.id
  user_id    = semaphore_ex_user.user.id
  role       = "owner"
}
