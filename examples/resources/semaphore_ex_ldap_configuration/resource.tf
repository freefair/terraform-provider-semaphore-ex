resource "semaphore_ex_ldap_configuration" "corporate" {
  id                       = "corporate"
  display_name             = "Corporate Directory"
  state                    = "disabled"
  server_url               = "ldaps://directory.example.test:636"
  tls_mode                 = "ldaps"
  trust_mode               = "system"
  bind_dn                  = "cn=semaphore,ou=service,dc=example,dc=test"
  bind_password            = var.ldap_bind_password
  bind_password_wo_version = 1
  search_base_dn           = "ou=people,dc=example,dc=test"
  user_filter              = "(uid={{username}})"
  identity_attribute       = "entryUUID"
  username_attribute       = "uid"
  name_attribute           = "cn"
  email_attribute          = "mail"
}
