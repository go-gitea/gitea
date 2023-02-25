// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// Telemetry settings
var Telemetry = struct {
	Enabled      bool
	ServiceName  string
	EndpointType string
	Endpoint     string
	UseTLS       bool
	PrettyPrint  bool
	Timestamps   bool
}{
	Enabled:      false,
	ServiceName:  "gitea",
	EndpointType: "stdout",
	Endpoint:     "",
	UseTLS:       true,
	PrettyPrint:  true,
	Timestamps:   true,
}

func loadTelemetryFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "telemetry", &Telemetry)
}
