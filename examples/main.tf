terraform {
  required_providers {
    juicefs = {
      source = "toowoxx/juicefs"
    }
  }
}

provider "juicefs" {}

data "juicefs_version" "version" {}

resource "juicefs_format" "format" {
  storage = "file"
  force = true
  metadata_uri = "sqlite3://metadata.db"
  storage_name = "test"
  bucket = "./test-jfs"
}

output "juicefs_version" {
  value = data.juicefs_version.version.version
}
