package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"truenas/internal/client"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &poolResource{}

type useStateIfExistsModifier struct{}

func (m useStateIfExistsModifier) Description(ctx context.Context) string {
	return "use state value if resource already exists, unless force_recreate is set"
}

func (m useStateIfExistsModifier) MarkdownDescription(ctx context.Context) string {
	return "use state value if resource already exists, unless force_recreate is set"
}

func (m useStateIfExistsModifier) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	if req.StateValue.IsNull() {
		return
	}

	var forceRecreate types.Bool
	req.Config.GetAttribute(ctx, path.Root("force_recreate"), &forceRecreate)
	if !forceRecreate.IsNull() && forceRecreate.ValueBool() {
		if !req.ConfigValue.Equal(req.StateValue) {
			resp.RequiresReplace = true
		}
		return
	}

	resp.PlanValue = req.StateValue
}

func NewPoolResource() resource.Resource {
	return &poolResource{}
}

type poolResource struct {
	client *client.Client
}

type poolResourceModel struct {
	ID            types.String   `tfsdk:"id"`
	Name          types.String   `tfsdk:"name"`
	ForceRecreate types.Bool     `tfsdk:"force_recreate"`
	Topology      *topologyModel `tfsdk:"topology"`
}

type topologyModel struct {
	Data []vdevModel `tfsdk:"data"`
}

type vdevModel struct {
	Type  types.String `tfsdk:"type"`
	Disks types.List   `tfsdk:"disks"`
}

func (r *poolResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pool"
}

func (r *poolResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "manages a truenas storage pool.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"force_recreate": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "when true, disables state preservation for disks allowing config changes to trigger recreation.",
			},
		},
		Blocks: map[string]schema.Block{
			"topology": schema.SingleNestedBlock{
				Blocks: map[string]schema.Block{
					"data": schema.ListNestedBlock{
						NestedObject: schema.NestedBlockObject{
							Attributes: map[string]schema.Attribute{
								"type": schema.StringAttribute{
									Required:            true,
									MarkdownDescription: "vdev type: stripe, mirror, raidz1, raidz2, raidz3",
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.RequiresReplace(),
									},
								},
								"disks": schema.ListAttribute{
									ElementType:         types.StringType,
									Optional:            true,
									Computed:            true,
									MarkdownDescription: "disks in this vdev. only used during creation; subsequent reads reflect actual pool state.",
									PlanModifiers: []planmodifier.List{
										useStateIfExistsModifier{},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *poolResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *poolResource) waitForSystemDataset(ctx context.Context) error {
	for {
		filters := []any{
			[]any{"method", "=", "systemdataset.update"},
			[]any{"state", "=", "RUNNING"},
		}
		apiResp, err := r.client.Call(ctx, "core.get_jobs", []any{filters})
		if err != nil {
			return fmt.Errorf("failed to query running jobs: %w", err)
		}

		var jobs []map[string]any
		if err := json.Unmarshal(apiResp.Result, &jobs); err != nil {
			return fmt.Errorf("failed to parse jobs response: %w", err)
		}

		if len(jobs) == 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (r *poolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data poolResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.waitForSystemDataset(ctx); err != nil {
		resp.Diagnostics.AddError("Client error", fmt.Sprintf("Error waiting for system dataset: %s", err))
		return
	}

	// construct api payload
	topology := map[string]any{}
	if data.Topology != nil {
		var dataVdevs []any
		for _, vdev := range data.Topology.Data {
			var diskStrings []string
			if !vdev.Disks.IsNull() && !vdev.Disks.IsUnknown() {
				var diskValues []types.String
				vdev.Disks.ElementsAs(ctx, &diskValues, false)
				for _, d := range diskValues {
					diskStrings = append(diskStrings, d.ValueString())
				}
			}

			dataVdevs = append(dataVdevs, map[string]any{
				"type":  strings.ToUpper(vdev.Type.ValueString()),
				"disks": diskStrings,
			})
		}
		topology["data"] = dataVdevs
	}

	params := []any{
		map[string]any{
			"name":     data.Name.ValueString(),
			"topology": topology,
		},
	}

	apiResp, err := r.client.Call(ctx, "pool.create", params)
	if err != nil {
		resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to create pool: %s", err))
		return
	}

	// check if response is a job ID (int) and wait for it
	var jobID int64
	if err := json.Unmarshal(apiResp.Result, &jobID); err == nil {
		if err := r.client.WaitForJob(ctx, jobID); err != nil {
			resp.Diagnostics.AddError("Client error", fmt.Sprintf("Pool creation job failed: %s", err))
			return
		}
	}

	data.ID = data.Name

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *poolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data poolResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	filters := []any{
		[]any{"name", "=", data.Name.ValueString()},
	}

	apiResp, err := r.client.Call(ctx, "pool.query", []any{filters})
	if err != nil {
		resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to read pool: %s", err))
		return
	}

	// parse response. it should be a list of pools.
	var pools []map[string]any
	if err := json.Unmarshal(apiResp.Result, &pools); err != nil {
		resp.Diagnostics.AddError("Client error", "Unable to parse pool query response")
		return
	}

	if len(pools) == 0 {
		// pool not found, remove from state
		resp.State.RemoveResource(ctx)
		return
	}

	pool := pools[0]
	data.ID = types.StringValue(pool["name"].(string))
	data.Name = types.StringValue(pool["name"].(string))

	if topology, ok := pool["topology"].(map[string]any); ok {
		if dataVdevs, ok := topology["data"].([]any); ok {
			var vdevs []vdevModel
			for _, v := range dataVdevs {
				vdevMap, ok := v.(map[string]any)
				if !ok {
					continue
				}

				vdevType := ""
				if t, ok := vdevMap["type"].(string); ok {
					vdevType = t
					if vdevType == "DISK" {
						vdevType = "STRIPE"
					}
				}

				var diskStrings []string
				if children, ok := vdevMap["children"].([]any); ok && len(children) > 0 {
					for _, c := range children {
						if childMap, ok := c.(map[string]any); ok {
							if disk, ok := childMap["disk"].(string); ok {
								diskStrings = append(diskStrings, disk)
							}
						}
					}
				} else if disk, ok := vdevMap["disk"].(string); ok {
					diskStrings = append(diskStrings, disk)
				}

				diskList, _ := types.ListValueFrom(ctx, types.StringType, diskStrings)
				vdevs = append(vdevs, vdevModel{
					Type:  types.StringValue(vdevType),
					Disks: diskList,
				})
			}

			if len(vdevs) > 0 {
				data.Topology = &topologyModel{
					Data: vdevs,
				}
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *poolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// TODO: implement pool updates (adding vdevs, changing properties)
}

func (r *poolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data poolResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.waitForSystemDataset(ctx); err != nil {
		resp.Diagnostics.AddError("Client error", fmt.Sprintf("Error waiting for system dataset: %s", err))
		return
	}

	// get numeric id
	filters := []any{
		[]any{"name", "=", data.Name.ValueString()},
	}
	apiResp, err := r.client.Call(ctx, "pool.query", []any{filters})
	if err != nil {
		resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to find pool for deletion: %s", err))
		return
	}

	// call pool.export (requires numeric id)
	var pools []map[string]any
	if err := json.Unmarshal(apiResp.Result, &pools); err != nil {
		resp.Diagnostics.AddError("Client error", "Unable to parse pool query response")
		return
	}

	if len(pools) == 0 {
		// pool already gone
		return
	}

	poolID := pools[0]["id"]
	// call pool.export(id, options)
	exportResp, err := r.client.Call(ctx, "pool.export", []any{poolID, map[string]any{"destroy": true, "cascade": true}})
	if err != nil {
		resp.Diagnostics.AddError("Client error", fmt.Sprintf("Unable to export/destroy pool: %s", err))
		return
	}

	// check if response is a job id (int)
	var jobID int64
	if err := json.Unmarshal(exportResp.Result, &jobID); err == nil {
		// it's a job, wait for it
		if err := r.client.WaitForJob(ctx, jobID); err != nil {
			resp.Diagnostics.AddError("Client error", fmt.Sprintf("Pool deletion job failed: %s", err))
			return
		}
	}
}

func (r *poolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}
