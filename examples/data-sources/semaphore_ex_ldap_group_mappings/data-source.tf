data "semaphore_ex_ldap_group_mappings" "available" {
  provider_id = "identity-provider"
}

output "available_ids" {
  value = data.semaphore_ex_ldap_group_mappings.available.ids
}
