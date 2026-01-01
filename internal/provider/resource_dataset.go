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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &datasetResource{}
var _ resource.ResourceWithImportState = &datasetResource{}

var aclPresets = map[string]struct {
	acltype string
	dacl    []map[string]any
}{
	"NFS4_OPEN": {
		acltype: "NFS4",
		dacl: []map[string]any{
			{"tag": "owner@", "type": "ALLOW", "perms": map[string]any{"BASIC": "FULL_CONTROL"}, "flags": map[string]any{"BASIC": "INHERIT"}},
			{"tag": "group@", "type": "ALLOW", "perms": map[string]any{"BASIC": "FULL_CONTROL"}, "flags": map[string]any{"BASIC": "INHERIT"}},
			{"tag": "everyone@", "type": "ALLOW", "perms": map[string]any{"BASIC": "MODIFY"}, "flags": map[string]any{"BASIC": "INHERIT"}},
		},
	},
	"NFS4_RESTRICTED": {
		acltype: "NFS4",
		dacl: []map[string]any{
			{"tag": "owner@", "type": "ALLOW", "perms": map[string]any{"BASIC": "FULL_CONTROL"}, "flags": map[string]any{"BASIC": "INHERIT"}},
			{"tag": "group@", "type": "ALLOW", "perms": map[string]any{"BASIC": "MODIFY"}, "flags": map[string]any{"BASIC": "INHERIT"}},
		},
	},
	"NFS4_HOME": {
		acltype: "NFS4",
		dacl: []map[string]any{
			{"tag": "owner@", "type": "ALLOW", "perms": map[string]any{"BASIC": "FULL_CONTROL"}, "flags": map[string]any{"BASIC": "INHERIT"}},
			{"tag": "group@", "type": "ALLOW", "perms": map[string]any{"BASIC": "MODIFY"}, "flags": map[string]any{"BASIC": "NOINHERIT"}},
			{"tag": "everyone@", "type": "ALLOW", "perms": map[string]any{"BASIC": "TRAVERSE"}, "flags": map[string]any{"BASIC": "NOINHERIT"}},
		},
	},
	"NFS4_ADMIN": {
		acltype: "NFS4",
		dacl: []map[string]any{
			{"tag": "owner@", "type": "ALLOW", "perms": map[string]any{"BASIC": "FULL_CONTROL"}, "flags": map[string]any{"BASIC": "INHERIT"}},
			{"tag": "group@", "type": "ALLOW", "perms": map[string]any{"BASIC": "TRAVERSE"}, "flags": map[string]any{"BASIC": "INHERIT"}},
		},
	},
	"POSIX_OPEN": {
		acltype: "POSIX1E",
		dacl: []map[string]any{
			{"tag": "USER_OBJ", "perms": map[string]any{"READ": true, "WRITE": true, "EXECUTE": true}, "default": true, "id": -1},
			{"tag": "GROUP_OBJ", "perms": map[string]any{"READ": true, "WRITE": true, "EXECUTE": true}, "default": true, "id": -1},
			{"tag": "OTHER", "perms": map[string]any{"READ": true, "WRITE": true, "EXECUTE": true}, "default": true, "id": -1},
			{"tag": "USER_OBJ", "perms": map[string]any{"READ": true, "WRITE": true, "EXECUTE": true}, "default": false, "id": -1},
			{"tag": "GROUP_OBJ", "perms": map[string]any{"READ": true, "WRITE": true, "EXECUTE": true}, "default": false, "id": -1},
			{"tag": "OTHER", "perms": map[string]any{"READ": true, "WRITE": true, "EXECUTE": true}, "default": false, "id": -1},
		},
	},
	"POSIX_RESTRICTED": {
		acltype: "POSIX1E",
		dacl: []map[string]any{
			{"tag": "USER_OBJ", "perms": map[string]any{"READ": true, "WRITE": true, "EXECUTE": true}, "default": true, "id": -1},
			{"tag": "GROUP_OBJ", "perms": map[string]any{"READ": true, "WRITE": true, "EXECUTE": true}, "default": true, "id": -1},
			{"tag": "OTHER", "perms": map[string]any{"READ": false, "WRITE": false, "EXECUTE": false}, "default": true, "id": -1},
			{"tag": "USER_OBJ", "perms": map[string]any{"READ": true, "WRITE": true, "EXECUTE": true}, "default": false, "id": -1},
			{"tag": "GROUP_OBJ", "perms": map[string]any{"READ": true, "WRITE": true, "EXECUTE": true}, "default": false, "id": -1},
			{"tag": "OTHER", "perms": map[string]any{"READ": false, "WRITE": false, "EXECUTE": false}, "default": false, "id": -1},
		},
	},
	"POSIX_HOME": {
		acltype: "POSIX1E",
		dacl: []map[string]any{
			{"tag": "USER_OBJ", "perms": map[string]any{"READ": true, "WRITE": true, "EXECUTE": true}, "default": true, "id": -1},
			{"tag": "GROUP_OBJ", "perms": map[string]any{"READ": true, "WRITE": true, "EXECUTE": true}, "default": true, "id": -1},
			{"tag": "OTHER", "perms": map[string]any{"READ": false, "WRITE": false, "EXECUTE": false}, "default": true, "id": -1},
			{"tag": "USER_OBJ", "perms": map[string]any{"READ": true, "WRITE": true, "EXECUTE": true}, "default": false, "id": -1},
			{"tag": "GROUP_OBJ", "perms": map[string]any{"READ": true, "WRITE": true, "EXECUTE": true}, "default": false, "id": -1},
			{"tag": "OTHER", "perms": map[string]any{"READ": true, "WRITE": false, "EXECUTE": true}, "default": false, "id": -1},
		},
	},
}

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
	PoolID      types.Int64  `tfsdk:"pool_id"`
	Pool        types.String `tfsdk:"pool"`
	Mountpoint  types.String `tfsdk:"mountpoint"`
	Comments    types.String `tfsdk:"comments"`
	Compression types.String `tfsdk:"compression"`
	Quota       types.Int64  `tfsdk:"quota"`
	Refquota    types.Int64  `tfsdk:"refquota"`
	Snapdir     types.String `tfsdk:"snapdir"`
	Acltype     types.String `tfsdk:"acltype"`
	Aclmode     types.String `tfsdk:"aclmode"`
	AclPreset   types.String `tfsdk:"acl_preset"`
	Sync        types.String `tfsdk:"sync"`
	Atime       types.String `tfsdk:"atime"`
	Readonly    types.String `tfsdk:"readonly"`
	Exec        types.String `tfsdk:"exec"`
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
			"pool_id": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "The pool's internal id. set this to truenas_pool.*.pool_id to automatically recreate datasets when the pool is replaced.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
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
			"quota": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Maximum space for dataset and descendants in bytes (0 = no quota).",
			},
			"refquota": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Maximum space for dataset only, excluding snapshots and descendants, in bytes (0 = no refquota).",
			},
			"snapdir": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Snapshot directory visibility (VISIBLE, HIDDEN, DISABLED).",
			},
			"acltype": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "ACL type (NFSV4, POSIX, OFF, INHERIT).",
			},
			"aclmode": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "ACL mode (PASSTHROUGH, RESTRICTED, DISCARD, INHERIT).",
			},
			"acl_preset": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "ACL preset to apply (NFS4_OPEN, NFS4_RESTRICTED, NFS4_HOME, NFS4_ADMIN, POSIX_OPEN, POSIX_RESTRICTED, POSIX_HOME, POSIX_ADMIN). Sets both acltype and applies the ACL entries with inheritance.",
			},
			"sync": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Sync mode (STANDARD, ALWAYS, DISABLED).",
			},
			"atime": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Access time updates (ON, OFF).",
			},
			"readonly": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Read-only mode (ON, OFF).",
			},
			"exec": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Allow execution of binaries (ON, OFF).",
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

func (r *datasetResource) applyACLPreset(ctx context.Context, mountpoint string, presetName string) error {
	preset, ok := aclPresets[strings.ToUpper(presetName)]
	if !ok {
		return fmt.Errorf("unknown ACL preset: %s", presetName)
	}

	params := []any{
		map[string]any{
			"path":    mountpoint,
			"dacl":    preset.dacl,
			"acltype": preset.acltype,
		},
	}

	apiResp, err := r.client.Call(ctx, "filesystem.setacl", params)
	if err != nil {
		return fmt.Errorf("failed to apply ACL preset: %w", err)
	}

	var jobID int64
	if err := json.Unmarshal(apiResp.Result, &jobID); err == nil {
		if err := r.client.WaitForJob(ctx, jobID); err != nil {
			return fmt.Errorf("ACL preset job failed: %w", err)
		}
	}

	return nil
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

	if !data.Quota.IsNull() && !data.Quota.IsUnknown() {
		createParams["quota"] = data.Quota.ValueInt64()
	}

	if !data.Refquota.IsNull() && !data.Refquota.IsUnknown() {
		createParams["refquota"] = data.Refquota.ValueInt64()
	}

	if !data.Snapdir.IsNull() && !data.Snapdir.IsUnknown() {
		createParams["snapdir"] = strings.ToUpper(data.Snapdir.ValueString())
	}

	if !data.AclPreset.IsNull() && !data.AclPreset.IsUnknown() {
		preset, ok := aclPresets[strings.ToUpper(data.AclPreset.ValueString())]
		if ok {
			if preset.acltype == "NFS4" {
				createParams["acltype"] = "NFSV4"
			} else {
				createParams["acltype"] = "POSIX"
			}
		}
	} else if !data.Acltype.IsNull() && !data.Acltype.IsUnknown() {
		createParams["acltype"] = strings.ToUpper(data.Acltype.ValueString())
	}

	if !data.Aclmode.IsNull() && !data.Aclmode.IsUnknown() {
		createParams["aclmode"] = strings.ToUpper(data.Aclmode.ValueString())
	}

	if !data.Sync.IsNull() && !data.Sync.IsUnknown() {
		createParams["sync"] = strings.ToUpper(data.Sync.ValueString())
	}

	if !data.Atime.IsNull() && !data.Atime.IsUnknown() {
		createParams["atime"] = strings.ToUpper(data.Atime.ValueString())
	}

	if !data.Readonly.IsNull() && !data.Readonly.IsUnknown() {
		createParams["readonly"] = strings.ToUpper(data.Readonly.ValueString())
	}

	if !data.Exec.IsNull() && !data.Exec.IsUnknown() {
		createParams["exec"] = strings.ToUpper(data.Exec.ValueString())
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

	if !data.AclPreset.IsNull() && !data.AclPreset.IsUnknown() && !data.Mountpoint.IsNull() {
		if err := r.applyACLPreset(ctx, data.Mountpoint.ValueString(), data.AclPreset.ValueString()); err != nil {
			resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to apply ACL preset: %s", err))
			return
		}
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

	if quota, ok := dataset["quota"].(map[string]any); ok {
		if parsed, ok := quota["parsed"].(float64); ok {
			data.Quota = types.Int64Value(int64(parsed))
		} else {
			data.Quota = types.Int64Value(0)
		}
	} else {
		data.Quota = types.Int64Value(0)
	}

	if refquota, ok := dataset["refquota"].(map[string]any); ok {
		if parsed, ok := refquota["parsed"].(float64); ok {
			data.Refquota = types.Int64Value(int64(parsed))
		} else {
			data.Refquota = types.Int64Value(0)
		}
	} else {
		data.Refquota = types.Int64Value(0)
	}

	if snapdir, ok := dataset["snapdir"].(map[string]any); ok {
		if value, ok := snapdir["value"].(string); ok {
			data.Snapdir = types.StringValue(value)
		} else {
			data.Snapdir = types.StringValue("")
		}
	} else {
		data.Snapdir = types.StringValue("")
	}

	if acltype, ok := dataset["acltype"].(map[string]any); ok {
		if value, ok := acltype["value"].(string); ok {
			data.Acltype = types.StringValue(value)
		} else {
			data.Acltype = types.StringValue("")
		}
	} else {
		data.Acltype = types.StringValue("")
	}

	if aclmode, ok := dataset["aclmode"].(map[string]any); ok {
		if value, ok := aclmode["value"].(string); ok {
			data.Aclmode = types.StringValue(value)
		} else {
			data.Aclmode = types.StringValue("")
		}
	} else {
		data.Aclmode = types.StringValue("")
	}

	if sync, ok := dataset["sync"].(map[string]any); ok {
		if value, ok := sync["value"].(string); ok {
			data.Sync = types.StringValue(value)
		} else {
			data.Sync = types.StringValue("")
		}
	} else {
		data.Sync = types.StringValue("")
	}

	if atime, ok := dataset["atime"].(map[string]any); ok {
		if value, ok := atime["value"].(string); ok {
			data.Atime = types.StringValue(value)
		} else {
			data.Atime = types.StringValue("")
		}
	} else {
		data.Atime = types.StringValue("")
	}

	if readonly, ok := dataset["readonly"].(map[string]any); ok {
		if value, ok := readonly["value"].(string); ok {
			data.Readonly = types.StringValue(value)
		} else {
			data.Readonly = types.StringValue("")
		}
	} else {
		data.Readonly = types.StringValue("")
	}

	if exec, ok := dataset["exec"].(map[string]any); ok {
		if value, ok := exec["value"].(string); ok {
			data.Exec = types.StringValue(value)
		} else {
			data.Exec = types.StringValue("")
		}
	} else {
		data.Exec = types.StringValue("")
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

	if !data.Quota.IsNull() && !data.Quota.IsUnknown() {
		updateParams["quota"] = data.Quota.ValueInt64()
	}

	if !data.Refquota.IsNull() && !data.Refquota.IsUnknown() {
		updateParams["refquota"] = data.Refquota.ValueInt64()
	}

	if !data.Snapdir.IsNull() && !data.Snapdir.IsUnknown() {
		updateParams["snapdir"] = strings.ToUpper(data.Snapdir.ValueString())
	}

	aclPresetChanged := false
	if !data.AclPreset.IsNull() && !data.AclPreset.IsUnknown() {
		preset, ok := aclPresets[strings.ToUpper(data.AclPreset.ValueString())]
		if ok {
			if preset.acltype == "NFS4" {
				updateParams["acltype"] = "NFSV4"
			} else {
				updateParams["acltype"] = "POSIX"
			}
			aclPresetChanged = state.AclPreset.IsNull() || state.AclPreset.ValueString() != data.AclPreset.ValueString()
		}
	} else if !data.Acltype.IsNull() && !data.Acltype.IsUnknown() {
		updateParams["acltype"] = strings.ToUpper(data.Acltype.ValueString())
	}

	if !data.Aclmode.IsNull() && !data.Aclmode.IsUnknown() {
		updateParams["aclmode"] = strings.ToUpper(data.Aclmode.ValueString())
	}

	if !data.Sync.IsNull() && !data.Sync.IsUnknown() {
		updateParams["sync"] = strings.ToUpper(data.Sync.ValueString())
	}

	if !data.Atime.IsNull() && !data.Atime.IsUnknown() {
		updateParams["atime"] = strings.ToUpper(data.Atime.ValueString())
	}

	if !data.Readonly.IsNull() && !data.Readonly.IsUnknown() {
		updateParams["readonly"] = strings.ToUpper(data.Readonly.ValueString())
	}

	if !data.Exec.IsNull() && !data.Exec.IsUnknown() {
		updateParams["exec"] = strings.ToUpper(data.Exec.ValueString())
	}

	if len(updateParams) > 0 {
		apiResp, err := r.client.Call(ctx, "pool.dataset.update", []any{state.ID.ValueString(), updateParams})
		if err != nil {
			resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to update dataset: %s", err))
			return
		}

		var jobID int64
		if err := json.Unmarshal(apiResp.Result, &jobID); err == nil {
			if err := r.client.WaitForJob(ctx, jobID); err != nil {
				resp.Diagnostics.AddError("Client error", fmt.Sprintf("Dataset update job failed: %s", err))
				return
			}
		}
	}

	data.ID = state.ID
	data.Pool = state.Pool

	diags, found := r.readDataset(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || !found {
		return
	}

	if aclPresetChanged && !data.Mountpoint.IsNull() {
		if err := r.applyACLPreset(ctx, data.Mountpoint.ValueString(), data.AclPreset.ValueString()); err != nil {
			resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to apply ACL preset: %s", err))
			return
		}
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
