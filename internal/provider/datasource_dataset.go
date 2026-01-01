package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"truenas/internal/client"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &datasetDataSource{}

func NewDatasetDataSource() datasource.DataSource {
	return &datasetDataSource{}
}

type datasetDataSource struct {
	client *client.Client
}

type datasetDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Pool        types.String `tfsdk:"pool"`
	Mountpoint  types.String `tfsdk:"mountpoint"`
	Compression types.String `tfsdk:"compression"`
	Quota       types.Int64  `tfsdk:"quota"`
	Refquota    types.Int64  `tfsdk:"refquota"`
	Comments    types.String `tfsdk:"comments"`
}

func (d *datasetDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset"
}

func (d *datasetDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves information about a ZFS dataset.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The full path of the dataset (e.g. pool/dataset or pool/parent/child).",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The dataset name (without pool prefix).",
			},
			"pool": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The pool containing this dataset.",
			},
			"mountpoint": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The filesystem mountpoint.",
			},
			"compression": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The compression algorithm.",
			},
			"quota": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "The quota in bytes (0 means no quota).",
			},
			"refquota": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "The refquota in bytes (0 means no refquota).",
			},
			"comments": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "User comments for the dataset.",
			},
		},
	}
}

func (d *datasetDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *datasetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data datasetDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	datasetID := data.ID.ValueString()

	filters := []any{
		[]any{"id", "=", datasetID},
	}

	apiResp, err := d.client.Call(ctx, "pool.dataset.query", []any{filters})
	if err != nil {
		resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to query dataset: %s", err))
		return
	}

	var datasets []map[string]any
	if err := json.Unmarshal(apiResp.Result, &datasets); err != nil {
		resp.Diagnostics.AddError("Client error", "Unable to parse dataset query response")
		return
	}

	if len(datasets) == 0 {
		resp.Diagnostics.AddError("Not found", fmt.Sprintf("Dataset %q not found", datasetID))
		return
	}

	dataset := datasets[0]

	data.ID = types.StringValue(dataset["id"].(string))

	if name, ok := dataset["name"].(string); ok {
		data.Name = types.StringValue(name)
	}

	if pool, ok := dataset["pool"].(string); ok {
		data.Pool = types.StringValue(pool)
	} else {
		parts := strings.SplitN(datasetID, "/", 2)
		data.Pool = types.StringValue(parts[0])
	}

	if mountpoint, ok := dataset["mountpoint"].(string); ok {
		data.Mountpoint = types.StringValue(mountpoint)
	}

	if compression, ok := dataset["compression"].(map[string]any); ok {
		if value, ok := compression["value"].(string); ok {
			data.Compression = types.StringValue(value)
		}
	}

	if quota, ok := dataset["quota"].(map[string]any); ok {
		if parsed, ok := quota["parsed"].(float64); ok {
			data.Quota = types.Int64Value(int64(parsed))
		}
	} else {
		data.Quota = types.Int64Value(0)
	}

	if refquota, ok := dataset["refquota"].(map[string]any); ok {
		if parsed, ok := refquota["parsed"].(float64); ok {
			data.Refquota = types.Int64Value(int64(parsed))
		}
	} else {
		data.Refquota = types.Int64Value(0)
	}

	if comments, ok := dataset["comments"].(map[string]any); ok {
		if value, ok := comments["value"].(string); ok {
			data.Comments = types.StringValue(value)
		}
	} else {
		data.Comments = types.StringValue("")
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
