# Explicitly restore a reviewed snapshot. The server records a new revision.
# Reconcile the workflow resource configuration afterwards to avoid reverting it.
action "semaphore_ex_project_workflow_restore" "release" {
  config {
    project_id     = 1
    workflow_id    = 4
    version_number = 2
    message        = "Restore reviewed workflow definition"
  }
}
