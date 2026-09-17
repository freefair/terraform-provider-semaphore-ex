resource "semaphore_ex_docker_execution_policy" "default" {
  allowed_images        = ["registry.example.test/job@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]
  require_digest        = true
  allowed_networks      = ["none"]
  network               = "none"
  user                  = "65534:0"
  nano_cpus             = 1000000000
  memory_bytes          = 536870912
  pids_limit            = 256
  pull_timeout_seconds  = 300
  max_image_size_bytes  = 2147483648
  seccomp_profile       = "default"
  apparmor_profile      = "docker-default"
  allow_privileged      = false
  allow_bind_mounts     = false
  allow_devices         = false
  allow_host_namespaces = false
}
