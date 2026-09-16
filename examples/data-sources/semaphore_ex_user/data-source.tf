# Lookup User by ID
data "semaphore_ex_user" "user" {
  # SemaphoreUI User ID
  id = 1
}

# Lookup User by Username
data "semaphore_ex_user" "batman" {
  username = "batman"
}

# Lookup User by Email
data "semaphore_ex_user" "superman" {
  email = "clark.kent@dailyplanet.com"
}
