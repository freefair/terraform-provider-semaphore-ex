# Look up a global runner by ID.
data "semaphore_ex_runner" "by_id" {
  id = 1
}

# Look up a global runner by name.
data "semaphore_ex_runner" "by_name" {
  name = "Example Global Runner"
}
