package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"truenas/internal/client"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &disksDataSource{}

func NewDisksDataSource() datasource.DataSource {
	return &disksDataSource{}
}

type disksDataSource struct {
	client *client.Client
}

type disksDataSourceModel struct {
	ID       types.String         `tfsdk:"id"`
	Ids      []types.String       `tfsdk:"ids"`
	Disks    []diskModel          `tfsdk:"disks"`
	BySerial map[string]diskModel `tfsdk:"by_serial"`
}

type diskModel struct {
	Name   types.String `tfsdk:"name"`
	Serial types.String `tfsdk:"serial"`
	Size   types.Int64  `tfsdk:"size"`
	Type   types.String `tfsdk:"type"`
}

func (d *disksDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_disks"
}

func (d *disksDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves a list of available (unused) disks.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"ids": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "List of disk names (e.g. sdb, sdc).",
			},
			"disks": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Computed: true,
						},
						"serial": schema.StringAttribute{
							Computed: true,
						},
						"size": schema.Int64Attribute{
							Computed: true,
						},
						"type": schema.StringAttribute{
							Computed: true,
						},
					},
				},
			},
			"by_serial": schema.MapNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Map of disks keyed by serial number for stable references.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Computed: true,
						},
						"serial": schema.StringAttribute{
							Computed: true,
						},
						"size": schema.Int64Attribute{
							Computed: true,
						},
						"type": schema.StringAttribute{
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func (d *disksDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected datasource configure type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *disksDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data disksDataSourceModel

	apiResp, err := d.client.Call(ctx, "disk.get_unused", nil)
	if err != nil {
		resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to query unused disks: %s", err))
		return
	}

	var disks []map[string]any
	if err := json.Unmarshal(apiResp.Result, &disks); err != nil {
		resp.Diagnostics.AddError("Client error", "Unable to parse disk query response")
		return
	}

	var ids []types.String
	var diskModels []diskModel
	bySerial := make(map[string]diskModel)

	for _, disk := range disks {
		name := disk["name"].(string)
		serial := ""
		if s, ok := disk["serial"].(string); ok {
			serial = s
		}
		size := int64(0)
		if s, ok := disk["size"].(float64); ok {
			size = int64(s)
		}
		dtype := ""
		if t, ok := disk["type"].(string); ok {
			dtype = t
		}

		model := diskModel{
			Name:   types.StringValue(name),
			Serial: types.StringValue(serial),
			Size:   types.Int64Value(size),
			Type:   types.StringValue(dtype),
		}

		ids = append(ids, types.StringValue(name))
		diskModels = append(diskModels, model)
		if serial != "" {
			bySerial[serial] = model
		}
	}

	data.ID = types.StringValue("disks")
	data.Ids = ids
	data.Disks = diskModels
	data.BySerial = bySerial

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
