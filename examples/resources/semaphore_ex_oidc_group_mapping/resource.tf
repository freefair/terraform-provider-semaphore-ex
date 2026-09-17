resource "semaphore_ex_oidc_group_mapping" "operators" {
  id          = "operators"
  provider_id = "corporate-oidc"
  claim_value = "operators"
  enabled     = true

  target = {
    scope   = "global"
    role_id = semaphore_ex_global_role.operator.id
  }
}
