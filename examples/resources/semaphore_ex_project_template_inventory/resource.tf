resource "semaphore_ex_project_template_inventory" "extra" {
  project_id   = semaphore_ex_project.example.id
  template_id  = semaphore_ex_project_template.example.id
  inventory_id = semaphore_ex_project_inventory.extra.id
}
