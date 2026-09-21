# Import ID is specified by the string "project/{project_id}/repository/{repository_id}".
# - {project_id} is the ID of the project in SemaphoreUI.
# - {repository_id} is the ID of the repository in SemaphoreUI.
terraform import semaphore_ex_project_repository.repository project/1/repository/2
```
Or using `import {}` block in the configuration file:
```hcl
import {
  to = semaphore_ex_project_repository.repository
  id = "project/1/repository/2"
}
