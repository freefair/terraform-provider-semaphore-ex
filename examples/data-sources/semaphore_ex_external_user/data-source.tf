# Look up an existing external user. Missing users produce an error.
data "semaphore_ex_external_user" "user" {
  username = "batman"
}

# Create and manage an external user explicitly when needed.
resource "semaphore_ex_user" "batman" {
  username = "batman"
  name     = "Bruce Wayne"
  email    = "batman@wayneenterprises.com"
  external = true
}
