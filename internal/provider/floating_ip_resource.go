package provider

import (
	"context"

	"github.com/anatoliacore/terraform-provider-anatoliacore/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*floatingIPResource)(nil)
var _ resource.ResourceWithConfigure = (*floatingIPResource)(nil)
var _ resource.ResourceWithImportState = (*floatingIPResource)(nil)

type floatingIPResource struct{ client *client.Client }
type floatingIPModel struct {
	ID              types.String `tfsdk:"id"`
	VDCID           types.String `tfsdk:"vdc_id"`
	InstanceID      types.String `tfsdk:"instance_id"`
	IPAddress       types.String `tfsdk:"ip_address"`
	Status          types.String `tfsdk:"status"`
	DesiredRevision types.Int64  `tfsdk:"desired_revision"`
	AppliedRevision types.Int64  `tfsdk:"applied_revision"`
}

func NewFloatingIPResource() resource.Resource { return &floatingIPResource{} }
func (r *floatingIPResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_floating_ip"
}
func (r *floatingIPResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id":          schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"vdc_id":      schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"instance_id": schema.StringAttribute{Optional: true}, "ip_address": schema.StringAttribute{Computed: true},
		"status": schema.StringAttribute{Computed: true}, "desired_revision": schema.Int64Attribute{Computed: true},
		"applied_revision": schema.Int64Attribute{Computed: true},
	}}
}
func (r *floatingIPResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	configureClient(req.ProviderData, &r.client, &resp.Diagnostics)
}
func setFloatingIPState(data *floatingIPModel, value *client.FloatingIP) {
	data.ID = types.StringValue(value.ID)
	data.VDCID = types.StringPointerValue(value.VDCID)
	data.InstanceID = types.StringPointerValue(value.DesiredInstanceID)
	data.IPAddress = types.StringValue(value.IPAddress)
	data.Status = types.StringValue(value.Status)
	data.DesiredRevision = types.Int64Value(value.DesiredRevision)
	data.AppliedRevision = types.Int64Value(value.AppliedRevision)
}
func (r *floatingIPResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data floatingIPModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	value, err := r.client.CreateFloatingIP(ctx, data.VDCID.ValueString())
	if err == nil && !data.InstanceID.IsNull() {
		createdID := value.ID
		value, err = r.client.AssociateFloatingIP(ctx, createdID, data.InstanceID.ValueString())
		if err != nil {
			// Allocation succeeded but association did not; release the address
			// before returning so Terraform never loses ownership of a billable IP.
			_ = r.client.DeleteFloatingIP(ctx, createdID)
		}
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to create floating IP", err.Error())
		return
	}
	setFloatingIPState(&data, value)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *floatingIPResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data floatingIPModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	value, err := r.client.GetFloatingIP(ctx, data.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read floating IP", err.Error())
		return
	}
	setFloatingIPState(&data, value)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *floatingIPResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state floatingIPModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	oldID, newID := optionalString(state.InstanceID), optionalString(plan.InstanceID)
	value, err := r.client.GetFloatingIP(ctx, state.ID.ValueString())
	if err == nil && ((oldID == nil) != (newID == nil) || (oldID != nil && newID != nil && *oldID != *newID)) {
		if oldID != nil {
			value, err = r.client.DisassociateFloatingIP(ctx, state.ID.ValueString())
		}
		if err == nil && newID != nil {
			value, err = r.client.AssociateFloatingIP(ctx, state.ID.ValueString(), *newID)
		}
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to update floating IP", err.Error())
		return
	}
	setFloatingIPState(&plan, value)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *floatingIPResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data floatingIPModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !data.InstanceID.IsNull() {
		_, _ = r.client.DisassociateFloatingIP(ctx, data.ID.ValueString())
	}
	if err := r.client.DeleteFloatingIP(ctx, data.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete floating IP", err.Error())
	}
}
func (r *floatingIPResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
