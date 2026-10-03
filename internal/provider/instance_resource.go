package provider

import (
	"context"
	"regexp"

	"github.com/anatoliacore/terraform-provider-anatoliacore/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*instanceResource)(nil)
var _ resource.ResourceWithConfigure = (*instanceResource)(nil)
var _ resource.ResourceWithImportState = (*instanceResource)(nil)

type instanceResource struct{ client *client.Client }

type instanceModel struct {
	ID           types.String  `tfsdk:"id"`
	Name         types.String  `tfsdk:"name"`
	CPUCores     types.Int64   `tfsdk:"cpu_cores"`
	RAMMB        types.Int64   `tfsdk:"ram_mb"`
	DiskGB       types.Int64   `tfsdk:"disk_gb"`
	OSTemplate   types.String  `tfsdk:"os_template"`
	VDCID        types.String  `tfsdk:"vdc_id"`
	SSHKeyID     types.String  `tfsdk:"ssh_key_id"`
	UserData     types.String  `tfsdk:"user_data"`
	IPAddress    types.String  `tfsdk:"ip_address"`
	Status       types.String  `tfsdk:"status"`
	PowerState   types.String  `tfsdk:"power_state"`
	HourlyPrice  types.Float64 `tfsdk:"hourly_price"`
	MonthlyPrice types.Float64 `tfsdk:"monthly_price"`
}

func NewInstanceResource() resource.Resource { return &instanceResource{} }

func (r *instanceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_instance"
}

func (r *instanceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	requiresReplace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
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
			PlanModifiers: requiresReplace,
		},
		"cpu_cores": schema.Int64Attribute{
			Required:   true,
			Validators: []validator.Int64{int64validator.Between(1, 64)},
		},
		"ram_mb": schema.Int64Attribute{
			Required: true,
			Validators: []validator.Int64{
				int64validator.Between(512, 262144),
				multipleOfInt64(256),
			},
		},
		"disk_gb": schema.Int64Attribute{
			Required:   true,
			Validators: []validator.Int64{int64validator.Between(10, 4096)},
		},
		"os_template": schema.StringAttribute{
			Required:      true,
			Validators:    []validator.String{stringvalidator.UTF8LengthBetween(1, 100)},
			PlanModifiers: requiresReplace,
		},
		"vdc_id":        schema.StringAttribute{Optional: true, PlanModifiers: requiresReplace},
		"ssh_key_id":    schema.StringAttribute{Optional: true, PlanModifiers: requiresReplace},
		"user_data":     schema.StringAttribute{Optional: true, Sensitive: true, PlanModifiers: requiresReplace},
		"ip_address":    schema.StringAttribute{Computed: true},
		"status":        schema.StringAttribute{Computed: true},
		"power_state":   schema.StringAttribute{Computed: true},
		"hourly_price":  schema.Float64Attribute{Computed: true},
		"monthly_price": schema.Float64Attribute{Computed: true},
	}}
}

func (r *instanceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	configureClient(req.ProviderData, &r.client, &resp.Diagnostics)
}

func optionalString(value types.String) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	result := value.ValueString()
	return &result
}

func setInstanceState(data *instanceModel, instance *client.Instance) {
	data.ID = types.StringValue(instance.ID)
	data.Name = types.StringValue(instance.Name)
	data.CPUCores = types.Int64Value(instance.CPUCores)
	data.RAMMB = types.Int64Value(instance.RAMMB)
	data.DiskGB = types.Int64Value(instance.DiskGB)
	data.OSTemplate = types.StringValue(instance.OSTemplate)
	data.IPAddress = types.StringPointerValue(instance.IPAddress)
	data.Status = types.StringValue(instance.Status)
	data.PowerState = types.StringPointerValue(instance.PowerState)
	data.HourlyPrice = types.Float64Value(instance.HourlyPrice)
	data.MonthlyPrice = types.Float64Value(instance.MonthlyPrice)
}

func (r *instanceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data instanceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.client.CreateInstance(ctx, client.InstanceCreate{
		Name: data.Name.ValueString(), CPUCores: data.CPUCores.ValueInt64(), RAMMB: data.RAMMB.ValueInt64(),
		DiskGB: data.DiskGB.ValueInt64(), OSTemplate: data.OSTemplate.ValueString(), VDCID: optionalString(data.VDCID),
		SSHKeyID: optionalString(data.SSHKeyID), UserData: optionalString(data.UserData),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create instance", err.Error())
		return
	}
	setInstanceState(&data, created)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *instanceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data instanceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	instance, err := r.client.GetInstance(ctx, data.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read instance", err.Error())
		return
	}
	setInstanceState(&data, instance)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *instanceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data instanceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	instance, err := r.client.ResizeInstance(
		ctx, data.ID.ValueString(), data.CPUCores.ValueInt64(), data.RAMMB.ValueInt64(), data.DiskGB.ValueInt64(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to resize instance", err.Error())
		return
	}
	setInstanceState(&data, instance)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *instanceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data instanceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteInstance(ctx, data.ID.ValueString(), data.Name.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete instance", err.Error())
	}
}

func (r *instanceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
