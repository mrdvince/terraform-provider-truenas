terraform {
  required_providers {
    truenas = {
      source = "registry.terraform.io/vince/truenas"
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

resource "truenas_pool" "tank" {
  name = "tank"
  topology {
    data {
      type  = "STRIPE"
      disks = [data.truenas_disks.all.ids[0]] # dynamically selects the first available disk
    }
  }
}

output "pool_id" {
  value = truenas_pool.tank.id
}
