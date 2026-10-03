package provider

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/anatoliacore/terraform-provider-anatoliacore/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*securityGroupResource)(nil)
var _ resource.ResourceWithConfigure = (*securityGroupResource)(nil)
var _ resource.ResourceWithImportState = (*securityGroupResource)(nil)

type securityGroupResource struct{ client *client.Client }
type securityGroupModel struct {
	ID              types.String `tfsdk:"id"`
	VDCID           types.String `tfsdk:"vdc_id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	RulesJSON       types.String `tfsdk:"rules_json"`
	Status          types.String `tfsdk:"status"`
	DesiredRevision types.Int64  `tfsdk:"desired_revision"`
	AppliedRevision types.Int64  `tfsdk:"applied_revision"`
}

func NewSecurityGroupResource() resource.Resource { return &securityGroupResource{} }
func (r *securityGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_security_group"
}
func (r *securityGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}}, "vdc_id": schema.StringAttribute{Required: true, PlanModifiers: replace}, "name": schema.StringAttribute{Required: true, PlanModifiers: replace}, "description": schema.StringAttribute{Optional: true, PlanModifiers: replace},
		"rules_json": schema.StringAttribute{Required: true, PlanModifiers: replace, MarkdownDescription: "JSON array of edge firewall rules. Each rule contains direction, protocol, optional port_start/port_end, source_cidr, and action=allow."}, "status": schema.StringAttribute{Computed: true}, "desired_revision": schema.Int64Attribute{Computed: true}, "applied_revision": schema.Int64Attribute{Computed: true},
	}}
}
func (r *securityGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	configureClient(req.ProviderData, &r.client, &resp.Diagnostics)
}
func securityGroupBody(data *securityGroupModel) (map[string]any, error) {
	var rules []map[string]any
	if err := json.Unmarshal([]byte(data.RulesJSON.ValueString()), &rules); err != nil {
		return nil, errors.New("rules_json must be a JSON array")
	}
	return map[string]any{"vdc_id": data.VDCID.ValueString(), "name": data.Name.ValueString(), "description": optionalString(data.Description), "rules": rules}, nil
}
func setSecurityGroupState(data *securityGroupModel, value *client.SecurityGroup) {
	data.ID = types.StringValue(value.ID)
	data.VDCID = types.StringPointerValue(value.VDCID)
	data.Name = types.StringValue(value.Name)
	data.Description = types.StringPointerValue(value.Description)
	// Preserve the caller's canonical JSON while refreshing an existing
	// resource. Provider responses may add server-owned rule identifiers; if
	// those are written into configuration state Terraform reports a perpetual
	// replacement diff. Imports have no configured value, so only then derive it
	// from the remote representation.
	if data.RulesJSON.IsNull() || data.RulesJSON.IsUnknown() {
		rules, _ := json.Marshal(value.Rules)
		data.RulesJSON = types.StringValue(string(rules))
	}
	data.Status = types.StringValue(value.Status)
	data.DesiredRevision = types.Int64Value(value.DesiredRevision)
	data.AppliedRevision = types.Int64Value(value.AppliedRevision)
}
func (r *securityGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data securityGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, err := securityGroupBody(&data)
	if err != nil {
		resp.Diagnostics.AddError("Invalid security group", err.Error())
		return
	}
	value, err := r.client.CreateSecurityGroup(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create security group", err.Error())
		return
	}
	setSecurityGroupState(&data, value)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *securityGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data securityGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	value, err := r.client.GetSecurityGroup(ctx, data.ID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read security group", err.Error())
		return
	}
	setSecurityGroupState(&data, value)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
func (r *securityGroupResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
}
func (r *securityGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data securityGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteSecurityGroup(ctx, data.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete security group", err.Error())
	}
}
func (r *securityGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
