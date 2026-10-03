package provider

import (
	"context"
	"errors"

	"github.com/anatoliacore/terraform-provider-anatoliacore/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*backupPolicyResource)(nil)
var _ resource.ResourceWithConfigure = (*backupPolicyResource)(nil)
var _ resource.ResourceWithImportState = (*backupPolicyResource)(nil)

type backupPolicyResource struct{ client *client.Client }
type backupPolicyModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	InstanceIDs     types.List   `tfsdk:"instance_ids"`
	BackupTier      types.String `tfsdk:"backup_tier"`
	ScheduleType    types.String `tfsdk:"schedule_type"`
	ScheduleHours   types.Int64  `tfsdk:"schedule_hours"`
	StartTime       types.String `tfsdk:"start_time"`
	Timezone        types.String `tfsdk:"timezone"`
	WeeklyDay       types.String `tfsdk:"weekly_day"`
	RetentionDays   types.Int64  `tfsdk:"retention_days"`
	ScheduleEnabled types.Bool   `tfsdk:"schedule_enabled"`
	Status          types.String `tfsdk:"status"`
}

func NewBackupPolicyResource() resource.Resource { return &backupPolicyResource{} }
func (r *backupPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_backup_policy"
}
func (r *backupPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}, "name": schema.StringAttribute{Required: true}, "description": schema.StringAttribute{Optional: true},
		"instance_ids": schema.ListAttribute{Required: true, ElementType: types.StringType}, "backup_tier": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("standard")},
		"schedule_type": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("daily")}, "schedule_hours": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(24), Validators: []validator.Int64{int64validator.Between(6, 168)}},
		"start_time": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("22:00")}, "timezone": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("Europe/Istanbul")}, "weekly_day": schema.StringAttribute{Optional: true},
		"retention_days": schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(7), Validators: []validator.Int64{int64validator.Between(1, 365)}}, "schedule_enabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)}, "status": schema.StringAttribute{Computed: true},
	}}
}
func (r *backupPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	configureClient(req.ProviderData, &r.client, &resp.Diagnostics)
}
func backupBody(ctx context.Context, data *backupPolicyModel) (map[string]any, error) {
	var ids []string
	if d := data.InstanceIDs.ElementsAs(ctx, &ids, false); d.HasError() {
		return nil, errors.New(d.Errors()[0].Summary())
	}
	return map[string]any{"name": data.Name.ValueString(), "description": optionalString(data.Description), "instance_ids": ids, "backup_tier": data.BackupTier.ValueString(), "schedule_type": data.ScheduleType.ValueString(), "schedule_hours": data.ScheduleHours.ValueInt64(), "start_time": data.StartTime.ValueString(), "timezone": data.Timezone.ValueString(), "weekly_day": optionalString(data.WeeklyDay), "retention_days": data.RetentionDays.ValueInt64(), "schedule_enabled": data.ScheduleEnabled.ValueBool()}, nil
}
func setBackupState(ctx context.Context, data *backupPolicyModel, value *client.BackupPolicy) {
	data.ID = types.StringValue(value.ID)
	data.Name = types.StringValue(value.Name)
	data.Description = types.StringPointerValue(value.Description)
	ids := make([]string, len(value.Targets))
	for i, target := range value.Targets {
		ids[i] = target.InstanceID
	}
	data.InstanceIDs, _ = types.ListValueFrom(ctx, types.StringType, ids)
	data.BackupTier = types.StringValue(value.BackupTier)
	data.ScheduleType = types.StringValue(value.ScheduleType)
	data.ScheduleHours = types.Int64Value(value.ScheduleHours)
	data.StartTime = types.StringValue(value.StartTime)
	data.Timezone = types.StringValue(value.Timezone)
	data.WeeklyDay = types.StringPointerValue(value.WeeklyDay)
	data.RetentionDays = types.Int64Value(value.RetentionDays)
	data.ScheduleEnabled = types.BoolValue(value.ScheduleEnabled)
	data.Status = types.StringValue(value.Status)
}
func (r *backupPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data backupPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, err := backupBody(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Invalid backup policy", err.Error())
		return
	}
	value, err := r.client.CreateBackupPolicy(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create backup policy", err.Error())
		return
	}
	setBackupState(ctx, &data, value)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *backupPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data backupPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	value, err := r.client.GetBackupPolicy(ctx, data.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read backup policy", err.Error())
		return
	}
	setBackupState(ctx, &data, value)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *backupPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data backupPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, err := backupBody(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Invalid backup policy", err.Error())
		return
	}
	value, err := r.client.UpdateBackupPolicy(ctx, data.ID.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update backup policy", err.Error())
		return
	}
	setBackupState(ctx, &data, value)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *backupPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data backupPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteBackupPolicy(ctx, data.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete backup policy", err.Error())
	}
}
func (r *backupPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
