# Lookup or Create External User
data "semaphore_ex_user" "user" {
  username = "batman"
}

# Lookup or Create External User with additional attributes
data "semaphore_ex_user" "batman" {
  username = "batman"
  name     = "Bruce Wayne"
  email    = "batman@wayneenterprises.com"
}
