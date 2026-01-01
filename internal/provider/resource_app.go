package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"truenas/internal/client"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &appResource{}
var _ resource.ResourceWithImportState = &appResource{}

func NewAppResource() resource.Resource {
	return &appResource{}
}

type appResource struct {
	client *client.Client
}

type appResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	CatalogApp       types.String `tfsdk:"catalog_app"`
	Train            types.String `tfsdk:"train"`
	Version          types.String `tfsdk:"version"`
	CustomApp        types.Bool   `tfsdk:"custom_app"`
	Values           types.String `tfsdk:"values"`
	CustomCompose    types.String `tfsdk:"custom_compose"`
	State            types.String `tfsdk:"state"`
	InstalledVersion types.String `tfsdk:"installed_version"`
	UpgradeAvailable types.Bool   `tfsdk:"upgrade_available"`
	Portals          types.Map    `tfsdk:"portals"`
}

func (r *appResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app"
}

func (r *appResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Deploys and manages a TrueNAS application from the catalog or custom Docker Compose.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The application name (used as ID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Application name. Must be lowercase, start with a letter, contain only alphanumerics and hyphens, max 40 chars.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"catalog_app": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Catalog application to deploy (e.g., 'immich', 'plex'). Required unless custom_app is true.",
			},
			"train": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("stable"),
				MarkdownDescription: "Release train (e.g., 'stable', 'community'). Defaults to 'stable'.",
			},
			"version": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("latest"),
				MarkdownDescription: "Application version. Defaults to 'latest'.",
			},
			"custom_app": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Set to true for custom Docker Compose apps.",
			},
			"values": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "JSON-encoded configuration values for the application. Structure depends on the catalog app.",
			},
			"custom_compose": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Docker Compose YAML/JSON configuration for custom apps. Only used when custom_app is true.",
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Current state of the application (DEPLOYING, RUNNING, STOPPED, CRASHED).",
			},
			"installed_version": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Currently installed version.",
			},
			"upgrade_available": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether an upgrade is available.",
			},
			"portals": schema.MapAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: "Map of portal names to URLs for accessing the application.",
			},
		},
	}
}

func (r *appResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected resource configure type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *appResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data appResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := map[string]any{
		"app_name": data.Name.ValueString(),
	}

	if !data.CatalogApp.IsNull() && !data.CatalogApp.IsUnknown() {
		params["catalog_app"] = data.CatalogApp.ValueString()
	}

	if !data.Train.IsNull() && !data.Train.IsUnknown() {
		params["train"] = data.Train.ValueString()
	}

	if !data.Version.IsNull() && !data.Version.IsUnknown() && data.Version.ValueString() != "latest" {
		params["version"] = data.Version.ValueString()
	}

	if !data.CustomApp.IsNull() && data.CustomApp.ValueBool() {
		params["custom_app"] = true
	}

	if !data.Values.IsNull() && !data.Values.IsUnknown() && data.Values.ValueString() != "" {
		var values map[string]any
		if err := json.Unmarshal([]byte(data.Values.ValueString()), &values); err != nil {
			resp.Diagnostics.AddError("Invalid values", fmt.Sprintf("Failed to parse values JSON: %s", err))
			return
		}
		params["values"] = values
	}

	if !data.CustomCompose.IsNull() && !data.CustomCompose.IsUnknown() && data.CustomCompose.ValueString() != "" {
		params["custom_compose_config_string"] = data.CustomCompose.ValueString()
	}

	apiResp, err := r.client.Call(ctx, "app.create", []any{params})
	if err != nil {
		resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to create app: %s", err))
		return
	}

	var jobID int64
	if err := json.Unmarshal(apiResp.Result, &jobID); err == nil && jobID > 0 {
		if err := r.client.WaitForJob(ctx, jobID); err != nil {
			resp.Diagnostics.AddError("Client error", fmt.Sprintf("App creation job failed: %s", err))
			return
		}
	}

	data.ID = data.Name

	r.readAppState(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *appResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data appResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.readAppState(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *appResource) readAppState(ctx context.Context, data *appResourceModel, diags *diag.Diagnostics) {
	filters := []any{
		[]any{"name", "=", data.Name.ValueString()},
	}

	apiResp, err := r.client.Call(ctx, "app.query", []any{filters})
	if err != nil {
		diags.AddError("Client error", fmt.Sprintf("Unable to read app: %s", err))
		return
	}

	var apps []map[string]any
	if err := json.Unmarshal(apiResp.Result, &apps); err != nil {
		diags.AddError("Client error", "Unable to parse app query response")
		return
	}

	if len(apps) == 0 {
		diags.AddError("Not found", fmt.Sprintf("App %s not found", data.Name.ValueString()))
		return
	}

	app := apps[0]

	data.ID = types.StringValue(app["name"].(string))

	if state, ok := app["state"].(string); ok {
		data.State = types.StringValue(state)
	}

	if version, ok := app["version"].(string); ok {
		data.InstalledVersion = types.StringValue(version)
	}

	if upgradeAvail, ok := app["upgrade_available"].(bool); ok {
		data.UpgradeAvailable = types.BoolValue(upgradeAvail)
	}

	if customApp, ok := app["custom_app"].(bool); ok {
		data.CustomApp = types.BoolValue(customApp)
	}

	portals := make(map[string]string)
	if portalsList, ok := app["portals"].([]any); ok {
		for _, p := range portalsList {
			if portal, ok := p.(map[string]any); ok {
				name, _ := portal["name"].(string)
				host, _ := portal["host"].(string)
				port := ""
				if portNum, ok := portal["port"].(float64); ok {
					port = fmt.Sprintf("%.0f", portNum)
				}
				scheme, _ := portal["scheme"].(string)
				path, _ := portal["path"].(string)
				if name != "" && host != "" {
					url := fmt.Sprintf("%s://%s", scheme, host)
					if port != "" {
						url = fmt.Sprintf("%s:%s", url, port)
					}
					if path != "" {
						url = url + path
					}
					portals[name] = url
				}
			}
		}
	}
	portalsValue, _ := types.MapValueFrom(ctx, types.StringType, portals)
	data.Portals = portalsValue

	if metadata, ok := app["metadata"].(map[string]any); ok {
		if train, ok := metadata["train"].(string); ok {
			data.Train = types.StringValue(train)
		}
	}
}

func (r *appResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data appResourceModel
	var state appResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateParams := map[string]any{}

	if !data.Values.IsNull() && !data.Values.IsUnknown() && data.Values.ValueString() != "" {
		var values map[string]any
		if err := json.Unmarshal([]byte(data.Values.ValueString()), &values); err != nil {
			resp.Diagnostics.AddError("Invalid values", fmt.Sprintf("Failed to parse values JSON: %s", err))
			return
		}
		updateParams["values"] = values
	}

	if !data.CustomCompose.IsNull() && !data.CustomCompose.IsUnknown() && data.CustomCompose.ValueString() != "" {
		updateParams["custom_compose_config_string"] = data.CustomCompose.ValueString()
	}

	if len(updateParams) > 0 {
		apiResp, err := r.client.Call(ctx, "app.update", []any{data.Name.ValueString(), updateParams})
		if err != nil {
			resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to update app: %s", err))
			return
		}

		var jobID int64
		if err := json.Unmarshal(apiResp.Result, &jobID); err == nil && jobID > 0 {
			if err := r.client.WaitForJob(ctx, jobID); err != nil {
				resp.Diagnostics.AddError("Client error", fmt.Sprintf("App update job failed: %s", err))
				return
			}
		}
	}

	r.readAppState(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *appResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data appResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Call(ctx, "app.delete", []any{data.Name.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to delete app: %s", err))
		return
	}

	var jobID int64
	if err := json.Unmarshal(apiResp.Result, &jobID); err == nil && jobID > 0 {
		if err := r.client.WaitForJob(ctx, jobID); err != nil {
			resp.Diagnostics.AddError("Client error", fmt.Sprintf("App deletion job failed: %s", err))
			return
		}
	}
}

func (r *appResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}
