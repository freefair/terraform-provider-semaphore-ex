# Integration-scoped alias:
# Import ID is "project/{project_id}/integration/{integration_id}/alias/{alias_id}".
terraform import semaphore_ex_integration_alias.deploy project/1/integration/2/alias/3

# Project-scoped alias (no integration):
# Import ID is "project/{project_id}/alias/{alias_id}".
terraform import semaphore_ex_integration_alias.deploy project/1/alias/3
```
Or using `import {}` block in the configuration file:
```hcl
import {
  to = semaphore_ex_integration_alias.deploy
  id = "project/1/integration/2/alias/3"
}
