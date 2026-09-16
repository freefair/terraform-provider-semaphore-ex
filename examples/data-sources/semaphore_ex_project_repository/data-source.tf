# Lookup by Repository ID
data "semaphore_ex_project_repository" "repo" {
  project_id = 1
  id         = 3
}

# Lookup by Repository Name
data "semaphore_ex_project_repository" "semaphore" {
  project_id = 1
  name       = "Semaphore"
}
