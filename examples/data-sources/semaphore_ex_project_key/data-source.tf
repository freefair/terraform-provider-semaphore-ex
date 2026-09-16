# Lookup by Key ID
data "semaphore_ex_project_key" "key" {
  project_id = 1
  id         = 3
}

# Lookup by Key Name
data "semaphore_ex_project_key" "none" {
  project_id = 1
  name       = "None"
}
