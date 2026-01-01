package provider

import (
	"context"
	"os"

	"truenas/internal/client"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &truenasProvider{}

type truenasProvider struct {
	version string
}

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
			"Missing host configuration",
			"The TrueNAS host URL was not found in the configuration or environment variable TRUENAS_HOST.",
		)
		return
	}

	if apiKey == "" {
		resp.Diagnostics.AddError(
			"Missing API key configuration",
			"The API key was not found in the configuration or environment variable TRUENAS_DEV_KEY.",
		)
		return
	}

	c, err := client.NewClient(host, apiKey)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create TrueNAS client",
			"An error occurred when creating the TrueNAS client: "+err.Error(),
		)
		return
	}

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *truenasProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewPoolResource,
		NewDatasetResource,
	}
}

func (p *truenasProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewSystemVersionDataSource,
		NewDisksDataSource,
		NewDatasetDataSource,
		NewAppAvailableDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &truenasProvider{
			version: version,
		}
	}
}
