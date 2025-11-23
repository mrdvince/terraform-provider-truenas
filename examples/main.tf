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

resource "truenas_pool" "tank" {
  name = "tank"
  force_recreate = true
  topology {
    data {
      type  = "STRIPE"
      disks = [data.truenas_disks.all.ids[0]]
    }
  }
}

output "pool_id" {
  value = truenas_pool.tank.id
}
