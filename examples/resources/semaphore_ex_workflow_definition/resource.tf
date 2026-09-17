resource "semaphore_ex_workflow_definition" "release" {
  project_id = 1
  name       = "Release workflow"

  parameters = [{ name = "release", type = "string", default_string = "main" }]
  # Keys are persisted as unique node display names and are recovered on import.
  nodes = [
    { key = "Build", template_id = 2, position_x = 0, position_y = 0 },
    { key = "Deploy", template_id = 3, position_x = 300, position_y = 0 },
  ]
  edges = [{ source_key = "Build", destination_key = "Deploy", condition = "on_success" }]
}
