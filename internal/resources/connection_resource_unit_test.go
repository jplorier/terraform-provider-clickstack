// Copyright (c) Lapse Technologies, Inc.
// SPDX-License-Identifier: MPL-2.0

package resources_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/teamlapse/terraform-provider-clickstack/internal/testmock"
)

func TestUnitConnectionResource_basic(t *testing.T) {
	mock := testmock.NewServer(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: mock.OSSProviderConfig() + `
resource "clickstack_connection" "test" {
  name                   = "Prod ClickHouse"
  host                   = "https://clickhouse.example.com:8443"
  username               = "default"
  password               = "secret"
  hyperdx_setting_prefix = "hyperdx_"
  prometheus_endpoint    = "http://prometheus:9090"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("clickstack_connection.test", "id"),
					resource.TestCheckResourceAttr("clickstack_connection.test", "name", "Prod ClickHouse"),
					resource.TestCheckResourceAttr("clickstack_connection.test", "host", "https://clickhouse.example.com:8443"),
					resource.TestCheckResourceAttr("clickstack_connection.test", "username", "default"),
					resource.TestCheckResourceAttr("clickstack_connection.test", "password", "secret"),
					resource.TestCheckResourceAttr("clickstack_connection.test", "hyperdx_setting_prefix", "hyperdx_"),
					resource.TestCheckResourceAttr("clickstack_connection.test", "prometheus_endpoint", "http://prometheus:9090"),
				),
			},
			// Update: clear the optional fields and rename.
			{
				Config: mock.OSSProviderConfig() + `
resource "clickstack_connection" "test" {
  name     = "Prod ClickHouse v2"
  host     = "https://clickhouse.example.com:8443"
  username = "default"
  password = "secret"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("clickstack_connection.test", "name", "Prod ClickHouse v2"),
					resource.TestCheckNoResourceAttr("clickstack_connection.test", "hyperdx_setting_prefix"),
					resource.TestCheckNoResourceAttr("clickstack_connection.test", "prometheus_endpoint"),
				),
			},
		},
	})
}
