package provider

import (
	"context"

	"github.com/anatoliacore/terraform-provider-anatoliacore/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*volumeResource)(nil)
var _ resource.ResourceWithConfigure = (*volumeResource)(nil)
var _ resource.ResourceWithImportState = (*volumeResource)(nil)

type volumeResource struct{ client *client.Client }
type volumeModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	SizeGB     types.Int64  `tfsdk:"size_gb"`
	InstanceID types.String `tfsdk:"instance_id"`
	Status     types.String `tfsdk:"status"`
}

func NewVolumeResource() resource.Resource { return &volumeResource{} }
func (r *volumeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_volume"
}
func (r *volumeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":          schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"name":        schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"size_gb":     schema.Int64Attribute{Required: true, Validators: []validator.Int64{int64validator.Between(1, 2000)}},
		"instance_id": schema.StringAttribute{Optional: true},
		"status":      schema.StringAttribute{Computed: true},
	}}
}
func (r *volumeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	configureClient(req.ProviderData, &r.client, &resp.Diagnostics)
}
func setVolumeState(data *volumeModel, value *client.Volume) {
	data.ID = types.StringValue(value.ID)
	data.Name = types.StringValue(value.Name)
	data.SizeGB = types.Int64Value(value.SizeGB)
	data.InstanceID = types.StringPointerValue(value.InstanceID)
	data.Status = types.StringValue(value.Status)
}
func (r *volumeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data volumeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	value, err := r.client.CreateVolume(ctx, data.Name.ValueString(), data.SizeGB.ValueInt64())
	if err == nil && !data.InstanceID.IsNull() && !data.InstanceID.IsUnknown() {
		createdID := value.ID
		value, err = r.client.AttachVolume(ctx, createdID, data.InstanceID.ValueString())
		if err != nil {
			// Do not leak an untracked billable volume when the second half of
			// create fails. The API's idempotency ledger makes this compensation
			// safe to retry.
			_ = r.client.DeleteVolume(ctx, createdID)
		}
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to create volume", err.Error())
		return
	}
	setVolumeState(&data, value)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *volumeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data volumeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	value, err := r.client.GetVolume(ctx, data.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read volume", err.Error())
		return
	}
	setVolumeState(&data, value)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *volumeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state volumeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	value, err := r.client.GetVolume(ctx, state.ID.ValueString())
	if err == nil && plan.SizeGB.ValueInt64() != state.SizeGB.ValueInt64() {
		value, err = r.client.ResizeVolume(ctx, state.ID.ValueString(), plan.SizeGB.ValueInt64())
	}
	oldInstance, newInstance := optionalString(state.InstanceID), optionalString(plan.InstanceID)
	if err == nil && ((oldInstance == nil) != (newInstance == nil) || (oldInstance != nil && newInstance != nil && *oldInstance != *newInstance)) {
		if oldInstance != nil {
			value, err = r.client.DetachVolume(ctx, state.ID.ValueString())
		}
		if err == nil && newInstance != nil {
			value, err = r.client.AttachVolume(ctx, state.ID.ValueString(), *newInstance)
		}
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to update volume", err.Error())
		return
	}
	setVolumeState(&plan, value)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *volumeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data volumeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !data.InstanceID.IsNull() {
		_, _ = r.client.DetachVolume(ctx, data.ID.ValueString())
	}
	if err := r.client.DeleteVolume(ctx, data.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete volume", err.Error())
	}
}
func (r *volumeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
