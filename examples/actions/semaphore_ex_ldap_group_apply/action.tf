variable "preview_token" {
  type      = string
  sensitive = true
  ephemeral = true
}

action "semaphore_ex_ldap_group_apply" "example" {
  config {
    provider_id   = "directory"
    preview_token = var.preview_token
  }
}

# terraform apply -invoke=action.semaphore_ex_ldap_group_apply.example
