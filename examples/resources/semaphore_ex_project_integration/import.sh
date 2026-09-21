# Import ID is specified by the string "project/{project_id}/integration/{integration_id}".
# - {project_id} is the ID of the project in SemaphoreUI.
# - {integration_id} is the ID of the integration in SemaphoreUI.
terraform import semaphore_ex_project_integration.open project/1/integration/2
```
Or using `import {}` block in the configuration file:
```hcl
import {
  to = semaphore_ex_project_integration.open
  id = "project/1/integration/2"
}
