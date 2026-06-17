# Terraform Provider for ClickStack

Terraform provider for managing dashboards, alerts, and saved searches on [ClickStack](https://clickhouse.com/docs/en/observability) (HyperDX on ClickHouse Cloud).

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.25 (to build the provider)

## Installation

### Terraform Registry

```hcl
terraform {
  required_providers {
    clickstack = {
      source  = "registry.terraform.io/teamlapse/clickstack"
      version = "~> 0.1"
    }
  }
}
```

### Local Development

```bash
go install .
```

The binary will be installed to `$GOPATH/bin/terraform-provider-clickstack`.

## Authentication

The provider supports two modes, selected by `auth_mode` (or `CLICKSTACK_AUTH_MODE`):

- `cloud_api_key` (default) — ClickHouse Cloud's managed ClickStack, using an API key pair over HTTP Basic auth.
- `personal_access_key` — self-hosted HyperDX OSS, using a Personal API Access Key (Bearer auth) against the `/api/v2/` endpoints.

### ClickHouse Cloud (`cloud_api_key`)

| Provider Argument   | Environment Variable           | Description                        |
|---------------------|--------------------------------|------------------------------------|
| `organization_id`   | `CLICKSTACK_ORGANIZATION_ID`   | ClickHouse Cloud organization ID   |
| `service_id`        | `CLICKSTACK_SERVICE_ID`        | ClickStack service ID              |
| `api_key_id`        | `CLICKSTACK_API_KEY_ID`        | API key ID                         |
| `api_key_secret`    | `CLICKSTACK_API_KEY_SECRET`    | API key secret                     |
| `base_url`          | `CLICKSTACK_BASE_URL`          | API base URL (default: `https://api.clickhouse.cloud`) |

```hcl
provider "clickstack" {
  organization_id = var.clickstack_organization_id
  service_id      = var.clickstack_service_id
  api_key_id      = var.clickstack_api_key_id
  api_key_secret  = var.clickstack_api_key_secret
}
```

### Self-hosted HyperDX OSS (`personal_access_key`)

Mint a Personal API Access Key in the HyperDX UI under **Team Settings → API Keys**.

| Provider Argument     | Environment Variable             | Description                          |
|-----------------------|----------------------------------|--------------------------------------|
| `personal_access_key` | `CLICKSTACK_PERSONAL_ACCESS_KEY` | HyperDX Personal API Access Key      |
| `base_url`            | `CLICKSTACK_BASE_URL`            | API base URL (required, e.g. `http://localhost:8000`) |

```hcl
provider "clickstack" {
  auth_mode           = "personal_access_key"
  base_url            = "http://localhost:8000"
  personal_access_key = var.clickstack_personal_access_key
}
```

> Some resources are mode-specific: `clickstack_saved_search` is Cloud-only, and `clickstack_connection` is OSS-only.

## Resources

- `clickstack_dashboard` — Manage observability dashboards with tiles, filters, and queries.
- `clickstack_alert` — Manage threshold-based alerts from dashboard tiles or saved searches.
- `clickstack_saved_search` — Manage reusable search queries (Cloud only).
- `clickstack_connection` — Manage ClickHouse connections (self-hosted HyperDX OSS only).

## Data Sources

- `clickstack_sources` — List available data sources (log, trace, metric, session).
- `clickstack_webhooks` — List configured webhooks (Slack, PagerDuty, etc.).

## Quick Start

```hcl
# Create a saved search for error logs
resource "clickstack_saved_search" "errors" {
  name   = "API Errors"
  query  = "level:error service:api-gateway"
  source = "log"

  sort {
    field = "timestamp"
    order = "desc"
  }
}

# Alert when errors exceed threshold
resource "clickstack_alert" "high_errors" {
  name            = "High Error Rate"
  source          = "saved_search"
  saved_search_id = clickstack_saved_search.errors.id
  threshold       = 50
  threshold_type  = "above"
  interval        = "5m"

  channel {
    type             = "email"
    email_recipients = ["oncall@example.com"]
  }
}
```

See the [`examples/`](examples/) directory for more complete usage.

## Development

### Building

```bash
go build -o terraform-provider-clickstack .
```

### Testing

Acceptance tests require a live ClickStack instance. Set the environment variables above, then:

```bash
TF_ACC=1 go test ./... -v
```

### Generating Documentation

```bash
go generate ./...
```

This runs `tfplugindocs` to regenerate the `docs/` directory from schema descriptions and example files.

## License

This project is licensed under the [Mozilla Public License 2.0](LICENSE).
