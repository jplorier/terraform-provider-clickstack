// Copyright (c) Lapse Technologies, Inc.
// SPDX-License-Identifier: MPL-2.0

package client

import "encoding/json"

// APIResponse is the ClickHouse Cloud API envelope.
type APIResponse[T any] struct {
	Status    int    `json:"status"`
	RequestID string `json:"requestId"`
	Result    T      `json:"result"`
	Error     string `json:"error,omitempty"`
}

// Dashboard represents a ClickStack dashboard.
type Dashboard struct {
	ID                 string             `json:"id,omitempty"`
	Name               string             `json:"name"`
	Tiles              []Tile             `json:"tiles"`
	Tags               []string           `json:"tags,omitempty"`
	Filters            []Filter           `json:"filters,omitempty"`
	SavedQuery         *string            `json:"savedQuery,omitempty"`
	SavedQueryLanguage *string            `json:"savedQueryLanguage,omitempty"`
	SavedFilterValues  []SavedFilterValue `json:"savedFilterValues,omitempty"`
}

// Tile represents a dashboard tile.
type Tile struct {
	ID     string          `json:"id,omitempty"`
	Name   string          `json:"name"`
	X      int             `json:"x"`
	Y      int             `json:"y"`
	W      int             `json:"w"`
	H      int             `json:"h"`
	Config json.RawMessage `json:"config,omitempty"`
	Series json.RawMessage `json:"series,omitempty"`
}

// Filter represents a dashboard-level filter.
type Filter struct {
	ID               string  `json:"id,omitempty"`
	Type             string  `json:"type"`
	Name             string  `json:"name"`
	Expression       string  `json:"expression"`
	SourceID         string  `json:"sourceId"`
	SourceMetricType *string `json:"sourceMetricType,omitempty"`
}

// SavedFilterValue represents a persisted filter value on a dashboard.
type SavedFilterValue struct {
	Condition string  `json:"condition"`
	Type      *string `json:"type,omitempty"`
}

// Alert represents a ClickStack alert.
type Alert struct {
	ID                    string         `json:"id,omitempty"`
	Name                  *string        `json:"name,omitempty"`
	Message               *string        `json:"message,omitempty"`
	Source                string         `json:"source"`
	Threshold             float64        `json:"threshold"`
	ThresholdType         string         `json:"thresholdType"`
	Interval              string         `json:"interval"`
	Channel               AlertChannel   `json:"channel"`
	DashboardID           *string        `json:"dashboardId,omitempty"`
	TileID                *string        `json:"tileId,omitempty"`
	SavedSearchID         *string        `json:"savedSearchId,omitempty"`
	GroupBy               *string        `json:"groupBy,omitempty"`
	ScheduleOffsetMinutes *int           `json:"scheduleOffsetMinutes,omitempty"`
	ScheduleStartAt       *string        `json:"scheduleStartAt,omitempty"`
	State                 string         `json:"state,omitempty"`
	TeamID                string         `json:"teamId,omitempty"`
	Silenced              *AlertSilenced `json:"silenced,omitempty"`
	CreatedAt             *string        `json:"createdAt,omitempty"`
	UpdatedAt             *string        `json:"updatedAt,omitempty"`
}

// AlertChannel represents the notification channel for an alert.
type AlertChannel struct {
	Type string `json:"type"`
	// Webhook fields
	WebhookID      *string `json:"webhookId,omitempty"`
	WebhookService *string `json:"webhookService,omitempty"`
	SlackChannelID *string `json:"slackChannelId,omitempty"`
	Severity       *string `json:"severity,omitempty"`
	// Email fields
	EmailRecipients []string `json:"emailRecipients,omitempty"`
}

// AlertSilenced represents silencing metadata on an alert.
type AlertSilenced struct {
	By    string  `json:"by"`
	At    string  `json:"at"`
	Until *string `json:"until,omitempty"`
}

// SavedSearch represents a ClickStack saved search.
type SavedSearch struct {
	ID      string           `json:"id,omitempty"`
	Name    string           `json:"name"`
	Query   string           `json:"query"`
	Source  string           `json:"source"`
	Tags    []string         `json:"tags,omitempty"`
	Columns []string         `json:"columns,omitempty"`
	Sort    *SavedSearchSort `json:"sort,omitempty"`
}

// SavedSearchSort represents the sort order for a saved search.
type SavedSearchSort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

// Source represents a ClickStack data source (read-only).
//
// Connection/From/TimestampValueExpression are populated by the self-hosted
// (HyperDX OSS) /api/v2/sources endpoint, which returns the full source
// document. The managed Cloud endpoint may omit them; they're tagged
// omitempty so the same struct round-trips both shapes.
type Source struct {
	ID                       string     `json:"id"`
	Name                     string     `json:"name"`
	Kind                     string     `json:"kind"`
	Connection               string     `json:"connection,omitempty"`
	From                     *SourceRef `json:"from,omitempty"`
	TimestampValueExpression string     `json:"timestampValueExpression,omitempty"`
}

// SourceRef points at the ClickHouse database+table a source reads from.
type SourceRef struct {
	DatabaseName string `json:"databaseName"`
	TableName    string `json:"tableName"`
}

// Connection represents a ClickHouse connection. This is a self-hosted
// HyperDX OSS concept (managed Cloud provisions the connection for you), so
// it is only available in personal_access_key auth mode. The API never
// returns the password.
type Connection struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	// nil means "unset": omitted on create, cleared on update.
	HyperdxSettingPrefix *string `json:"hyperdxSettingPrefix,omitempty"`
	PrometheusEndpoint   *string `json:"prometheusEndpoint,omitempty"`
	CreatedAt            *string `json:"createdAt,omitempty"`
	UpdatedAt            *string `json:"updatedAt,omitempty"`
}

// Webhook represents a ClickStack webhook (read-only).
type Webhook struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Service     string  `json:"service"`
	URL         string  `json:"url"`
	Description *string `json:"description,omitempty"`
	CreatedAt   *string `json:"createdAt,omitempty"`
	UpdatedAt   *string `json:"updatedAt,omitempty"`
}
