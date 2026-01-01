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

var _ datasource.DataSource = &appCategoriesDataSource{}

func NewAppCategoriesDataSource() datasource.DataSource {
	return &appCategoriesDataSource{}
}

type appCategoriesDataSource struct {
	client *client.Client
}

type appCategoriesDataSourceModel struct {
	ID         types.String   `tfsdk:"id"`
	Categories []types.String `tfsdk:"categories"`
}

func (d *appCategoriesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_categories"
}

func (d *appCategoriesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Query available app categories from the TrueNAS catalog.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"categories": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "List of available app categories.",
			},
		},
	}
}

func (d *appCategoriesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *appCategoriesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data appCategoriesDataSourceModel

	apiResp, err := d.client.Call(ctx, "app.categories", nil)
	if err != nil {
		resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to query app categories: %s", err))
		return
	}

	var categories []string
	if err := json.Unmarshal(apiResp.Result, &categories); err != nil {
		resp.Diagnostics.AddError("Client error", "Unable to parse app.categories response")
		return
	}

	data.ID = types.StringValue("app_categories")
	data.Categories = make([]types.String, len(categories))
	for i, cat := range categories {
		data.Categories[i] = types.StringValue(cat)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
