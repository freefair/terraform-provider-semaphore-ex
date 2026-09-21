# Import ID is specified by the string "runner/{runner_id}".
# - {runner_id} is the ID of the global runner in SemaphoreUI.
terraform import semaphore_ex_runner.runner runner/1
```
Or using `import {}` block in the configuration file:
```hcl
import {
  to = semaphore_ex_runner.runner
  id = "runner/1"
}
