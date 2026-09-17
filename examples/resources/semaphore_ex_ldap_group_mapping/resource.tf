resource "semaphore_ex_ldap_group_mapping" "operators" {
  id                = "operators"
  provider_id       = "corporate"
  group_external_id = "entryuuid:12345678-1234-1234-1234-123456789abc"
  enabled           = true

  target = {
    scope   = "global"
    role_id = semaphore_ex_global_role.operator.id
  }
}
