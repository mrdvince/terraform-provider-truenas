package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAppAvailableDataSource_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccAppAvailableDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.truenas_app_available.all", "id"),
					resource.TestCheckResourceAttrSet("data.truenas_app_available.all", "apps.%"),
				),
			},
		},
	})
}

func TestAccAppAvailableDataSource_filtered(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccAppAvailableDataSourceConfigFiltered(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.truenas_app_available.media", "id"),
					resource.TestCheckResourceAttrSet("data.truenas_app_available.media", "apps.%"),
				),
			},
		},
	})
}

func testAccAppAvailableDataSourceConfig() string {
	return `
data "truenas_app_available" "all" {}
`
}

func testAccAppAvailableDataSourceConfigFiltered() string {
	return `
data "truenas_app_available" "media" {
  category = "media"
}
`
}
