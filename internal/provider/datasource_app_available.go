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

var _ datasource.DataSource = &appAvailableDataSource{}

func NewAppAvailableDataSource() datasource.DataSource {
	return &appAvailableDataSource{}
}

type appAvailableDataSource struct {
	client *client.Client
}

type appAvailableDataSourceModel struct {
	ID       types.String            `tfsdk:"id"`
	Train    types.String            `tfsdk:"train"`
	Category types.String            `tfsdk:"category"`
	Apps     map[string]appInfoModel `tfsdk:"apps"`
}

type appInfoModel struct {
	Name             types.String   `tfsdk:"name"`
	Title            types.String   `tfsdk:"title"`
	Description      types.String   `tfsdk:"description"`
	LatestVersion    types.String   `tfsdk:"latest_version"`
	LatestAppVersion types.String   `tfsdk:"latest_app_version"`
	HumanVersion     types.String   `tfsdk:"human_version"`
	Train            types.String   `tfsdk:"train"`
	Catalog          types.String   `tfsdk:"catalog"`
	Installed        types.Bool     `tfsdk:"installed"`
	Healthy          types.Bool     `tfsdk:"healthy"`
	IconURL          types.String   `tfsdk:"icon_url"`
	Home             types.String   `tfsdk:"home"`
	Categories       []types.String `tfsdk:"categories"`
	Tags             []types.String `tfsdk:"tags"`
}

func (d *appAvailableDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_available"
}

func (d *appAvailableDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Query available apps from the TrueNAS app catalog.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"train": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Filter by train (e.g., 'stable', 'community'). Defaults to all trains.",
			},
			"category": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Filter by category (e.g., 'media', 'storage', 'networking').",
			},
			"apps": schema.MapNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Map of available apps keyed by app name.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The app name used in app.create.",
						},
						"title": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Display title of the app.",
						},
						"description": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Short description of the app.",
						},
						"latest_version": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Latest chart version.",
						},
						"latest_app_version": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Latest application version.",
						},
						"human_version": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Human-readable version string.",
						},
						"train": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Train the app belongs to.",
						},
						"catalog": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Catalog the app belongs to.",
						},
						"installed": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the app is currently installed.",
						},
						"healthy": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the app is healthy.",
						},
						"icon_url": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "URL to the app icon.",
						},
						"home": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "App homepage URL.",
						},
						"categories": schema.ListAttribute{
							ElementType:         types.StringType,
							Computed:            true,
							MarkdownDescription: "Categories the app belongs to.",
						},
						"tags": schema.ListAttribute{
							ElementType:         types.StringType,
							Computed:            true,
							MarkdownDescription: "Tags associated with the app.",
						},
					},
				},
			},
		},
	}
}

func (d *appAvailableDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *appAvailableDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data appAvailableDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := d.client.Call(ctx, "app.available", nil)
	if err != nil {
		resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to query available apps: %s", err))
		return
	}

	var apps []map[string]any
	if err := json.Unmarshal(apiResp.Result, &apps); err != nil {
		resp.Diagnostics.AddError("Client error", "Unable to parse app.available response")
		return
	}

	trainFilter := ""
	if !data.Train.IsNull() {
		trainFilter = data.Train.ValueString()
	}
	categoryFilter := ""
	if !data.Category.IsNull() {
		categoryFilter = data.Category.ValueString()
	}

	appMap := make(map[string]appInfoModel)
	for _, app := range apps {
		name, _ := app["name"].(string)
		train, _ := app["train"].(string)

		if trainFilter != "" && train != trainFilter {
			continue
		}

		if categoryFilter != "" {
			categories, _ := app["categories"].([]any)
			found := false
			for _, c := range categories {
				if cs, ok := c.(string); ok && cs == categoryFilter {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		title, _ := app["title"].(string)
		description, _ := app["description"].(string)
		latestVersion, _ := app["latest_version"].(string)
		latestAppVersion, _ := app["latest_app_version"].(string)
		humanVersion, _ := app["latest_human_version"].(string)
		catalog, _ := app["catalog"].(string)
		installed, _ := app["installed"].(bool)
		healthy, _ := app["healthy"].(bool)
		iconURL, _ := app["icon_url"].(string)
		home, _ := app["home"].(string)

		var categories []types.String
		if catList, ok := app["categories"].([]any); ok {
			for _, c := range catList {
				if cs, ok := c.(string); ok {
					categories = append(categories, types.StringValue(cs))
				}
			}
		}

		var tags []types.String
		if tagList, ok := app["tags"].([]any); ok {
			for _, t := range tagList {
				if ts, ok := t.(string); ok {
					tags = append(tags, types.StringValue(ts))
				}
			}
		}

		appMap[name] = appInfoModel{
			Name:             types.StringValue(name),
			Title:            types.StringValue(title),
			Description:      types.StringValue(description),
			LatestVersion:    types.StringValue(latestVersion),
			LatestAppVersion: types.StringValue(latestAppVersion),
			HumanVersion:     types.StringValue(humanVersion),
			Train:            types.StringValue(train),
			Catalog:          types.StringValue(catalog),
			Installed:        types.BoolValue(installed),
			Healthy:          types.BoolValue(healthy),
			IconURL:          types.StringValue(iconURL),
			Home:             types.StringValue(home),
			Categories:       categories,
			Tags:             tags,
		}
	}

	id := "apps"
	if trainFilter != "" {
		id = fmt.Sprintf("apps-%s", trainFilter)
	}
	if categoryFilter != "" {
		id = fmt.Sprintf("%s-%s", id, categoryFilter)
	}
	data.ID = types.StringValue(id)
	data.Apps = appMap

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
