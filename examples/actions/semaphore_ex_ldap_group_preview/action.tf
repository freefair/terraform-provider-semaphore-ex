action "semaphore_ex_ldap_group_preview" "example" {
  config {
    provider_id = "directory"
  }
}

# terraform apply -invoke=action.semaphore_ex_ldap_group_preview.example
