# The server authorizes this decision against the immutable approval snapshot.
action "semaphore_ex_project_workflow_approval" "approve_release" {
  config {
    project_id  = 1
    workflow_id = 4
    run_id      = 88
    node_id     = 19
    status      = "approved"
    comment     = "Reviewed in change window."
  }
}

# terraform apply -invoke=action.semaphore_ex_project_workflow_approval.approve_release
