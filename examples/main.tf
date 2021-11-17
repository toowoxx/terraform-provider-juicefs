terraform {
  required_providers {
    juicefs = {
      source = "toowoxx/juicefs"
    }
  }
}

provider "juicefs" {}

data "juicefs_version" "version" {}

output "juicefs_version" {
  value = data.juicefs_version.version.version
}
