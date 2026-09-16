resource "semaphore_ex_project" "project" {
  name = "Example Project"
}

resource "semaphore_ex_project_key" "none" {
  project_id = semaphore_ex_project.project.id
  name       = "None"
  none       = {}
}

# Static Inventory Example
resource "semaphore_ex_project_inventory" "static" {
  project_id = semaphore_ex_project.project.id
  name       = "Static Inventory"
  ssh_key_id = semaphore_ex_project_key.none.id
  static = {
    inventory = <<-EOT
      [website]
      172.18.8.40
      172.18.8.41
    EOT
    # Optional
    become_key_id = 3
  }
}

# Static YAML Inventory Example
resource "semaphore_ex_project_inventory" "static_yaml" {
  project_id = semaphore_ex_project.project.id
  name       = "Static YAML Inventory"
  ssh_key_id = semaphore_ex_project_key.none.id
  static_yaml = {
    inventory = yamlencode({
      all = {
        children = {
          website = {
            hosts = {
              "172.18.8.40" = {}
              "172.18.8.41" = {}
            }
          }
        }
      }
    })
  }
}

# File Inventory Example
resource "semaphore_ex_project_inventory" "file" {
  project_id = semaphore_ex_project.project.id
  name       = "Static Inventory"
  ssh_key_id = semaphore_ex_project_key.none.id
  file = {
    path = "inventory/dev/hosts"
    # Optional
    repository_id = 1
  }
}

# Terraform Workspace Inventory Example
resource "semaphore_ex_project_inventory" "terraform" {
  project_id = semaphore_ex_project.project.id
  name       = "Terraform Workspace Inventory"
  ssh_key_id = semaphore_ex_project_key.none.id
  terraform_workspace = {
    workspace = "workspace-name"
  }
}

# OpenTofu Workspace Inventory Example
resource "semaphore_ex_project_inventory" "tofu" {
  project_id = semaphore_ex_project.project.id
  name       = "OpenTofu Workspace Inventory"
  ssh_key_id = semaphore_ex_project_key.none.id
  tofu_workspace = {
    workspace = "workspace-name"
  }
}
