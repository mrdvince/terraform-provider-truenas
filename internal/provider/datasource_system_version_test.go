package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSystemVersionDataSource_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccSystemVersionDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_system_version.test", "id", "truenas-system-version"),
					resource.TestCheckResourceAttrSet("data.truenas_system_version.test", "version"),
				),
			},
		},
	})
}

func TestAccSystemVersionDataSource_versionFormat(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: testAccSystemVersionDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestMatchResourceAttr("data.truenas_system_version.test", "version", regexp.MustCompile(`^\d+\.\d+`)),
				),
			},
		},
	})
}

func testAccSystemVersionDataSourceConfig() string {
	return `
data "truenas_system_version" "test" {}
`
}
