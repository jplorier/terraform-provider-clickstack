// Copyright (c) Lapse Technologies, Inc.
// SPDX-License-Identifier: MPL-2.0

package resources

import (
	"context"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/teamlapse/terraform-provider-clickstack/internal/client"
)

var (
	_ resource.Resource                = &ConnectionResource{}
	_ resource.ResourceWithImportState = &ConnectionResource{}
)

type ConnectionResource struct {
	client *client.Client
}

type connectionResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	Host                 types.String `tfsdk:"host"`
	Username             types.String `tfsdk:"username"`
	Password             types.String `tfsdk:"password"`
	HyperdxSettingPrefix types.String `tfsdk:"hyperdx_setting_prefix"`
	PrometheusEndpoint   types.String `tfsdk:"prometheus_endpoint"`
	CreatedAt            types.String `tfsdk:"created_at"`
	UpdatedAt            types.String `tfsdk:"updated_at"`
}

func NewConnectionResource() resource.Resource {
	return &ConnectionResource{}
}

func (r *ConnectionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connection"
}

func (r *ConnectionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a ClickHouse connection. Only available with self-hosted HyperDX OSS (personal_access_key auth_mode); ClickHouse Cloud manages the connection for you.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Connection ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Display name for the connection.",
			},
			"host": schema.StringAttribute{
				Required:    true,
				Description: "ClickHouse HTTP endpoint URL (e.g. https://clickhouse.example.com:8443).",
			},
			"username": schema.StringAttribute{
				Required:    true,
				Description: "ClickHouse username.",
			},
			"password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "ClickHouse password. Never returned by the API, so it cannot be detected if changed outside Terraform. Leave unset for passwordless connections.",
			},
			"hyperdx_setting_prefix": schema.StringAttribute{
				Optional:    true,
				Description: "Optional prefix for HyperDX-specific ClickHouse settings. Alphanumeric characters and underscores only.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[a-zA-Z0-9_]+$`),
						"must contain only alphanumeric characters and underscores",
					),
				},
			},
			"prometheus_endpoint": schema.StringAttribute{
				Optional:    true,
				Description: "Optional Prometheus-compatible API endpoint. When set, PromQL queries are proxied to this endpoint.",
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Creation timestamp.",
			},
			"updated_at": schema.StringAttribute{
				Computed:    true,
				Description: "Last update timestamp.",
			},
		},
	}
}

func (r *ConnectionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected resource configure type", "Expected *client.Client")
		return
	}
	r.client = c
}

func (r *ConnectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan connectionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateConnection(ctx, expandConnection(plan))
	if err != nil {
		resp.Diagnostics.AddError("Unable to create connection", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, flattenConnection(created, plan.Password))...)
}

func (r *ConnectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state connectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	conn, err := r.client.GetConnection(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read connection", err.Error())
		return
	}

	// The API never returns the password, so preserve the prior value.
	resp.Diagnostics.Append(resp.State.Set(ctx, flattenConnection(conn, state.Password))...)
}

func (r *ConnectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan connectionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state connectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateConnection(ctx, state.ID.ValueString(), expandConnection(plan))
	if err != nil {
		resp.Diagnostics.AddError("Unable to update connection", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, flattenConnection(updated, plan.Password))...)
}

func (r *ConnectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state connectionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteConnection(ctx, state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete connection", err.Error())
	}
}

func (r *ConnectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func expandConnection(plan connectionResourceModel) client.Connection {
	c := client.Connection{
		Name:     plan.Name.ValueString(),
		Host:     plan.Host.ValueString(),
		Username: plan.Username.ValueString(),
		Password: plan.Password.ValueString(),
	}
	if !plan.HyperdxSettingPrefix.IsNull() && !plan.HyperdxSettingPrefix.IsUnknown() {
		v := plan.HyperdxSettingPrefix.ValueString()
		c.HyperdxSettingPrefix = &v
	}
	if !plan.PrometheusEndpoint.IsNull() && !plan.PrometheusEndpoint.IsUnknown() {
		v := plan.PrometheusEndpoint.ValueString()
		c.PrometheusEndpoint = &v
	}
	return c
}

// flattenConnection builds resource state from an API connection. password is
// carried over from the plan/prior state because the API never returns it.
func flattenConnection(c *client.Connection, password types.String) connectionResourceModel {
	return connectionResourceModel{
		ID:                   types.StringValue(c.ID),
		Name:                 types.StringValue(c.Name),
		Host:                 types.StringValue(c.Host),
		Username:             types.StringValue(c.Username),
		Password:             password,
		HyperdxSettingPrefix: stringPtrValue(c.HyperdxSettingPrefix),
		PrometheusEndpoint:   stringPtrValue(c.PrometheusEndpoint),
		CreatedAt:            stringPtrValue(c.CreatedAt),
		UpdatedAt:            stringPtrValue(c.UpdatedAt),
	}
}

func stringPtrValue(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}
	return types.StringValue(*s)
}
