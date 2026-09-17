variable "directory_test_password" {
  type      = string
  sensitive = true
  ephemeral = true
}

variable "local_recovery_password" {
  type      = string
  sensitive = true
  ephemeral = true
}

# Configure LDAP in disabled or shadow state first. Invoke this action
# explicitly, then apply the requested active or selected_users state.
action "semaphore_ex_ldap_test" "verify" {
  config {
    provider_id             = "corporate"
    username                = "directory-test-user"
    password                = var.directory_test_password
    recovery_admin_user_id  = 1
    recovery_admin_password = var.local_recovery_password
  }
}
