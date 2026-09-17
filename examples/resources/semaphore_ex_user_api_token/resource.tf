resource "semaphore_ex_user_api_token" "automation" {
  name = "terraform-automation"

  # Change this value, or use terraform apply -replace, to rotate the token.
  keepers = {
    rotation = "2026-09"
  }
}

output "automation_api_credential" {
  value     = semaphore_ex_user_api_token.automation.credential
  sensitive = true
}
