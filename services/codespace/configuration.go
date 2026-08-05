// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/modules/setting"
)

const (
	// ManagerServiceMaxMessageSize is the fixed request and response limit for the Manager control plane.
	ManagerServiceMaxMessageSize = 32 * 1024 * 1024
	openTokenExpire              = 60 * time.Second
	runtimeMetadataMaxSize       = 256 * 1024
	devContainerConfigMaxSize    = 64 * 1024
)

// Init validates Codespace runtime entrypoint configuration during web startup.
func Init(ctx context.Context) error {
	if !setting.Codespace.Enabled {
		return nil
	}
	if err := ValidateCodespaceConfig(); err != nil {
		return err
	}
	if err := ValidateGitTransports(); err != nil {
		return err
	}
	return WarnManagerGatewayAddressConflicts(ctx)
}

// ValidateCodespaceConfig verifies cross-field Codespace settings before runtime entrypoints start.
func ValidateCodespaceConfig() error {
	if setting.Codespace.ControlPlaneTimeout <= 0 {
		return errors.New("[codespace] CONTROL_PLANE_TIMEOUT must be positive")
	}
	if setting.Codespace.ManagerOfflineTimeout <= 0 {
		return errors.New("[codespace] MANAGER_OFFLINE_TIMEOUT must be positive")
	}
	if setting.Codespace.OperationLeaseTimeout <= 0 {
		return errors.New("[codespace] OPERATION_LEASE_TIMEOUT must be positive")
	}
	if setting.Codespace.OperationLeaseTimeout%time.Millisecond != 0 {
		return errors.New("[codespace] OPERATION_LEASE_TIMEOUT must be a positive whole number of milliseconds")
	}
	if setting.Codespace.OperationMaxDuration <= setting.Codespace.OperationLeaseTimeout {
		return errors.New("[codespace] OPERATION_MAX_DURATION must be greater than OPERATION_LEASE_TIMEOUT")
	}
	if setting.Codespace.OperationMaxDuration%time.Second != 0 {
		return errors.New("[codespace] OPERATION_MAX_DURATION must be a positive whole number of seconds")
	}
	if setting.Codespace.ManagerOfflineTimeout%time.Second != 0 {
		return errors.New("[codespace] MANAGER_OFFLINE_TIMEOUT must be a positive whole number of seconds")
	}
	if setting.Codespace.QueueTimeout <= 0 {
		return errors.New("[codespace] QUEUE_TIMEOUT must be positive")
	}
	if setting.Codespace.QueueTimeout%time.Second != 0 {
		return errors.New("[codespace] QUEUE_TIMEOUT must be a positive whole number of seconds")
	}
	if setting.Codespace.ControlPlaneTimeout > setting.Codespace.ManagerOfflineTimeout/4 {
		return errors.New("[codespace] CONTROL_PLANE_TIMEOUT must be no greater than MANAGER_OFFLINE_TIMEOUT/4")
	}
	if setting.Codespace.AutoStopMinTimeout <= 0 ||
		setting.Codespace.AutoStopDefaultTimeout <= 0 ||
		setting.Codespace.AutoStopMaxTimeout <= 0 {
		return errors.New("[codespace] AUTO_STOP timeouts must be positive")
	}
	if setting.Codespace.AutoStopMinTimeout%time.Second != 0 ||
		setting.Codespace.AutoStopDefaultTimeout%time.Second != 0 ||
		setting.Codespace.AutoStopMaxTimeout%time.Second != 0 {
		return errors.New("[codespace] AUTO_STOP timeouts must be whole seconds")
	}
	if setting.Codespace.AutoStopMinTimeout > setting.Codespace.AutoStopDefaultTimeout ||
		setting.Codespace.AutoStopDefaultTimeout > setting.Codespace.AutoStopMaxTimeout {
		return errors.New("[codespace] AUTO_STOP_MIN_TIMEOUT <= AUTO_STOP_DEFAULT_TIMEOUT <= AUTO_STOP_MAX_TIMEOUT is required")
	}
	if setting.Codespace.LogMaxSize <= 0 {
		return errors.New("[codespace] LOG_MAX_SIZE must be positive")
	}
	if LogReadMaxBytes >= setting.Codespace.LogMaxSize ||
		codespaceLogInternalSummaryReserve >= setting.Codespace.LogMaxSize {
		return errors.New("[codespace] LOG_MAX_SIZE must be greater than the internal log page and state summary reserve sizes")
	}
	return nil
}

type gitTransportCapabilities struct {
	HTTPEnabled  bool
	SSHEnabled   bool
	HTTPDisabled string
	SSHDisabled  string
}

// ValidateGitTransports verifies that new Codespaces have a usable Git clone entrypoint.
func ValidateGitTransports() error {
	protocol, err := createGitProtocol()
	if err != nil {
		return err
	}
	_, err = resolveGitTransportCapabilities(protocol)
	return err
}

func resolveGitTransportCapabilities(protocol string) (*gitTransportCapabilities, error) {
	capabilities := &gitTransportCapabilities{
		HTTPEnabled: true,
	}
	var unavailable []string
	if setting.Repository.DisableHTTPGit {
		capabilities.HTTPEnabled = false
		capabilities.HTTPDisabled = "[repository] DISABLE_HTTP_GIT=true"
		unavailable = append(unavailable, "http: "+capabilities.HTTPDisabled)
	}
	if disabled := gitSSHCloneDisabledReason(); disabled != "" {
		capabilities.SSHDisabled = disabled
		unavailable = append(unavailable, "ssh: "+disabled)
	} else {
		capabilities.SSHEnabled = true
	}

	if !capabilities.HTTPEnabled && !capabilities.SSHEnabled {
		return nil, fmt.Errorf("codespace git transport unavailable: %s", strings.Join(unavailable, "; "))
	}
	switch protocol {
	case codespace_model.GitProtocolHTTP:
		if !capabilities.HTTPEnabled {
			return nil, fmt.Errorf("codespace git transport unavailable: http: %s", capabilities.HTTPDisabled)
		}
	case codespace_model.GitProtocolSSH:
		if !capabilities.SSHEnabled {
			return nil, fmt.Errorf("codespace git transport unavailable: ssh: %s", capabilities.SSHDisabled)
		}
	default:
		return nil, fmt.Errorf("invalid codespace git protocol %q", protocol)
	}
	return capabilities, nil
}

// ManagerServiceTimings returns server-selected ManagerService control values.
func ManagerServiceTimings() (heartbeatMillis, metadataRefreshMillis, maxMessageBytes int64, giteaWebURL string) {
	return int64((setting.Codespace.ManagerOfflineTimeout / 4) / time.Millisecond),
		int64((setting.Codespace.ManagerOfflineTimeout / 2) / time.Millisecond),
		ManagerServiceMaxMessageSize,
		setting.AppURL
}
