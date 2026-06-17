# ---------------------------------------------------------------------------
# Connections: ClickHouse connections for self-hosted HyperDX OSS.
#
# Only available with auth_mode = "personal_access_key". ClickHouse Cloud
# manages the connection for you, so this resource is not used there.
# ---------------------------------------------------------------------------

resource "clickstack_connection" "primary" {
  name     = "Primary ClickHouse"
  host     = "https://clickhouse.example.com:8443"
  username = "default"
  password = var.clickhouse_password
}

# Optional: route PromQL queries to a Prometheus-compatible endpoint and
# namespace HyperDX-specific ClickHouse settings with a prefix.
resource "clickstack_connection" "with_prometheus" {
  name                   = "ClickHouse + Prometheus"
  host                   = "https://clickhouse.example.com:8443"
  username               = "hyperdx"
  password               = var.clickhouse_password
  hyperdx_setting_prefix = "hyperdx_"
  prometheus_endpoint    = "http://prometheus:9090"
}

variable "clickhouse_password" {
  type      = string
  sensitive = true
}
