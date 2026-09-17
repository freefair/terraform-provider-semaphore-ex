resource "semaphore_ex_workflow_trigger" "release" {
  project_id  = 1
  workflow_id = 42
  name        = "release-api"
  type        = "api"

  input_mappings = [
    {
      parameter    = "region"
      source       = "request"
      key          = "target_region"
      fixed_string = null
    },
    {
      parameter     = "confirmed"
      source        = "fixed"
      fixed_boolean = true
    },
  ]
}
