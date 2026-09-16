# Lookup by View ID
data "semaphore_ex_project_view" "view" {
  project_id = 1
  id         = 3
}

# Lookup by View Name
data "semaphore_ex_project_view" "prod" {
  project_id = 1
  title      = "Prod"
}
