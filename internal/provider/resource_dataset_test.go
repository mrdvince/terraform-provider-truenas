package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatasetResource_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccDatasetResourceConfig("dsrestest", "testds"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.test", "name", "testds"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "pool", "dsrestest"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "id", "dsrestest/testds"),
					resource.TestCheckResourceAttrSet("truenas_dataset.test", "mountpoint"),
				),
			},
		},
	})
}

func TestAccDatasetResource_nested(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccDatasetResourceConfigNested("dsnesttest"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.apps", "id", "dsnesttest/apps"),
					resource.TestCheckResourceAttr("truenas_dataset.immich", "id", "dsnesttest/apps/immich"),
					resource.TestCheckResourceAttr("truenas_dataset.uploads", "id", "dsnesttest/apps/immich/uploads"),
				),
			},
		},
	})
}

func TestAccDatasetResource_import(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccDatasetResourceConfig("dsimporttest", "importds"),
			},
			{
				ResourceName:      "truenas_dataset.test",
				ImportState:       true,
				ImportStateId:     "dsimporttest/importds",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccDatasetResourceConfig(poolName, datasetName string) string {
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

resource "truenas_dataset" "test" {
  name   = %q
  parent = truenas_pool.test.name
}
`, poolName, datasetName)
}

func testAccDatasetResourceConfigNested(poolName string) string {
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

resource "truenas_dataset" "immich" {
  name   = "immich"
  parent = truenas_dataset.apps.id
}

resource "truenas_dataset" "uploads" {
  name   = "uploads"
  parent = truenas_dataset.immich.id
}
`, poolName)
}
