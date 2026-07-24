// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"math"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
)

// Codespace contains site-wide defaults for Codespace creation.
var Codespace = struct {
	Enabled                    bool
	GitProtocol                string
	GitSSHKnownHosts           []string
	GatewayRequireHTTPS        bool
	ControlPlaneTimeout        time.Duration
	ControlPlaneMaxSize        int64
	ManagerOfflineTimeout      time.Duration
	OperationLeaseTimeout      time.Duration
	OperationMaxDuration       time.Duration
	QueueTimeout               time.Duration
	OpenTokenExpire            time.Duration
	LogMaxSize                 int64
	RuntimeMetadataMaxSize     int64
	CodespaceRepoConfigMaxSize int64
	AutoStopDefaultTimeout     time.Duration
	AutoStopMinTimeout         time.Duration
	AutoStopMaxTimeout         time.Duration
}{
	Enabled:                    true,
	GitProtocol:                "http",
	GitSSHKnownHosts:           nil,
	GatewayRequireHTTPS:        false,
	ControlPlaneTimeout:        30 * time.Second,
	ControlPlaneMaxSize:        8 * 1024 * 1024,
	ManagerOfflineTimeout:      120 * time.Second,
	OperationLeaseTimeout:      300 * time.Second,
	OperationMaxDuration:       2 * time.Hour,
	QueueTimeout:               5 * time.Minute,
	OpenTokenExpire:            60 * time.Second,
	LogMaxSize:                 64 * 1024 * 1024,
	RuntimeMetadataMaxSize:     256 * 1024,
	CodespaceRepoConfigMaxSize: 64 * 1024,
	AutoStopDefaultTimeout:     30 * time.Minute,
	AutoStopMinTimeout:         5 * time.Minute,
	AutoStopMaxTimeout:         168 * time.Hour,
}

func loadCodespaceFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("codespace")
	Codespace.Enabled = sec.Key("ENABLED").MustBool(true)
	Codespace.GitSSHKnownHosts = sec.Key("GIT_SSH_KNOWN_HOSTS").Strings(",")
	Codespace.GatewayRequireHTTPS = sec.Key("GATEWAY_REQUIRE_HTTPS").MustBool(false)
	Codespace.ControlPlaneTimeout = sec.Key("CONTROL_PLANE_TIMEOUT").MustDuration(30 * time.Second)
	Codespace.ControlPlaneMaxSize = mustCodespaceBytes(sec, "CONTROL_PLANE_MAX_MESSAGE_SIZE", "8MiB")
	Codespace.ManagerOfflineTimeout = sec.Key("MANAGER_OFFLINE_TIMEOUT").MustDuration(120 * time.Second)
	Codespace.OperationLeaseTimeout = sec.Key("OPERATION_LEASE_TIMEOUT").MustDuration(300 * time.Second)
	Codespace.OperationMaxDuration = sec.Key("OPERATION_MAX_DURATION").MustDuration(2 * time.Hour)
	Codespace.QueueTimeout = sec.Key("QUEUE_TIMEOUT").MustDuration(5 * time.Minute)
	Codespace.OpenTokenExpire = sec.Key("OPEN_TOKEN_EXPIRE").MustDuration(60 * time.Second)
	Codespace.LogMaxSize = mustCodespaceBytes(sec, "LOG_MAX_SIZE", "64MiB")
	Codespace.RuntimeMetadataMaxSize = mustCodespaceBytes(sec, "RUNTIME_METADATA_MAX_SIZE", "256KiB")
	Codespace.CodespaceRepoConfigMaxSize = mustCodespaceBytes(sec, "CODESPACE_REPO_CONFIG_MAX_SIZE", "64KiB")
	Codespace.AutoStopDefaultTimeout = sec.Key("AUTO_STOP_DEFAULT_TIMEOUT").MustDuration(30 * time.Minute)
	Codespace.AutoStopMinTimeout = sec.Key("AUTO_STOP_MIN_TIMEOUT").MustDuration(5 * time.Minute)
	Codespace.AutoStopMaxTimeout = sec.Key("AUTO_STOP_MAX_TIMEOUT").MustDuration(168 * time.Hour)
	protocol := strings.ToLower(strings.TrimSpace(sec.Key("GIT_PROTOCOL").MustString("http")))
	switch protocol {
	case "http", "ssh":
		Codespace.GitProtocol = protocol
	default:
		Codespace.GitProtocol = "http"
	}
}

func mustCodespaceBytes(section ConfigSection, key, defaultValue string) int64 {
	value := section.Key(key).MustString(defaultValue)
	bytes, err := humanize.ParseBytes(value)
	if err != nil || bytes > math.MaxInt64 {
		return -1
	}
	return int64(bytes)
}
