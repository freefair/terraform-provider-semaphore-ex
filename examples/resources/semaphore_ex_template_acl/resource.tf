resource "semaphore_ex_template_acl" "operator" {
  project_id  = semaphore_ex_project.example.id
  template_id = semaphore_ex_project_template.example.id
  role_slug   = "project_manager"

  allowed_permissions = ["template.read", "template.run"]
  denied_permissions  = ["template.delete"]
}
