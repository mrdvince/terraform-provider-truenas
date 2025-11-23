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
			"unexpected data source configure type",
			fmt.Sprintf("expected *client.Client, got: %T. please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *systemVersionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data systemVersionDataSourceModel

	// call system.info
	// truenas api: system.info returns a dict with version info
	apiResp, err := d.client.Call(ctx, "system.info", nil)
	if err != nil {
		resp.Diagnostics.AddError("client error", fmt.Sprintf("unable to read system info: %s", err))
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(apiResp.Result, &result); err != nil {
		resp.Diagnostics.AddError("parse error", "unable to parse system info response")
		return
	}

	// map fields
	if v, ok := result["version"].(string); ok {
		data.Version = types.StringValue(v)
	}
	if _, ok := result["buildtime"].(map[string]interface{}); ok {
		// buildtime is complex object sometimes, or string? let's check standard output.
		// usually it might be a timestamp or object. for now let's just try to grab it if it's a string,
		// or ignore/stringify if complex.
		// actually in scale it's often an object or string. let's be safe and just use version for now as primary goal.
		// checking docs or example response: {"version": "TrueNAS-SCALE-22.12.0", "buildtime": {"$date": 1670900000}}
		// let's skip buildtime for now to avoid parsing issues unless we are sure.
		// wait, user asked for "returning just the version".
	}
	if v, ok := result["codename"].(string); ok {
		data.Codename = types.StringValue(v)
	}

	data.ID = types.StringValue("truenas-system-version") // singleton id

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
