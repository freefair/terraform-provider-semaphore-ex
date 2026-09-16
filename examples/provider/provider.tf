# The local name semaphore matches Terraform's inferred resource prefix.
terraform {
  required_providers {
    semaphore = {
      source = "freefair/semaphore-ex"
    }
  }
}

provider "semaphore" {
  api_base_url = "http://localhost:3000/api"
  # Supply the token through SEMAPHOREUI_API_TOKEN.
}
