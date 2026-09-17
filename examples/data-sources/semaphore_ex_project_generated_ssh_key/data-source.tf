data "semaphore_ex_project_generated_ssh_key" "deploy" {
  project_id = 1
  id         = 4
}

# Public metadata is available through public_key, fingerprint, and algorithm.
