data "semaphore_ex_cross_project_grant" "release_template" {
  owner_project_id = semaphore_ex_project.owner.id
  id               = semaphore_ex_cross_project_grant.release_template.id
}
