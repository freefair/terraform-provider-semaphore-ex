resource "semaphore_ex_project" "project" {
  name = "Example Project"
}

data "semaphore_ex_project_template" "template" {
  project_id = semaphore_ex_project.project.id
  name       = "Example Template"
}

resource "semaphore_ex_project_schedule" "schedule" {
  project_id  = semaphore_ex_project.project.id
  template_id = data.semaphore_ex_project_template.template.id
  name        = "Example Schedule"
  cron_format = "0 0 * * *"
  enabled     = true
}
