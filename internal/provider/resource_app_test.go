package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAppResource_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `
resource "truenas_app" "test" {
  name        = "syncthing"
  catalog_app = "syncthing"
  train       = "stable"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_app.test", "name", "syncthing"),
					resource.TestCheckResourceAttr("truenas_app.test", "catalog_app", "syncthing"),
					resource.TestCheckResourceAttr("truenas_app.test", "train", "stable"),
					resource.TestCheckResourceAttrSet("truenas_app.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_app.test", "state"),
				),
			},
		},
	})
}

func TestAccAppResource_withValues(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `
resource "truenas_app" "test" {
  name        = "syncthing"
  catalog_app = "syncthing"
  train       = "stable"
  values      = jsonencode({
    TZ = "Europe/Amsterdam"
  })
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_app.test", "name", "syncthing"),
					resource.TestCheckResourceAttrSet("truenas_app.test", "state"),
				),
			},
		},
	})
}
