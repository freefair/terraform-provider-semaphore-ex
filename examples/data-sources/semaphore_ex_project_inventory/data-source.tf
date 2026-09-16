# Lookup by Inventory ID
data "semaphore_ex_project_inventory" "inventory" {
  project_id = 1
  id         = 2
}

# Lookup by Inventory Name
data "semaphore_ex_project_inventory" "example" {
  project_id = 1
  name       = "Example Invewntory"
}
