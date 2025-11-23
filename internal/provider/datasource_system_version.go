package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"truenas/internal/client"
)

var _ datasource.DataSource = &systemVersionDataSource{}

func NewSystemVersionDataSource() datasource.DataSource {
	return &systemVersionDataSource{}
}

type systemVersionDataSource struct {
	client *client.Client
}

type systemVersionDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	BuildTime types.String `tfsdk:"build_time"`
	Version   types.String `tfsdk:"version"`
	Codename  types.String `tfsdk:"codename"`
}

func (d *systemVersionDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_system_version"
}

func (d *systemVersionDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "retrieves the system version information from truenas.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"build_time": schema.StringAttribute{
				Computed: true,
			},
			"version": schema.StringAttribute{
				Computed: true,
			},
			"codename": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (d *systemVersionDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected data source configure type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *systemVersionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data systemVersionDataSourceModel

	apiResp, err := d.client.Call(ctx, "system.info", nil)
	if err != nil {
		resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to read system info: %s", err))
		return
	}

	var result map[string]any
	if err := json.Unmarshal(apiResp.Result, &result); err != nil {
		resp.Diagnostics.AddError("Parse error", "Unable to parse system info response")
		return
	}

	if v, ok := result["version"].(string); ok {
		data.Version = types.StringValue(v)
	}
	// TODO: parse buildtime (can be object like {"$date": 1670900000} or string)
	if v, ok := result["codename"].(string); ok {
		data.Codename = types.StringValue(v)
	}

	data.ID = types.StringValue("truenas-system-version")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
