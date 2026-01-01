package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatasetDataSource_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccDatasetDataSourceConfig("dstest"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_dataset.test", "id", "dstest"),
					resource.TestCheckResourceAttr("data.truenas_dataset.test", "name", "dstest"),
					resource.TestCheckResourceAttr("data.truenas_dataset.test", "pool", "dstest"),
					resource.TestCheckResourceAttrSet("data.truenas_dataset.test", "mountpoint"),
				),
			},
		},
	})
}

func TestAccDatasetDataSource_nested(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccDatasetDataSourceConfigNested("dsnestedtest"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_dataset.child", "id", "dsnestedtest/apps"),
					resource.TestCheckResourceAttr("data.truenas_dataset.child", "name", "apps"),
					resource.TestCheckResourceAttr("data.truenas_dataset.child", "pool", "dsnestedtest"),
				),
			},
		},
	})
}

func testAccDatasetDataSourceConfig(poolName string) string {
	return fmt.Sprintf(`
data "truenas_disks" "available" {}

resource "truenas_pool" "test" {
  name = %q
  topology {
    data {
      type  = "STRIPE"
      disks = [data.truenas_disks.available.ids[0]]
    }
  }
}

data "truenas_dataset" "test" {
  id = truenas_pool.test.name
}
`, poolName)
}

func testAccDatasetDataSourceConfigNested(poolName string) string {
	return fmt.Sprintf(`
data "truenas_disks" "available" {}

resource "truenas_pool" "test" {
  name = %q
  topology {
    data {
      type  = "STRIPE"
      disks = [data.truenas_disks.available.ids[0]]
    }
  }
}

resource "truenas_dataset" "apps" {
  name   = "apps"
  parent = truenas_pool.test.name
}

data "truenas_dataset" "child" {
  id = truenas_dataset.apps.id
}
`, poolName)
}
