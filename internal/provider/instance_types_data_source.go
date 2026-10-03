package provider

import (
	"context"

	"github.com/anatoliacore/terraform-provider-anatoliacore/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*instanceTypesDataSource)(nil)
var _ datasource.DataSourceWithConfigure = (*instanceTypesDataSource)(nil)

type instanceTypesDataSource struct{ client *client.Client }
type instanceTypesModel struct {
	ID    types.String `tfsdk:"id"`
	Types types.List   `tfsdk:"types"`
}

var instanceTypeAttributeTypes = map[string]attr.Type{
	"id": types.StringType, "name": types.StringType, "cpu_cores": types.Int64Type,
	"ram_mb": types.Int64Type, "disk_gb": types.Int64Type, "description": types.StringType,
	"price_per_hour": types.Float64Type,
}

func NewInstanceTypesDataSource() datasource.DataSource { return &instanceTypesDataSource{} }
func (d *instanceTypesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_instance_types"
}
func (d *instanceTypesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true},
		"types": schema.ListNestedAttribute{Computed: true, NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"id": schema.StringAttribute{Computed: true}, "name": schema.StringAttribute{Computed: true},
				"cpu_cores": schema.Int64Attribute{Computed: true}, "ram_mb": schema.Int64Attribute{Computed: true},
				"disk_gb": schema.Int64Attribute{Computed: true}, "description": schema.StringAttribute{Computed: true},
				"price_per_hour": schema.Float64Attribute{Computed: true},
			},
		}},
	}}
}
func (d *instanceTypesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	configureClient(req.ProviderData, &d.client, &resp.Diagnostics)
}
func (d *instanceTypesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	items, err := d.client.ListInstanceTypes(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to list instance types", err.Error())
		return
	}
	values := make([]attr.Value, 0, len(items))
	for _, item := range items {
		values = append(values, types.ObjectValueMust(instanceTypeAttributeTypes, map[string]attr.Value{
			"id": types.StringValue(item.ID), "name": types.StringValue(item.Name),
			"cpu_cores": types.Int64Value(item.CPUCores), "ram_mb": types.Int64Value(item.RAMMB),
			"disk_gb": types.Int64Value(item.DiskGB), "description": types.StringPointerValue(item.Description),
			"price_per_hour": types.Float64PointerValue(item.PricePerHour),
		}))
	}
	data := instanceTypesModel{ID: types.StringValue("catalog"), Types: types.ListValueMust(types.ObjectType{AttrTypes: instanceTypeAttributeTypes}, values)}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
