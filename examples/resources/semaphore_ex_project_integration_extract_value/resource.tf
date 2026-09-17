resource "semaphore_ex_project_integration_extract_value" "commit" {
  project_id     = semaphore_ex_project.example.id
  integration_id = semaphore_ex_project_integration.example.id
  name           = "Commit to deploy"
  value_source   = "body"
  body_data_type = "json"
  key            = "after"
  variable       = "GIT_COMMIT"
  variable_type  = "environment"
}
