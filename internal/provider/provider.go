package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/anatoliacore/terraform-provider-anatoliacore/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = (*anatoliaCoreProvider)(nil)

type anatoliaCoreProvider struct{ version string }

type providerModel struct {
	APIKey  types.String `tfsdk:"api_key"`
	BaseURL types.String `tfsdk:"base_url"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &anatoliaCoreProvider{version: version} }
}

func (p *anatoliaCoreProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "anatoliacore"
	resp.Version = p.version
}

func (p *anatoliaCoreProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"api_key": schema.StringAttribute{
			Optional:            true,
			Sensitive:           true,
			MarkdownDescription: "AnatoliaCore API key. Defaults to `ANATOLIACORE_API_KEY`.",
		},
		"base_url": schema.StringAttribute{
			Optional:            true,
			MarkdownDescription: "Public API base URL. Defaults to `ANATOLIACORE_BASE_URL` or the production endpoint.",
		},
	}}
}

func (p *anatoliaCoreProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiKey := os.Getenv("ANATOLIACORE_API_KEY")
	if !config.APIKey.IsNull() && !config.APIKey.IsUnknown() {
		apiKey = config.APIKey.ValueString()
	}
	baseURL := os.Getenv("ANATOLIACORE_BASE_URL")
	if baseURL == "" {
		baseURL = "https://console.anatoliacore.com/api/public/v1"
	}
	if !config.BaseURL.IsNull() && !config.BaseURL.IsUnknown() {
		baseURL = config.BaseURL.ValueString()
	}
	configured, err := client.New(baseURL, apiKey)
	if err != nil {
		resp.Diagnostics.AddError("Invalid AnatoliaCore provider configuration", err.Error())
		return
	}
	resp.ResourceData = configured
	resp.DataSourceData = configured
}

func (p *anatoliaCoreProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{NewInstanceResource, NewVDCResource, NewVolumeResource, NewFloatingIPResource, NewSecurityGroupResource, NewBackupPolicyResource}
}

func (p *anatoliaCoreProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{NewInstanceTypesDataSource}
}

func configureClient(providerData any, target **client.Client, diagnostics interface {
	AddError(string, string)
}) {
	if providerData == nil {
		return
	}
	configured, ok := providerData.(*client.Client)
	if !ok {
		diagnostics.AddError("Unexpected provider data", fmt.Sprintf("Expected *client.Client, got %T", providerData))
		return
	}
	*target = configured
}
