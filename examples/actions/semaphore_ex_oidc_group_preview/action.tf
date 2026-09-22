variable "group_claim" {
  type      = list(string)
  sensitive = true
  ephemeral = true
}

action "semaphore_ex_oidc_group_preview" "example" {
  config {
    provider_id = "oidc"
    user_id     = 1
    claim       = var.group_claim
  }
}

# terraform apply -invoke=action.semaphore_ex_oidc_group_preview.example
