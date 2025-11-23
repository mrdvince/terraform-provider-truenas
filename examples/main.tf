terraform {
  required_providers {
    truenas = {
      source = "registry.terraform.io/mrdvince/truenas"
    }
  }
}

provider "truenas" {
  # api_key and host from env vars
}

data "truenas_disks" "all" {}

output "available_disks" {
  value = data.truenas_disks.all.ids
}

output "disks_by_serial" {
  value = data.truenas_disks.all.by_serial
}

resource "truenas_pool" "storage" {
  name           = "storage"
  force_recreate = true
  topology {
    data {
      type  = "RAIDZ2"
      disks = data.truenas_disks.all.ids
    }
  }
}

output "pool_id" {
  value = truenas_pool.storage.id
}
