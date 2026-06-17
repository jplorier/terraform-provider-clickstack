// Copyright (c) Lapse Technologies, Inc.
// SPDX-License-Identifier: MPL-2.0

package datasources

import (
	"context"

	"github.com/teamlapse/terraform-provider-clickstack/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &SourcesDataSource{}

type SourcesDataSource struct {
	client *client.Client
}

type sourcesDataSourceModel struct {
	Sources []sourceModel `tfsdk:"sources"`
}

type sourceModel struct {
	ID                       types.String `tfsdk:"id"`
	Name                     types.String `tfsdk:"name"`
	Kind                     types.String `tfsdk:"kind"`
	Connection               types.String `tfsdk:"connection"`
	FromDatabase             types.String `tfsdk:"from_database"`
	FromTable                types.String `tfsdk:"from_table"`
	TimestampValueExpression types.String `tfsdk:"timestamp_value_expression"`
}

func NewSourcesDataSource() datasource.DataSource {
	return &SourcesDataSource{}
}

func (d *SourcesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sources"
}

func (d *SourcesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists all ClickStack data sources (log, trace, metric, session).",
		Attributes: map[string]schema.Attribute{
			"sources": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of data sources.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "Source ID.",
						},
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "Source name.",
						},
						"kind": schema.StringAttribute{
							Computed:    true,
							Description: "Source kind (log, trace, metric, session).",
						},
						"connection": schema.StringAttribute{
							Computed:    true,
							Description: "ClickHouse connection ID this source reads from. Populated by self-hosted HyperDX OSS; may be empty on managed Cloud. Useful as the `connection` field in dashboard tile configs.",
						},
						"from_database": schema.StringAttribute{
							Computed:    true,
							Description: "ClickHouse database the source queries (e.g. `default` or `otel`). Populated by self-hosted HyperDX OSS.",
						},
						"from_table": schema.StringAttribute{
							Computed:    true,
							Description: "ClickHouse table the source queries (e.g. `otel_logs`). Empty for metric sources, which fan out across gauge/sum/histogram tables.",
						},
						"timestamp_value_expression": schema.StringAttribute{
							Computed:    true,
							Description: "SQL expression for the source's timestamp column. Populated by self-hosted HyperDX OSS.",
						},
					},
				},
			},
		},
	}
}

func (d *SourcesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected data source configure type", "Expected *client.Client")
		return
	}
	d.client = c
}

func (d *SourcesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	sources, err := d.client.ListSources(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to list sources", err.Error())
		return
	}

	state := sourcesDataSourceModel{
		Sources: []sourceModel{},
	}
	for _, s := range sources {
		fromDB, fromTable := "", ""
		if s.From != nil {
			fromDB = s.From.DatabaseName
			fromTable = s.From.TableName
		}
		state.Sources = append(state.Sources, sourceModel{
			ID:                       types.StringValue(s.ID),
			Name:                     types.StringValue(s.Name),
			Kind:                     types.StringValue(s.Kind),
			Connection:               types.StringValue(s.Connection),
			FromDatabase:             types.StringValue(fromDB),
			FromTable:                types.StringValue(fromTable),
			TimestampValueExpression: types.StringValue(s.TimestampValueExpression),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
