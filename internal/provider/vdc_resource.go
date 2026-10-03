package provider

import (
	"context"
	"regexp"

	"github.com/anatoliacore/terraform-provider-anatoliacore/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*vdcResource)(nil)
var _ resource.ResourceWithConfigure = (*vdcResource)(nil)
var _ resource.ResourceWithImportState = (*vdcResource)(nil)

type vdcResource struct{ client *client.Client }
type vdcModel struct {
	ID            types.String  `tfsdk:"id"`
	Name          types.String  `tfsdk:"name"`
	CPULimitMHz   types.Int64   `tfsdk:"cpu_limit_mhz"`
	RAMLimitMB    types.Int64   `tfsdk:"ram_limit_mb"`
	DiskLimitGB   types.Int64   `tfsdk:"disk_limit_gb"`
	Status        types.String  `tfsdk:"status"`
	HourlyPrice   types.Float64 `tfsdk:"hourly_price"`
	PublicIPCount types.Int64   `tfsdk:"public_ip_count"`
	PublicIPs     types.List    `tfsdk:"public_ips"`
}

func NewVDCResource() resource.Resource { return &vdcResource{} }
func (r *vdcResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vdc"
}
func (r *vdcResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
		}},
		"name": schema.StringAttribute{
			Required: true,
			Validators: []validator.String{
				stringvalidator.UTF8LengthBetween(1, 63),
				stringvalidator.RegexMatches(regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`), "must start with an alphanumeric character and contain only letters, numbers, dots, underscores, or hyphens"),
			},
		},
		"cpu_limit_mhz": schema.Int64Attribute{
			Required:   true,
			Validators: []validator.Int64{int64validator.Between(1000, 1000000)},
		},
		"ram_limit_mb": schema.Int64Attribute{
			Required: true,
			Validators: []validator.Int64{
				int64validator.Between(1024, 4194304),
				multipleOfInt64(256),
			},
		},
		"disk_limit_gb": schema.Int64Attribute{
			Required:   true,
			Validators: []validator.Int64{int64validator.Between(10, 1048576)},
		},
		"public_ip_count": schema.Int64Attribute{
			Optional:      true,
			Default:       int64default.StaticInt64(1),
			PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			Validators:    []validator.Int64{int64validator.Between(1, 5)},
		},
		"public_ips":   schema.ListAttribute{Computed: true, ElementType: types.StringType},
		"status":       schema.StringAttribute{Computed: true},
		"hourly_price": schema.Float64Attribute{Computed: true},
	}}
}
func (r *vdcResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	configureClient(req.ProviderData, &r.client, &resp.Diagnostics)
}
func setVDCState(data *vdcModel, vdc *client.VDC) {
	data.ID = types.StringValue(vdc.ID)
	data.Name = types.StringValue(vdc.Name)
	data.CPULimitMHz = types.Int64Value(vdc.CPULimitMHz)
	data.RAMLimitMB = types.Int64Value(vdc.RAMLimitMB)
	data.DiskLimitGB = types.Int64Value(vdc.DiskLimitGB)
	data.Status = types.StringValue(vdc.Status)
	data.HourlyPrice = types.Float64Value(vdc.HourlyPrice)
	data.PublicIPCount = types.Int64Value(vdc.PublicIPCount)
	values := make([]attr.Value, len(vdc.PublicIPs))
	for index, address := range vdc.PublicIPs {
		values[index] = types.StringValue(address)
	}
	data.PublicIPs = types.ListValueMust(types.StringType, values)
}
func (r *vdcResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data vdcModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	vdc, err := r.client.CreateVDC(ctx, map[string]any{
		"name": data.Name.ValueString(), "cpu_limit_mhz": data.CPULimitMHz.ValueInt64(),
		"ram_limit_mb": data.RAMLimitMB.ValueInt64(), "disk_limit_gb": data.DiskLimitGB.ValueInt64(),
		"public_ip_count": data.PublicIPCount.ValueInt64(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create VDC", err.Error())
		return
	}
	setVDCState(&data, vdc)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *vdcResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data vdcModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	vdc, err := r.client.GetVDC(ctx, data.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read VDC", err.Error())
		return
	}
	setVDCState(&data, vdc)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *vdcResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data vdcModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	vdc, err := r.client.UpdateVDC(ctx, data.ID.ValueString(), map[string]any{
		"name": data.Name.ValueString(), "cpu_limit_mhz": data.CPULimitMHz.ValueInt64(),
		"ram_limit_mb": data.RAMLimitMB.ValueInt64(), "disk_limit_gb": data.DiskLimitGB.ValueInt64(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to update VDC", err.Error())
		return
	}
	setVDCState(&data, vdc)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *vdcResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data vdcModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteVDC(ctx, data.ID.ValueString(), data.Name.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete VDC", err.Error())
	}
}
func (r *vdcResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
