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

func TestAccDatasetResource_properties(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccDatasetResourceConfigWithProperties("dsproptest", "propds", "LZ4", 0, "HIDDEN"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.test", "name", "propds"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "compression", "LZ4"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "quota", "0"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "snapdir", "HIDDEN"),
				),
			},
			{
				Config: testAccDatasetResourceConfigWithProperties("dsproptest", "propds", "ZSTD", 1073741824, "VISIBLE"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.test", "compression", "ZSTD"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "quota", "1073741824"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "snapdir", "VISIBLE"),
				),
			},
		},
	})
}

func TestAccDatasetResource_aclSettings(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccDatasetResourceConfigWithAcl("dsacltest", "aclds", "NFSV4", "PASSTHROUGH"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.test", "name", "aclds"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "acltype", "NFSV4"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "aclmode", "PASSTHROUGH"),
				),
			},
		},
	})
}

func testAccDatasetResourceConfigWithProperties(poolName, datasetName, compression string, quota int64, snapdir string) string {
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
  name        = %q
  parent      = truenas_pool.test.name
  compression = %q
  quota       = %d
  snapdir     = %q
}
`, poolName, datasetName, compression, quota, snapdir)
}

func testAccDatasetResourceConfigWithAcl(poolName, datasetName, acltype, aclmode string) string {
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
  name    = %q
  parent  = truenas_pool.test.name
  acltype = %q
  aclmode = %q
}
`, poolName, datasetName, acltype, aclmode)
}
