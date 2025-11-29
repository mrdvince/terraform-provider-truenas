package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPoolResource_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccPoolResourceConfig("testpool"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_pool.test", "name", "testpool"),
					resource.TestCheckResourceAttr("truenas_pool.test", "id", "testpool"),
					resource.TestCheckResourceAttr("truenas_pool.test", "topology.data.#", "1"),
					resource.TestCheckResourceAttr("truenas_pool.test", "topology.data.0.type", "STRIPE"),
				),
			},
		},
	})
}

func TestAccPoolResource_import(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccPoolResourceConfig("importtest"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_pool.test", "name", "importtest"),
				),
			},
			{
				ResourceName:      "truenas_pool.test",
				ImportState:       true,
				ImportStateId:     "importtest",
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"force_recreate",
					"topology.data.0.disks",
				},
			},
		},
	})
}

func TestAccPoolResource_forceRecreate(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccPoolResourceConfigWithForceRecreate("recreatetest", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_pool.test", "name", "recreatetest"),
					resource.TestCheckResourceAttr("truenas_pool.test", "force_recreate", "true"),
				),
				// with force_recreate and dynamic disk selection, drift is expected
				// since the first available disk changes after pool creation
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccPoolResource_mirror(t *testing.T) {
	testAccPreCheck(t)
	t.Skip("requires at least 2 disks")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccPoolResourceConfigMirror("mirrortest"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_pool.test", "name", "mirrortest"),
					resource.TestCheckResourceAttr("truenas_pool.test", "topology.data.0.type", "MIRROR"),
					resource.TestCheckResourceAttr("truenas_pool.test", "topology.data.0.disks.#", "2"),
				),
			},
		},
	})
}

func testAccPoolResourceConfig(name string) string {
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
`, name)
}

func testAccPoolResourceConfigWithForceRecreate(name string, forceRecreate bool) string {
	return fmt.Sprintf(`
data "truenas_disks" "available" {}

resource "truenas_pool" "test" {
  name           = %q
  force_recreate = %t
  topology {
    data {
      type  = "STRIPE"
      disks = [data.truenas_disks.available.ids[0]]
    }
  }
}
`, name, forceRecreate)
}

func testAccPoolResourceConfigMirror(name string) string {
	return fmt.Sprintf(`
data "truenas_disks" "available" {}

resource "truenas_pool" "test" {
  name = %q
  topology {
    data {
      type  = "MIRROR"
      disks = [data.truenas_disks.available.ids[0], data.truenas_disks.available.ids[1]]
    }
  }
}
`, name)
}
