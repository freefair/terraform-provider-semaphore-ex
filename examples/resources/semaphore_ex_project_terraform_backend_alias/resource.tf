resource "semaphore_ex_project_terraform_backend_alias" "state" {
  project_id   = 1
  inventory_id = 2
  auth_key_id  = 3
}

# Configure Terraform's HTTP backend with this alias and the selected
# login-password Access Key's login/password. Keep the password in an
# environment variable such as TF_HTTP_PASSWORD, never in this file.
