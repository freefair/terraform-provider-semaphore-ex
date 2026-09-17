action "semaphore_ex_project_template_publish" "release" {
  config {
    project_id  = 1
    template_id = 2
  }
}

# Publish explicitly with:
# terraform apply -invoke=action.semaphore_ex_project_template_publish.release
