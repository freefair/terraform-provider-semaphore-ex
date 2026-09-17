resource "semaphore_ex_project_generated_ssh_key" "deploy" {
  project_id       = semaphore_ex_project.example.id
  name             = "deploy"
  login            = "deploy"
  login_wo_version = 1
  algorithm        = "ed25519"
}

# The private key remains encrypted on the Semaphore EX server and is never in Terraform state.
# Increment login_wo_version when changing login; login and algorithm replace this key.
# Use the explicit rotation action for an in-place key rotation.
