package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDisksDataSource_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccDisksDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_disks.test", "id", "disks"),
					resource.TestCheckResourceAttrSet("data.truenas_disks.test", "ids.#"),
					resource.TestCheckResourceAttrSet("data.truenas_disks.test", "disks.#"),
				),
			},
		},
	})
}

func TestAccDisksDataSource_hasSerialMap(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccDisksDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.truenas_disks.test", "by_serial.%"),
				),
			},
		},
	})
}

func TestAccDisksDataSource_diskAttributes(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccDisksDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr("data.truenas_disks.test", "disks.0.name", regexp.MustCompile(`^sd[a-z]+$`)),
					resource.TestCheckResourceAttrSet("data.truenas_disks.test", "disks.0.size"),
				),
			},
		},
	})
}

func testAccDisksDataSourceConfig() string {
	return `
data "truenas_disks" "test" {}
`
}
