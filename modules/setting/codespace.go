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
	Enabled                bool
	GitProtocol            string
	GitSSHKnownHosts       []string
	GatewayRequireHTTPS    bool
	ControlPlaneTimeout    time.Duration
	ManagerOfflineTimeout  time.Duration
	OperationLeaseTimeout  time.Duration
	OperationMaxDuration   time.Duration
	QueueTimeout           time.Duration
	LogMaxSize             int64
	AutoStopDefaultTimeout time.Duration
	AutoStopMinTimeout     time.Duration
	AutoStopMaxTimeout     time.Duration
}{
	Enabled:                true,
	GitProtocol:            "http",
	GitSSHKnownHosts:       nil,
	GatewayRequireHTTPS:    false,
	ControlPlaneTimeout:    30 * time.Second,
	ManagerOfflineTimeout:  120 * time.Second,
	OperationLeaseTimeout:  300 * time.Second,
	OperationMaxDuration:   2 * time.Hour,
	QueueTimeout:           5 * time.Minute,
	LogMaxSize:             64 * 1024 * 1024,
	AutoStopDefaultTimeout: 30 * time.Minute,
	AutoStopMinTimeout:     5 * time.Minute,
	AutoStopMaxTimeout:     168 * time.Hour,
}

func loadCodespaceFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("codespace")
	Codespace.Enabled = sec.Key("ENABLED").MustBool(true)
	Codespace.GitSSHKnownHosts = sec.Key("GIT_SSH_KNOWN_HOSTS").Strings(",")
	Codespace.GatewayRequireHTTPS = sec.Key("GATEWAY_REQUIRE_HTTPS").MustBool(false)
	Codespace.ControlPlaneTimeout = sec.Key("CONTROL_PLANE_TIMEOUT").MustDuration(30 * time.Second)
	Codespace.ManagerOfflineTimeout = sec.Key("MANAGER_OFFLINE_TIMEOUT").MustDuration(120 * time.Second)
	Codespace.OperationLeaseTimeout = sec.Key("OPERATION_LEASE_TIMEOUT").MustDuration(300 * time.Second)
	Codespace.OperationMaxDuration = sec.Key("OPERATION_MAX_DURATION").MustDuration(2 * time.Hour)
	Codespace.QueueTimeout = sec.Key("QUEUE_TIMEOUT").MustDuration(5 * time.Minute)
	Codespace.LogMaxSize = mustCodespaceBytes(sec, "LOG_MAX_SIZE", "64MiB")
	Codespace.AutoStopDefaultTimeout = sec.Key("AUTO_STOP_DEFAULT_TIMEOUT").MustDuration(30 * time.Minute)
	Codespace.AutoStopMinTimeout = sec.Key("AUTO_STOP_MIN_TIMEOUT").MustDuration(5 * time.Minute)
	Codespace.AutoStopMaxTimeout = sec.Key("AUTO_STOP_MAX_TIMEOUT").MustDuration(168 * time.Hour)
	Codespace.GitProtocol = strings.ToLower(strings.TrimSpace(sec.Key("GIT_PROTOCOL").MustString("http")))
}

func mustCodespaceBytes(section ConfigSection, key, defaultValue string) int64 {
	value := section.Key(key).MustString(defaultValue)
	bytes, err := humanize.ParseBytes(value)
	if err != nil || bytes > math.MaxInt64 {
		return -1
	}
	return int64(bytes)
}
