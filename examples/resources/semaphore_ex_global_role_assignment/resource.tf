resource "semaphore_ex_global_role_assignment" "auditor" {
  user_id = semaphore_ex_user.example.id
  role_id = semaphore_ex_global_role.auditor.id
}
