# Lookup by Integration ID
data "semaphore_ex_project_integration" "by_id" {
  project_id = 1
  id         = 2
}

# Lookup by Integration Name
data "semaphore_ex_project_integration" "by_name" {
  project_id = 1
  name       = "github-webhook"
}
