# TrueNAS Terraform Provider

A Terraform provider for managing TrueNAS SCALE resources via the WebSocket API.

## Provider Configuration

The provider requires an API key and host URL. These can be set via environment variables or in the provider block.

```hcl
provider "truenas" {
  host    = "your-host-address-or-ip"
  api_key = "your-api-key"
}
```

### Environment Variables

- `TRUENAS_HOST` - The TrueNAS host URL
- `TRUENAS_DEV_KEY` - The API key

When environment variables are set, the provider block can be empty:

```hcl
provider "truenas" {}
```

## Resources

### truenas_pool

Manages a ZFS storage pool.

```hcl
data "truenas_disks" "all" {}

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
```

#### Arguments

- `name` - (Required) The pool name. Changing this forces recreation.
- `force_recreate` - (Optional) When true, topology changes trigger pool recreation. When false, disk changes are ignored after initial creation.
- `topology` - (Required) Pool topology configuration.
  - `data` - (Required) List of data vdevs.
    - `type` - (Required) Vdev type: STRIPE, MIRROR, RAIDZ1, RAIDZ2, RAIDZ3. Changing this forces recreation.
    - `disks` - (Optional) List of disk names. If omitted, all unused disks are used.

#### Attributes

- `id` - The pool name.
- `pool_id` - The internal numeric ID. Useful for triggering dependent resource replacement when the pool is recreated.

### truenas_dataset

Manages a ZFS dataset.

```hcl
resource "truenas_dataset" "apps" {
  name    = "apps"
  parent  = truenas_pool.storage.name
  pool_id = truenas_pool.storage.pool_id
}

resource "truenas_dataset" "immich" {
  name        = "immich"
  parent      = truenas_dataset.apps.id
  pool_id     = truenas_pool.storage.pool_id
  compression = "LZ4"
}
```

#### Arguments

- `name` - (Required) The dataset name. Changing this forces recreation.
- `parent` - (Required) The parent path (pool name or parent dataset ID). Changing this forces recreation.
- `pool_id` - (Optional) The pool's internal ID. Set this to `truenas_pool.*.pool_id` to automatically recreate datasets when the pool is replaced.
- `comments` - (Optional) User comments.
- `compression` - (Optional) Compression algorithm: OFF, LZ4, GZIP, ZLE, LZJB, ZSTD.
- `quota` - (Optional) Maximum space for dataset and descendants in bytes (0 = no quota).
- `refquota` - (Optional) Maximum space for dataset only in bytes (0 = no refquota).
- `snapdir` - (Optional) Snapshot directory visibility: VISIBLE, HIDDEN, DISABLED.
- `acltype` - (Optional) ACL type: NFSV4, POSIX, OFF, INHERIT.
- `aclmode` - (Optional) ACL mode: PASSTHROUGH, RESTRICTED, DISCARD, INHERIT.
- `sync` - (Optional) Sync mode: STANDARD, ALWAYS, DISABLED.
- `atime` - (Optional) Access time updates: ON, OFF.
- `readonly` - (Optional) Read-only mode: ON, OFF.
- `exec` - (Optional) Allow execution of binaries: ON, OFF.

#### Attributes

- `id` - The full dataset path (e.g., `pool/parent/name`).
- `pool` - The pool containing this dataset.
- `mountpoint` - The filesystem mountpoint.
- `quota` - The quota in bytes.
- `refquota` - The refquota in bytes.
- `snapdir` - Snapshot directory visibility.
- `acltype` - ACL type.
- `aclmode` - ACL mode.
- `sync` - Sync mode.
- `atime` - Access time setting.
- `readonly` - Read-only mode.
- `exec` - Execution permission.

## Data Sources

### truenas_disks

Retrieves available (unused) disks.

```hcl
data "truenas_disks" "all" {}

output "disk_names" {
  value = data.truenas_disks.all.ids
}
```

#### Attributes

- `ids` - List of disk names (e.g., sdb, sdc).
- `disks` - List of disk objects with `name`, `serial`, `size`, and `type`.
- `by_serial` - Map of disks keyed by serial number for stable references.

### truenas_dataset

Retrieves information about an existing dataset.

```hcl
data "truenas_dataset" "root" {
  id = truenas_pool.storage.name
}

output "compression" {
  value = data.truenas_dataset.root.compression
}
```

#### Arguments

- `id` - (Required) The full dataset path (e.g., `pool/dataset` or `pool/parent/child`).

#### Attributes

- `name` - The dataset name without pool prefix.
- `pool` - The pool containing this dataset.
- `mountpoint` - The filesystem mountpoint.
- `compression` - The compression algorithm.
- `quota` - The quota in bytes (0 means no quota).
- `refquota` - The refquota in bytes.
- `comments` - User comments.

### truenas_system_version

Retrieves TrueNAS system version information.

```hcl
data "truenas_system_version" "info" {}

output "version" {
  value = data.truenas_system_version.info.version
}
```

#### Attributes

- `version` - The TrueNAS version string.
- `codename` - The release codename.

## Development

Build the provider:

```bash
go build -o bin/terraform-provider-truenas
```

Run acceptance tests:

```bash
export TRUENAS_HOST="https://your-truenas"
export TRUENAS_DEV_KEY="your-api-key"
TF_ACC=1 go test ./internal/provider/ -v
```

Use a dev override in `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/mrdvince/truenas" = "/path/to/terraform-provider-truenas/bin"
  }
  direct {}
}
```
