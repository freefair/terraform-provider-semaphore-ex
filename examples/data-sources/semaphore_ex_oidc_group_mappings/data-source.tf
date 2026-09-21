data "semaphore_ex_oidc_group_mappings" "available" {
  provider_id = "identity-provider"
}

output "available_ids" {
  value = data.semaphore_ex_oidc_group_mappings.available.ids
}
