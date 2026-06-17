// Copyright (c) Lapse Technologies, Inc.
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AuthMode selects the API surface (Cloud vs self-hosted/OSS) and the
// authentication style. The two modes are not interchangeable: each targets
// a different URL path and credential scheme.
type AuthMode string

const (
	// AuthModeCloudAPIKey targets ClickHouse Cloud's managed ClickStack:
	// HTTP Basic auth (apiKeyID:apiKeySecret) against
	// {base_url}/v1/organizations/{org}/services/{svc}/clickstack/{resource}.
	AuthModeCloudAPIKey AuthMode = "cloud_api_key"

	// AuthModePersonalAccessKey targets self-hosted HyperDX OSS:
	// Bearer auth (a Personal API Access Key minted in Team Settings)
	// against {base_url}/api/v2/{resource}.
	AuthModePersonalAccessKey AuthMode = "personal_access_key"
)

// Config carries all credentials and endpoint info needed to construct a
// Client. Only the fields relevant to AuthMode need to be populated.
type Config struct {
	BaseURL  string
	AuthMode AuthMode

	// Cloud (AuthModeCloudAPIKey)
	OrganizationID string
	ServiceID      string
	APIKeyID       string
	APIKeySecret   string

	// OSS (AuthModePersonalAccessKey)
	PersonalAccessKey string
}

// Client is an HTTP client for the ClickStack API, supporting both
// ClickHouse Cloud (managed) and self-hosted HyperDX OSS.
type Client struct {
	cfg        Config
	httpClient *http.Client
}

// NewClient creates a new ClickStack API client from a Config.
func NewClient(cfg Config) *Client {
	if cfg.AuthMode == "" {
		cfg.AuthMode = AuthModeCloudAPIKey
	}
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) isOSS() bool {
	return c.cfg.AuthMode == AuthModePersonalAccessKey
}

func (c *Client) basePath() string {
	if c.isOSS() {
		return c.cfg.BaseURL + "/api/v2"
	}
	return fmt.Sprintf("%s/v1/organizations/%s/services/%s/clickstack",
		c.cfg.BaseURL, c.cfg.OrganizationID, c.cfg.ServiceID)
}

func (c *Client) doRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.basePath()+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	if c.isOSS() {
		req.Header.Set("Authorization", "Bearer "+c.cfg.PersonalAccessKey)
	} else {
		req.SetBasicAuth(c.cfg.APIKeyID, c.cfg.APIKeySecret)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr APIResponse[json.RawMessage]
		msg := string(respBody)
		requestID := ""
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error != "" {
			msg = apiErr.Error
			requestID = apiErr.RequestID
		}
		if resp.StatusCode == http.StatusNotFound {
			return nil, &NotFoundError{Message: msg, RequestID: requestID}
		}
		return nil, fmt.Errorf("API error (status %d, requestId %s): %s", resp.StatusCode, requestID, msg)
	}

	return respBody, nil
}

// unwrapResult extracts the payload from the API response envelope. Cloud
// returns `{status, requestId, result: T}`; OSS returns `{data: T}`. We
// probe for whichever key is present so call sites don't need to branch.
func unwrapResult[T any](data []byte) (T, error) {
	var zero T

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return zero, fmt.Errorf("unmarshaling response: %w", err)
	}

	for _, key := range []string{"result", "data"} {
		raw, ok := probe[key]
		if !ok || len(raw) == 0 || string(raw) == "null" {
			continue
		}
		var v T
		if err := json.Unmarshal(raw, &v); err != nil {
			return zero, fmt.Errorf("unmarshaling %q: %w", key, err)
		}
		return v, nil
	}

	return zero, fmt.Errorf("response had neither 'result' nor 'data' payload: %s", string(data))
}

// errOSSUnsupported is returned by client methods whose endpoint doesn't
// exist in the OSS external v2 API surface.
func errOSSUnsupported(op string) error {
	return fmt.Errorf("%s is not supported by self-hosted HyperDX OSS (no external API endpoint); manage in the UI", op)
}

// errCloudUnsupported is returned by client methods whose endpoint only
// exists in self-hosted HyperDX OSS. ClickHouse Cloud manages the underlying
// ClickHouse connection for you, so it has no connections API.
func errCloudUnsupported(op string) error {
	return fmt.Errorf("%s is only supported by self-hosted HyperDX OSS (personal_access_key auth_mode); ClickHouse Cloud manages the connection for you", op)
}

// NotFoundError is returned when the API returns a 404.
type NotFoundError struct {
	Message   string
	RequestID string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("not found (requestId %s): %s", e.RequestID, e.Message)
}

// IsNotFound returns true if the error is a 404 from the API.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*NotFoundError)
	return ok
}

// --- Dashboards ---

func (c *Client) ListDashboards(ctx context.Context) ([]Dashboard, error) {
	data, err := c.doRequest(ctx, http.MethodGet, "/dashboards", nil)
	if err != nil {
		return nil, err
	}
	return unwrapResult[[]Dashboard](data)
}

func (c *Client) GetDashboard(ctx context.Context, id string) (*Dashboard, error) {
	data, err := c.doRequest(ctx, http.MethodGet, "/dashboards/"+id, nil)
	if err != nil {
		return nil, err
	}
	result, err := unwrapResult[Dashboard](data)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) CreateDashboard(ctx context.Context, dashboard Dashboard) (*Dashboard, error) {
	data, err := c.doRequest(ctx, http.MethodPost, "/dashboards", dashboard)
	if err != nil {
		return nil, err
	}
	result, err := unwrapResult[Dashboard](data)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UpdateDashboard(ctx context.Context, id string, dashboard Dashboard) (*Dashboard, error) {
	// Both Cloud and OSS external v2 update dashboards via PUT /dashboards/:id.
	data, err := c.doRequest(ctx, http.MethodPut, "/dashboards/"+id, dashboard)
	if err != nil {
		return nil, err
	}
	result, err := unwrapResult[Dashboard](data)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteDashboard(ctx context.Context, id string) error {
	_, err := c.doRequest(ctx, http.MethodDelete, "/dashboards/"+id, nil)
	return err
}

// --- Alerts ---

func (c *Client) ListAlerts(ctx context.Context) ([]Alert, error) {
	data, err := c.doRequest(ctx, http.MethodGet, "/alerts", nil)
	if err != nil {
		return nil, err
	}
	return unwrapResult[[]Alert](data)
}

func (c *Client) GetAlert(ctx context.Context, id string) (*Alert, error) {
	data, err := c.doRequest(ctx, http.MethodGet, "/alerts/"+id, nil)
	if err != nil {
		return nil, err
	}
	result, err := unwrapResult[Alert](data)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) CreateAlert(ctx context.Context, alert Alert) (*Alert, error) {
	data, err := c.doRequest(ctx, http.MethodPost, "/alerts", alert)
	if err != nil {
		return nil, err
	}
	result, err := unwrapResult[Alert](data)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UpdateAlert(ctx context.Context, id string, alert Alert) (*Alert, error) {
	data, err := c.doRequest(ctx, http.MethodPut, "/alerts/"+id, alert)
	if err != nil {
		return nil, err
	}
	result, err := unwrapResult[Alert](data)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteAlert(ctx context.Context, id string) error {
	_, err := c.doRequest(ctx, http.MethodDelete, "/alerts/"+id, nil)
	return err
}

// --- Saved Searches ---
//
// Not exposed by the self-hosted external v2 API; Cloud-only.

func (c *Client) ListSavedSearches(ctx context.Context) ([]SavedSearch, error) {
	if c.isOSS() {
		return nil, errOSSUnsupported("ListSavedSearches")
	}
	data, err := c.doRequest(ctx, http.MethodGet, "/savedSearches", nil)
	if err != nil {
		return nil, err
	}
	return unwrapResult[[]SavedSearch](data)
}

func (c *Client) GetSavedSearch(ctx context.Context, id string) (*SavedSearch, error) {
	if c.isOSS() {
		return nil, errOSSUnsupported("GetSavedSearch")
	}
	data, err := c.doRequest(ctx, http.MethodGet, "/savedSearches/"+id, nil)
	if err != nil {
		return nil, err
	}
	result, err := unwrapResult[SavedSearch](data)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) CreateSavedSearch(ctx context.Context, search SavedSearch) (*SavedSearch, error) {
	if c.isOSS() {
		return nil, errOSSUnsupported("CreateSavedSearch")
	}
	data, err := c.doRequest(ctx, http.MethodPost, "/savedSearches", search)
	if err != nil {
		return nil, err
	}
	result, err := unwrapResult[SavedSearch](data)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UpdateSavedSearch(ctx context.Context, id string, search SavedSearch) (*SavedSearch, error) {
	if c.isOSS() {
		return nil, errOSSUnsupported("UpdateSavedSearch")
	}
	data, err := c.doRequest(ctx, http.MethodPut, "/savedSearches/"+id, search)
	if err != nil {
		return nil, err
	}
	result, err := unwrapResult[SavedSearch](data)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteSavedSearch(ctx context.Context, id string) error {
	if c.isOSS() {
		return errOSSUnsupported("DeleteSavedSearch")
	}
	_, err := c.doRequest(ctx, http.MethodDelete, "/savedSearches/"+id, nil)
	return err
}

// --- Sources (read-only) ---

func (c *Client) ListSources(ctx context.Context) ([]Source, error) {
	data, err := c.doRequest(ctx, http.MethodGet, "/sources", nil)
	if err != nil {
		return nil, err
	}
	return unwrapResult[[]Source](data)
}

// --- Webhooks (read-only) ---

func (c *Client) ListWebhooks(ctx context.Context) ([]Webhook, error) {
	data, err := c.doRequest(ctx, http.MethodGet, "/webhooks", nil)
	if err != nil {
		return nil, err
	}
	return unwrapResult[[]Webhook](data)
}

// --- Connections ---
//
// Self-hosted HyperDX OSS only; ClickHouse Cloud manages the connection.

func (c *Client) ListConnections(ctx context.Context) ([]Connection, error) {
	if !c.isOSS() {
		return nil, errCloudUnsupported("ListConnections")
	}
	data, err := c.doRequest(ctx, http.MethodGet, "/connections", nil)
	if err != nil {
		return nil, err
	}
	return unwrapResult[[]Connection](data)
}

func (c *Client) GetConnection(ctx context.Context, id string) (*Connection, error) {
	if !c.isOSS() {
		return nil, errCloudUnsupported("GetConnection")
	}
	data, err := c.doRequest(ctx, http.MethodGet, "/connections/"+id, nil)
	if err != nil {
		return nil, err
	}
	result, err := unwrapResult[Connection](data)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) CreateConnection(ctx context.Context, conn Connection) (*Connection, error) {
	if !c.isOSS() {
		return nil, errCloudUnsupported("CreateConnection")
	}
	data, err := c.doRequest(ctx, http.MethodPost, "/connections", conn)
	if err != nil {
		return nil, err
	}
	result, err := unwrapResult[Connection](data)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) UpdateConnection(ctx context.Context, id string, conn Connection) (*Connection, error) {
	if !c.isOSS() {
		return nil, errCloudUnsupported("UpdateConnection")
	}
	// Send the optional fields explicitly so that clearing them in config
	// clears them server-side: '' clears the setting prefix and null clears
	// the Prometheus endpoint. Omitting either would instead preserve the
	// existing value, leaving Terraform in perpetual drift. An empty password
	// is still omitted so the existing one is kept.
	prefix := ""
	if conn.HyperdxSettingPrefix != nil {
		prefix = *conn.HyperdxSettingPrefix
	}
	body := struct {
		Name                 string  `json:"name"`
		Host                 string  `json:"host"`
		Username             string  `json:"username"`
		Password             string  `json:"password,omitempty"`
		HyperdxSettingPrefix string  `json:"hyperdxSettingPrefix"`
		PrometheusEndpoint   *string `json:"prometheusEndpoint"`
	}{
		Name:                 conn.Name,
		Host:                 conn.Host,
		Username:             conn.Username,
		Password:             conn.Password,
		HyperdxSettingPrefix: prefix,
		PrometheusEndpoint:   conn.PrometheusEndpoint,
	}
	data, err := c.doRequest(ctx, http.MethodPut, "/connections/"+id, body)
	if err != nil {
		return nil, err
	}
	result, err := unwrapResult[Connection](data)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteConnection(ctx context.Context, id string) error {
	if !c.isOSS() {
		return errCloudUnsupported("DeleteConnection")
	}
	_, err := c.doRequest(ctx, http.MethodDelete, "/connections/"+id, nil)
	return err
}
