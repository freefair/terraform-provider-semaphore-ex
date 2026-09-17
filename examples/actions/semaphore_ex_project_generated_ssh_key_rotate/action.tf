# Rotation replaces the existing generated SSH key and invalidates its private material.
action "semaphore_ex_project_generated_ssh_key_rotate" "deploy" {
  config {
    project_id       = 1
    key_id           = 4
    algorithm        = "ed25519"
    confirm_rotation = true
  }
}

# terraform apply -invoke=action.semaphore_ex_project_generated_ssh_key_rotate.deploy
