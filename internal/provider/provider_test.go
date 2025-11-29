package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func testAccProtoV6ProviderFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"truenas": providerserver.NewProtocol6WithError(New("test")()),
	}
}

func testAccPreCheck(t *testing.T) {
	if os.Getenv("TRUENAS_HOST") == "" && os.Getenv("truenas_host") == "" {
		t.Skip("TRUENAS_HOST must be set for acceptance tests")
	}
	if os.Getenv("TRUENAS_DEV_KEY") == "" && os.Getenv("truenas_dev_key") == "" {
		t.Skip("TRUENAS_DEV_KEY must be set for acceptance tests")
	}
}

func TestProvider_Schema(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: `provider "truenas" {}`,
			},
		},
	})
}
