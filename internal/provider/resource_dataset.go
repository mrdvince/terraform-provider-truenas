package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"truenas/internal/client"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &datasetResource{}
var _ resource.ResourceWithImportState = &datasetResource{}

func NewDatasetResource() resource.Resource {
	return &datasetResource{}
}

type datasetResource struct {
	client *client.Client
}

type datasetResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Parent      types.String `tfsdk:"parent"`
	Pool        types.String `tfsdk:"pool"`
	Mountpoint  types.String `tfsdk:"mountpoint"`
	Comments    types.String `tfsdk:"comments"`
	Compression types.String `tfsdk:"compression"`
}

func (r *datasetResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset"
}

func (r *datasetResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a ZFS dataset.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The full path of the dataset (pool/name or pool/parent/name).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The dataset name.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"parent": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The parent dataset path (pool name or parent dataset id).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"pool": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The pool containing this dataset.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"mountpoint": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The filesystem mountpoint.",
			},
			"comments": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "User comments for the dataset.",
			},
			"compression": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Compression algorithm (OFF, LZ4, GZIP, ZLE, LZJB, ZSTD).",
			},
		},
	}
}

func (r *datasetResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *datasetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data datasetResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fullPath := data.Parent.ValueString() + "/" + data.Name.ValueString()

	createParams := map[string]any{
		"name": fullPath,
	}

	if !data.Comments.IsNull() && !data.Comments.IsUnknown() {
		createParams["comments"] = data.Comments.ValueString()
	}

	if !data.Compression.IsNull() && !data.Compression.IsUnknown() {
		createParams["compression"] = strings.ToUpper(data.Compression.ValueString())
	}

	_, err := r.client.Call(ctx, "pool.dataset.create", []any{createParams})
	if err != nil {
		resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to create dataset: %s", err))
		return
	}

	data.ID = types.StringValue(fullPath)

	parts := strings.SplitN(fullPath, "/", 2)
	data.Pool = types.StringValue(parts[0])

	diags, found := r.readDataset(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || !found {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *datasetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data datasetResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags, found := r.readDataset(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *datasetResource) readDataset(ctx context.Context, data *datasetResourceModel) (diag.Diagnostics, bool) {
	var diags diag.Diagnostics
	filters := []any{
		[]any{"id", "=", data.ID.ValueString()},
	}

	apiResp, err := r.client.Call(ctx, "pool.dataset.query", []any{filters})
	if err != nil {
		diags.AddError("Client error", fmt.Sprintf("Unable to read dataset: %s", err))
		return diags, false
	}

	var datasets []map[string]any
	if err := json.Unmarshal(apiResp.Result, &datasets); err != nil {
		diags.AddError("Client error", "Unable to parse dataset query response")
		return diags, false
	}

	if len(datasets) == 0 {
		return diags, false
	}

	dataset := datasets[0]

	data.ID = types.StringValue(dataset["id"].(string))

	if name, ok := dataset["name"].(string); ok {
		parts := strings.Split(name, "/")
		data.Name = types.StringValue(parts[len(parts)-1])
	}

	if pool, ok := dataset["pool"].(string); ok {
		data.Pool = types.StringValue(pool)
	}

	if mountpoint, ok := dataset["mountpoint"].(string); ok {
		data.Mountpoint = types.StringValue(mountpoint)
	}

	if comments, ok := dataset["comments"].(map[string]any); ok {
		if value, ok := comments["value"].(string); ok {
			data.Comments = types.StringValue(value)
		} else {
			data.Comments = types.StringValue("")
		}
	} else {
		data.Comments = types.StringValue("")
	}

	if compression, ok := dataset["compression"].(map[string]any); ok {
		if value, ok := compression["value"].(string); ok {
			data.Compression = types.StringValue(value)
		} else {
			data.Compression = types.StringValue("")
		}
	} else {
		data.Compression = types.StringValue("")
	}

	return diags, true
}

func (r *datasetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data datasetResourceModel
	var state datasetResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateParams := map[string]any{}

	if !data.Comments.IsNull() && !data.Comments.IsUnknown() {
		updateParams["comments"] = data.Comments.ValueString()
	}

	if !data.Compression.IsNull() && !data.Compression.IsUnknown() {
		updateParams["compression"] = strings.ToUpper(data.Compression.ValueString())
	}

	if len(updateParams) > 0 {
		_, err := r.client.Call(ctx, "pool.dataset.update", []any{state.ID.ValueString(), updateParams})
		if err != nil {
			resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to update dataset: %s", err))
			return
		}
	}

	data.ID = state.ID
	data.Pool = state.Pool

	diags, found := r.readDataset(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || !found {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *datasetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data datasetResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "pool.dataset.delete", []any{data.ID.ValueString()})
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "does not exist") {
			return
		}
		resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to delete dataset: %s", err))
		return
	}
}

func (r *datasetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	datasetID := req.ID

	parts := strings.Split(datasetID, "/")
	if len(parts) < 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Dataset ID must be in format pool/name or pool/parent/name")
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), datasetID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), parts[len(parts)-1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("parent"), strings.Join(parts[:len(parts)-1], "/"))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("pool"), parts[0])...)
}
