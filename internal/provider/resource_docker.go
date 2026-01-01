package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"truenas/internal/client"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &dockerResource{}

func NewDockerResource() resource.Resource {
	return &dockerResource{}
}

type dockerResource struct {
	client *client.Client
}

type dockerResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Pool               types.String `tfsdk:"pool"`
	EnableImageUpdates types.Bool   `tfsdk:"enable_image_updates"`
	Nvidia             types.Bool   `tfsdk:"nvidia"`
}

func (r *dockerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_docker"
}

func (r *dockerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Configures Docker/container settings for TrueNAS apps. Must be configured before deploying apps.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"pool": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The storage pool to use for Docker containers and app data.",
			},
			"enable_image_updates": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Enable automatic Docker image updates. Defaults to true.",
			},
			"nvidia": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Enable NVIDIA GPU support. Defaults to false.",
			},
		},
	}
}

func (r *dockerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *dockerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data dockerResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := map[string]any{
		"pool": data.Pool.ValueString(),
	}

	if !data.EnableImageUpdates.IsNull() && !data.EnableImageUpdates.IsUnknown() {
		params["enable_image_updates"] = data.EnableImageUpdates.ValueBool()
	}

	if !data.Nvidia.IsNull() && !data.Nvidia.IsUnknown() {
		params["nvidia"] = data.Nvidia.ValueBool()
	}

	apiResp, err := r.client.Call(ctx, "docker.update", []any{params})
	if err != nil {
		resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to configure Docker: %s", err))
		return
	}

	var jobID int64
	if err := json.Unmarshal(apiResp.Result, &jobID); err == nil && jobID > 0 {
		if err := r.client.WaitForJob(ctx, jobID); err != nil {
			resp.Diagnostics.AddError("Client error", fmt.Sprintf("Docker configuration job failed: %s", err))
			return
		}
	}

	data.ID = types.StringValue("docker")

	r.readDockerConfig(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *dockerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data dockerResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.readDockerConfig(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *dockerResource) readDockerConfig(ctx context.Context, data *dockerResourceModel, diags *diag.Diagnostics) {
	apiResp, err := r.client.Call(ctx, "docker.config", nil)
	if err != nil {
		diags.AddError("Client error", fmt.Sprintf("Unable to read Docker config: %s", err))
		return
	}

	var config map[string]any
	if err := json.Unmarshal(apiResp.Result, &config); err != nil {
		diags.AddError("Client error", "Unable to parse Docker config response")
		return
	}

	data.ID = types.StringValue("docker")

	if pool, ok := config["pool"].(string); ok {
		data.Pool = types.StringValue(pool)
	}

	if enableUpdates, ok := config["enable_image_updates"].(bool); ok {
		data.EnableImageUpdates = types.BoolValue(enableUpdates)
	}

	if nvidia, ok := config["nvidia"].(bool); ok {
		data.Nvidia = types.BoolValue(nvidia)
	}
}

func (r *dockerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data dockerResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := map[string]any{
		"pool": data.Pool.ValueString(),
	}

	if !data.EnableImageUpdates.IsNull() && !data.EnableImageUpdates.IsUnknown() {
		params["enable_image_updates"] = data.EnableImageUpdates.ValueBool()
	}

	if !data.Nvidia.IsNull() && !data.Nvidia.IsUnknown() {
		params["nvidia"] = data.Nvidia.ValueBool()
	}

	apiResp, err := r.client.Call(ctx, "docker.update", []any{params})
	if err != nil {
		resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to update Docker config: %s", err))
		return
	}

	var jobID int64
	if err := json.Unmarshal(apiResp.Result, &jobID); err == nil && jobID > 0 {
		if err := r.client.WaitForJob(ctx, jobID); err != nil {
			resp.Diagnostics.AddError("Client error", fmt.Sprintf("Docker configuration job failed: %s", err))
			return
		}
	}

	r.readDockerConfig(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *dockerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	params := map[string]any{
		"pool": nil,
	}

	apiResp, err := r.client.Call(ctx, "docker.update", []any{params})
	if err != nil {
		resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to clear Docker config: %s", err))
		return
	}

	var jobID int64
	if err := json.Unmarshal(apiResp.Result, &jobID); err == nil && jobID > 0 {
		if err := r.client.WaitForJob(ctx, jobID); err != nil {
			resp.Diagnostics.AddError("Client error", fmt.Sprintf("Docker configuration job failed: %s", err))
			return
		}
	}
}
