package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"truenas/internal/client"
)

// ensure truenasprovider satisfies various provider interfaces.
var _ provider.Provider = &truenasProvider{}

// truenasprovider defines the provider implementation.
type truenasProvider struct {
	version string
}

// truenasprovidermodel describes the provider data model.
type truenasProviderModel struct {
	ApiKey types.String `tfsdk:"api_key"`
	Host   types.String `tfsdk:"host"`
}

func (p *truenasProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "truenas"
	resp.Version = p.version
}

func (p *truenasProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				MarkdownDescription: "the truenas api key.",
				Optional:            true,
				Sensitive:           true,
			},
			"host": schema.StringAttribute{
				MarkdownDescription: "the truenas host url (e.g., https://192.168.1.100).",
				Optional:            true,
			},
		},
	}
}

func (p *truenasProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data truenasProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := os.Getenv("TRUENAS_DEV_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("truenas_dev_key")
	}
	if !data.ApiKey.IsNull() {
		apiKey = data.ApiKey.ValueString()
	}

	host := os.Getenv("TRUENAS_HOST")
	if host == "" {
		host = os.Getenv("truenas_host")
	}
	if !data.Host.IsNull() {
		host = data.Host.ValueString()
	}

	if host == "" {
		resp.Diagnostics.AddError(
			"missing host configuration",
			"the truenas host url was not found in the configuration or environment variable TRUENAS_HOST.",
		)
		return
	}

	if apiKey == "" {
		resp.Diagnostics.AddError(
			"missing api key configuration",
			"while configuring the provider, the api key was not found in the configuration or environment variable truenas_dev_key.",
		)
		return
	}

	c, err := client.NewClient(host, apiKey)
	if err != nil {
		resp.Diagnostics.AddError(
			"unable to create truenas client",
			"an error occurred when creating the truenas client: "+err.Error(),
		)
		return
	}

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *truenasProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{}
}

func (p *truenasProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewSystemVersionDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &truenasProvider{
			version: version,
		}
	}
}
