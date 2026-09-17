resource "semaphore_ex_global_credential_grant" "application" {
  credential_id = semaphore_ex_global_credential.deployment_token.id
  project_id    = semaphore_ex_project.application.id
  operations    = ["reference", "consume"]
}
