resource "semaphore_ex_global_role" "auditor" {
  name                = "Auditor"
  project_permissions = []
  global_permissions  = ["global.audit.read"]
}
