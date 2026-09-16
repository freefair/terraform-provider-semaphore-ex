# Lookup by Project ID
data "semaphore_ex_project" "project" {
  id = 1
}

# Lookup by Project Name
data "semaphore_ex_project" "example" {
  name = "Example Project"
}
