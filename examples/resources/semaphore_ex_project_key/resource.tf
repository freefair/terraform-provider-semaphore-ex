resource "semaphore_ex_project" "project" {
  name = "Example Project"
}

resource "semaphore_ex_project_key" "login_password" {
  project_id = semaphore_ex_project.project.id
  name       = "Example Login"
  login_password = {
    login    = "username"
    password = "password"
  }
}

resource "semaphore_ex_project_key" "ssh" {
  project_id = semaphore_ex_project.project.id
  name       = "Example SSH"
  ssh = {
    passphrase  = "password"
    private_key = file("./id_rsa")
  }
}

resource "semaphore_ex_project_key" "none" {
  project_id = semaphore_ex_project.project.id
  name       = "Example None"
  none       = {}
}

# A string key can hold an arbitrary secret value for integrations and secret-backed variables.
resource "semaphore_ex_project_key" "string" {
  project_id = 1
  name       = "integration token"

  string = {
    value = "replace-with-a-sensitive-value"
  }
}

# A remote reference stores only the reference metadata in Terraform state.
# `storage_id` must name a project secret storage when storage_type is "vault".
resource "semaphore_ex_project_key" "remote_string" {
  project_id = 1
  name       = "remote integration token"

  string = {}

  remote_reference = {
    storage_type = "vault"
    storage_id   = 1
    mount        = "secret"
    path         = "integrations/example"
    field        = "token"
  }
}

# Write-only / ephemeral secrets — for SSH keys or passwords fetched from a
# secret store like Vault. The `*_wo` values are sent to SemaphoreUI on apply
# but never persisted to Terraform state. Bump the matching `_wo_version` to
# rotate the secret.
ephemeral "vault_kv_secret_v2" "deploy_key" {
  mount = "secret"
  name  = "deploy-ssh-key"
}

resource "semaphore_ex_project_key" "ephemeral_ssh" {
  project_id = semaphore_ex_project.project.id
  name       = "Ephemeral SSH"
  ssh = {
    login                  = "deploy"
    private_key_wo         = ephemeral.vault_kv_secret_v2.deploy_key.data["private_key"]
    private_key_wo_version = 1 # bump to push a rotated key
  }
}
