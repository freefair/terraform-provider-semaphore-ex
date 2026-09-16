resource "semaphore_ex_project" "project" {
  name = "Example Project"
}

resource "semaphore_ex_project_key" "none" {
  project_id = semaphore_ex_project.project.id
  name       = "None"
  none       = {}
}

resource "semaphore_ex_project_repository" "repository" {
  project_id = semaphore_ex_project.project.id
  name       = "Example Repository"
  url        = "https://github.com/semaphoreui/semaphore.git"
  branch     = "develop"
  ssh_key_id = semaphore_ex_project_key.none.id
}

resource "semaphore_ex_project_inventory" "inventory" {
  project_id = semaphore_ex_project.project.id
  name       = "Example Inventory"
  ssh_key_id = semaphore_ex_project_key.none.id
  static = {
    inventory = "localhost ansible_connection=local"
  }
}

resource "semaphore_ex_project_template" "deploy" {
  project_id    = semaphore_ex_project.project.id
  inventory_id  = semaphore_ex_project_inventory.inventory.id
  repository_id = semaphore_ex_project_repository.repository.id
  name          = "Deploy"
  playbook      = "deploy.yml"
}

resource "semaphore_ex_project_integration" "deploy" {
  project_id  = semaphore_ex_project.project.id
  template_id = semaphore_ex_project_template.deploy.id
  name        = "deploy-webhook"
}

# Integration-scoped alias — incoming requests trigger this integration's
# template directly. Most common form; emit `url` to share with the upstream
# webhook caller (GitHub, etc.).
resource "semaphore_ex_integration_alias" "deploy" {
  project_id     = semaphore_ex_project.project.id
  integration_id = semaphore_ex_project_integration.deploy.id
}

output "deploy_webhook_url" {
  value = semaphore_ex_integration_alias.deploy.url
}

# Project-scoped alias — omit integration_id. Incoming requests are routed to
# an integration via matchers defined on each integration. Useful when one
# entry-point URL fans out to multiple integrations based on payload.
resource "semaphore_ex_integration_alias" "router" {
  project_id = semaphore_ex_project.project.id
}
