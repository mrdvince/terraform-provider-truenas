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
      type  = "RAIDZ1"
      disks = data.truenas_disks.all.ids
    }
  }
}

output "pool_id" {
  value = truenas_pool.storage.id
}

data "truenas_dataset" "root" {
  id = truenas_pool.storage.name
}

output "dataset_info" {
  value = {
    id          = data.truenas_dataset.root.id
    name        = data.truenas_dataset.root.name
    pool        = data.truenas_dataset.root.pool
    mountpoint  = data.truenas_dataset.root.mountpoint
    compression = data.truenas_dataset.root.compression
  }
}

resource "truenas_dataset" "apps" {
  name    = "apps"
  parent  = truenas_pool.storage.name
  pool_id = truenas_pool.storage.pool_id
}

resource "truenas_dataset" "immich" {
  name    = "immich"
  parent  = truenas_dataset.apps.id
  pool_id = truenas_pool.storage.pool_id
}

resource "truenas_dataset" "uploads" {
  name    = "uploads"
  parent  = truenas_dataset.immich.id
  pool_id = truenas_pool.storage.pool_id
}

resource "truenas_dataset" "data" {
  name    = "thumbs"
  parent  = truenas_dataset.immich.id
  pool_id = truenas_pool.storage.pool_id
}

output "nested_datasets" {
  value = {
    apps    = truenas_dataset.apps.id
    immich  = truenas_dataset.immich.id
    uploads = truenas_dataset.uploads.id
    data    = truenas_dataset.data.id
  }
}
