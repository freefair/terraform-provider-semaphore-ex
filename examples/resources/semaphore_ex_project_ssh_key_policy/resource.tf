resource "semaphore_ex_project" "dependencies" {
  name = "Private dependencies"
}

resource "semaphore_ex_project_generated_ssh_key" "dependencies" {
  project_id       = semaphore_ex_project.dependencies.id
  name             = "Dependency repositories"
  login            = "git"
  login_wo_version = 1
  algorithm        = "ed25519"
}

resource "semaphore_ex_project_ssh_key_policy" "dependencies" {
  project_id = semaphore_ex_project.dependencies.id
  default_ssh_keys = [{
    access_key_id = semaphore_ex_project_generated_ssh_key.dependencies.id
    hosts         = ["github.com", "gitlab.com"]
  }]
  always_ssh_keys = []
}

# Authorize the generated public key on your Git hosts.
# The separate policy allows keys and project to be created in the same apply.
