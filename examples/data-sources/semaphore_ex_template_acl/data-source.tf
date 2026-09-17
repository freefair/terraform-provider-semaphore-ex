data "semaphore_ex_template_acl" "operator" {
  project_id  = semaphore_ex_project.example.id
  template_id = semaphore_ex_project_template.example.id
  id          = semaphore_ex_template_acl.operator.id
}
