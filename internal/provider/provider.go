// Copyright (c) Lapse Technologies, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/teamlapse/terraform-provider-clickstack/internal/client"
	"github.com/teamlapse/terraform-provider-clickstack/internal/datasources"
	"github.com/teamlapse/terraform-provider-clickstack/internal/resources"
)

var _ provider.Provider = &ClickStackProvider{}

// ClickStackProvider implements the Terraform provider. It targets either
// ClickHouse Cloud's managed ClickStack (HTTP Basic auth, Cloud API URL
// shape) or a self-hosted HyperDX OSS deployment (Bearer auth, /api/v2/...
// URL shape), depending on auth_mode.
type ClickStackProvider struct {
	version string
}

type clickStackProviderModel struct {
	BaseURL  types.String `tfsdk:"base_url"`
	AuthMode types.String `tfsdk:"auth_mode"`

	// Cloud
	OrganizationID types.String `tfsdk:"organization_id"`
	ServiceID      types.String `tfsdk:"service_id"`
	APIKeyID       types.String `tfsdk:"api_key_id"`
	APIKeySecret   types.String `tfsdk:"api_key_secret"`

	// OSS
	PersonalAccessKey types.String `tfsdk:"personal_access_key"`
}

// New returns a new provider factory function.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &ClickStackProvider{version: version}
	}
}

func (p *ClickStackProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "clickstack"
	resp.Version = p.version
}

func (p *ClickStackProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for managing dashboards and alerts on ClickStack — either ClickHouse Cloud's managed offering or self-hosted HyperDX OSS.",
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				Description: "Base URL of the ClickStack API. Defaults to https://api.clickhouse.cloud for cloud_api_key mode; required for personal_access_key mode (e.g. http://clickstack-api.clickstack.svc.cluster.local:8000). Can also be set via CLICKSTACK_BASE_URL env var.",
				Optional:    true,
			},
			"auth_mode": schema.StringAttribute{
				Description: "Authentication and API surface to use. One of `cloud_api_key` (ClickHouse Cloud managed ClickStack; default) or `personal_access_key` (self-hosted HyperDX OSS, /api/v2/ endpoints). Can also be set via CLICKSTACK_AUTH_MODE env var.",
				Optional:    true,
			},
			"organization_id": schema.StringAttribute{
				Description: "ClickHouse Cloud organization ID (cloud_api_key mode only). Can also be set via CLICKSTACK_ORGANIZATION_ID env var.",
				Optional:    true,
			},
			"service_id": schema.StringAttribute{
				Description: "ClickHouse Cloud service ID for the ClickStack instance (cloud_api_key mode only). Can also be set via CLICKSTACK_SERVICE_ID env var.",
				Optional:    true,
			},
			"api_key_id": schema.StringAttribute{
				Description: "ClickHouse Cloud API key ID (cloud_api_key mode only). Can also be set via CLICKSTACK_API_KEY_ID env var.",
				Optional:    true,
				Sensitive:   true,
			},
			"api_key_secret": schema.StringAttribute{
				Description: "ClickHouse Cloud API key secret (cloud_api_key mode only). Can also be set via CLICKSTACK_API_KEY_SECRET env var.",
				Optional:    true,
				Sensitive:   true,
			},
			"personal_access_key": schema.StringAttribute{
				Description: "HyperDX OSS Personal API Access Key (personal_access_key mode only). Mint one in the HyperDX UI under Team Settings → API Keys. Can also be set via CLICKSTACK_PERSONAL_ACCESS_KEY env var.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

// firstNonEmpty returns the first non-empty string from a TF-config value
// (if set) and a list of env var names, in order.
func firstNonEmpty(cfg types.String, envVars ...string) string {
	if !cfg.IsNull() && !cfg.IsUnknown() && cfg.ValueString() != "" {
		return cfg.ValueString()
	}
	for _, env := range envVars {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	return ""
}

func (p *ClickStackProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config clickStackProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	authModeStr := firstNonEmpty(config.AuthMode, "CLICKSTACK_AUTH_MODE")
	if authModeStr == "" {
		authModeStr = string(client.AuthModeCloudAPIKey)
	}

	var authMode client.AuthMode
	switch authModeStr {
	case string(client.AuthModeCloudAPIKey):
		authMode = client.AuthModeCloudAPIKey
	case string(client.AuthModePersonalAccessKey):
		authMode = client.AuthModePersonalAccessKey
	default:
		resp.Diagnostics.AddError(
			"Invalid auth_mode",
			"auth_mode must be one of \"cloud_api_key\" or \"personal_access_key\", got: "+authModeStr,
		)
		return
	}

	baseURL := firstNonEmpty(config.BaseURL, "CLICKSTACK_BASE_URL")
	if baseURL == "" {
		if authMode == client.AuthModeCloudAPIKey {
			baseURL = "https://api.clickhouse.cloud"
		} else {
			resp.Diagnostics.AddError(
				"Missing base_url",
				"base_url is required in personal_access_key mode. Set the provider attribute or CLICKSTACK_BASE_URL (e.g. http://clickstack-api.clickstack.svc.cluster.local:8000).",
			)
			return
		}
	}

	cfg := client.Config{
		BaseURL:  baseURL,
		AuthMode: authMode,
	}

	if authMode == client.AuthModeCloudAPIKey {
		cfg.OrganizationID = firstNonEmpty(config.OrganizationID, "CLICKSTACK_ORGANIZATION_ID")
		cfg.ServiceID = firstNonEmpty(config.ServiceID, "CLICKSTACK_SERVICE_ID")
		cfg.APIKeyID = firstNonEmpty(config.APIKeyID, "CLICKSTACK_API_KEY_ID")
		cfg.APIKeySecret = firstNonEmpty(config.APIKeySecret, "CLICKSTACK_API_KEY_SECRET")

		missing := []string{}
		if cfg.OrganizationID == "" {
			missing = append(missing, "organization_id")
		}
		if cfg.ServiceID == "" {
			missing = append(missing, "service_id")
		}
		if cfg.APIKeyID == "" {
			missing = append(missing, "api_key_id")
		}
		if cfg.APIKeySecret == "" {
			missing = append(missing, "api_key_secret")
		}
		for _, m := range missing {
			resp.Diagnostics.AddError(
				"Missing "+m,
				m+" is required in cloud_api_key mode. Set the provider attribute or the matching CLICKSTACK_* env var.",
			)
		}
		if resp.Diagnostics.HasError() {
			return
		}
	} else {
		cfg.PersonalAccessKey = firstNonEmpty(config.PersonalAccessKey, "CLICKSTACK_PERSONAL_ACCESS_KEY")
		if cfg.PersonalAccessKey == "" {
			resp.Diagnostics.AddError(
				"Missing personal_access_key",
				"personal_access_key is required in personal_access_key mode. Set the provider attribute or CLICKSTACK_PERSONAL_ACCESS_KEY.",
			)
			return
		}
	}

	c := client.NewClient(cfg)
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *ClickStackProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewDashboardResource,
		resources.NewAlertResource,
		resources.NewSavedSearchResource,
	}
}

func (p *ClickStackProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		datasources.NewSourcesDataSource,
		datasources.NewWebhooksDataSource,
	}
}
