# The owner creates this grant. Setting accepted requires the configured
# provider identity to manage both the owner and consumer projects.
resource "semaphore_ex_cross_project_grant" "release_template" {
  owner_project_id    = semaphore_ex_project.owner.id
  consumer_project_id = semaphore_ex_project.consumer.id
  template_id         = semaphore_ex_project_template.release.id

  min_version = 1
  max_version = 3
  operations  = ["reference", "run"]
  reason      = "Allow the release workflow to use approved template versions."
  accepted    = true
}
