package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDockerResource_basic(t *testing.T) {
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `
resource "truenas_docker" "test" {
  pool = "storage"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_docker.test", "pool", "storage"),
					resource.TestCheckResourceAttr("truenas_docker.test", "enable_image_updates", "true"),
					resource.TestCheckResourceAttrSet("truenas_docker.test", "id"),
				),
			},
		},
	})
}
