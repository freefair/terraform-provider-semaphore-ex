variable "group_claim" {
  type      = list(string)
  sensitive = true
  ephemeral = true
}

ephemeral "semaphore_ex_oidc_group_preview" "example" {
  provider_id = "oidc"
  user_id     = 1
  claim       = var.group_claim
}

# Reference ephemeral.semaphore_ex_oidc_group_preview.example.result in an ephemeral context.
