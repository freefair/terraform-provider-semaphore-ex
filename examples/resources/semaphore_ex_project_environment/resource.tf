resource "semaphore_ex_project" "project" {
  name = "Example Project"
}

resource "semaphore_ex_project_environment" "environment" {
  project_id = semaphore_ex_project.project.id
  name       = "Example Environment"

  # extraVars
  variables = {
    key1 = "value1"
    key2 = "value2"
  }

  # environment variables
  environment = {
    KEY1 = "value1"
    KEY2 = "value2"
  }

  # Omit this block after importing an existing environment to retain its
  # storage binding. An empty block explicitly clears that binding.
  secret_storage = {
    id         = 1
    key_prefix = "terraform-"
  }

  # secrets
  secrets = [{
    # extraVar Secret
    name  = "key3"
    type  = "var"
    value = "value3"
    }, {
    # environment Secret
    name  = "KEY4"
    type  = "env"
    value = "value4"
    }, {
    # Remote runtime secret; do not set value when storage_id is present.
    name       = "REMOTE_TOKEN"
    type       = "env"
    storage_id = 1
    mount      = "secret"
    path       = "applications/example"
    version    = 0
    field      = "token"
  }]

  sync_enabled  = true
  sync_interval = 30
  sync_paths = [{
    access_key_id = 1
    mount         = "secret"
    path          = "applications/example"
    field         = "token"
    prefix        = "EXAMPLE_"
    separator     = "_"
  }]
}

# Use JSON inputs for values that are not string maps.
resource "semaphore_ex_project_environment" "typed" {
  project_id = semaphore_ex_project.project.id
  name       = "Typed variables"
  variables_json = jsonencode({
    retries = 3
    enabled = true
    targets = ["web", "worker"]
    limits  = { memory_mb = 512 }
  })
  environment_json = jsonencode({ LOG_LEVEL = "info", RETRIES = 3 })
}
