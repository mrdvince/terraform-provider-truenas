terraform {
  required_providers {
    truenas = {
      source = "registry.terraform.io/vince/truenas"
    }
  }
}

provider "truenas" {
  # api_key is set via TRUENAS_DEV_KEY env var
  # host is default to https://192.168.50.39
}

data "truenas_system_version" "test" {}

output "truenas_version" {
  value = data.truenas_system_version.test.version
}
