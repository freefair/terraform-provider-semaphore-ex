resource "semaphore_ex_totp_policy" "organization" {
  state             = "required_selected"
  selected_user_ids = [42]
}
