resource "semaphore_ex_app" "custom" {
  id       = "custom_tool"
  title    = "Custom tool"
  path     = "/usr/local/bin/custom-tool"
  args     = ["--verbose"]
  active   = true
  priority = 10
}
