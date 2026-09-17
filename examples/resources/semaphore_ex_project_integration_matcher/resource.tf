resource "semaphore_ex_project_integration_matcher" "branch" {
  project_id     = semaphore_ex_project.example.id
  integration_id = semaphore_ex_project_integration.example.id
  name           = "Main branch only"
  match_type     = "body"
  body_data_type = "json"
  key            = "ref"
  method         = "equals"
  value          = "refs/heads/main"
}
