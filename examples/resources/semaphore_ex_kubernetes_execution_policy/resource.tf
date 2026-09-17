resource "semaphore_ex_kubernetes_execution_policy" "cluster" {
  cluster_alias              = "qa-cluster"
  allowed_namespaces         = ["semaphore-jobs"]
  allowed_images             = ["registry.example.test/job@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]
  allowed_service_accounts   = ["semaphore-task"]
  allowed_runtime_classes    = [""]
  runtime_class              = ""
  allowed_volume_types       = ["emptyDir", "secret"]
  allowed_network_profiles   = ["deny-all"]
  network_profile            = "deny-all"
  network_policy_enforcement = "network-policy"
  resources = {
    cpu_request_milli               = 100
    cpu_limit_milli                 = 500
    memory_request_bytes            = 67108864
    memory_limit_bytes              = 268435456
    ephemeral_storage_request_bytes = 67108864
    ephemeral_storage_limit_bytes   = 268435456
  }
  terminal_retention_seconds = 3600
}
