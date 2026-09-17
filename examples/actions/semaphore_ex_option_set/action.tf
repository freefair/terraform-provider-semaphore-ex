variable "option_value" {
  type      = string
  sensitive = true
  ephemeral = true
}

action "semaphore_ex_option_set" "configure" {
  config {
    key   = "example.option"
    value = var.option_value
  }
}

# terraform apply -invoke=action.semaphore_ex_option_set.configure
