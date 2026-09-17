data "semaphore_ex_global_credential_grant" "application" {
  credential_id = semaphore_ex_global_credential.deployment_token.id
  id            = semaphore_ex_global_credential_grant.application.id
}
