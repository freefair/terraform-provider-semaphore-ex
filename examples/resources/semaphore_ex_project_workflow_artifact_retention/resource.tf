resource "semaphore_ex_project_workflow_artifact_retention" "configuration" {
  project_id         = 1
  retention_seconds  = 3600
  max_artifact_bytes = 1048576
  max_run_bytes      = 2097152
}
