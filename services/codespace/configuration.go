// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/modules/setting"

	"google.golang.org/protobuf/proto"
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
	if setting.Codespace.ControlPlaneMaxSize <= 0 {
		return errors.New("[codespace] CONTROL_PLANE_MAX_MESSAGE_SIZE must be positive")
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
	if setting.Codespace.OpenTokenExpire <= 0 || setting.Codespace.OpenTokenExpire%time.Second != 0 {
		return errors.New("[codespace] OPEN_TOKEN_EXPIRE must be a positive whole number of seconds")
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
	if setting.Codespace.RuntimeMetadataMaxSize <= 0 {
		return errors.New("[codespace] RUNTIME_METADATA_MAX_SIZE must be positive")
	}
	minControlPlaneSize, minControlPlaneMessage := minimumControlPlaneMaxMessageSize()
	if setting.Codespace.ControlPlaneMaxSize < minControlPlaneSize {
		return fmt.Errorf("[codespace] CONTROL_PLANE_MAX_MESSAGE_SIZE=%d must be at least %d bytes for %s", setting.Codespace.ControlPlaneMaxSize, minControlPlaneSize, minControlPlaneMessage)
	}
	if setting.Codespace.DevContainerConfigMaxSize <= 0 {
		return errors.New("[codespace] DEVCONTAINER_CONFIG_MAX_SIZE must be positive")
	}
	if strings.TrimSpace(setting.Codespace.DevContainerDefaultImage) == "" {
		return errors.New("[codespace] DEVCONTAINER_DEFAULT_IMAGE must not be empty")
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
		setting.Codespace.ControlPlaneMaxSize,
		setting.AppURL
}

func minimumControlPlaneMaxMessageSize() (int64, string) {
	maxString := strings.Repeat("x", 512)
	maxName := strings.Repeat("n", 255)
	maxUUID := "ffffffff-ffff-4fff-8fff-ffffffffffff"
	maxRuntimeSettings := &codespacev1.EffectiveCodespaceRuntimeSettings{
		AutoStopEnabled:       true,
		IdleTimeoutSeconds:    86_400,
		InteractionGeneration: 1<<62 - 1,
	}

	var minSize int64
	var minName string
	track := func(name string, message proto.Message) {
		if size := int64(proto.Size(message)); size > minSize {
			minSize = size
			minName = name
		}
	}
	declareRequest := &codespacev1.DeclareManagerRequest{
		ProtocolVersion: 1,
		Environments:    make([]*codespacev1.EnvironmentTag, 0, 64),
	}
	for range 64 {
		declareRequest.Environments = append(declareRequest.Environments, &codespacev1.EnvironmentTag{
			Tag:         strings.Repeat("t", 64),
			Description: strings.Repeat("d", 255),
		})
	}
	track("DeclareManagerRequest", declareRequest)

	fetchRequest := &codespacev1.FetchOperationsRequest{
		ProtocolVersion:          1,
		StartupCapacityAvailable: 10_000,
		AcceptedOperationTypes:   []codespacev1.AcceptedOperationType{codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_CREATE, codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_RESUME},
		CleanupCapacityAvailable: 256,
		ObservedOperations:       make([]*codespacev1.ObservedOperation, 0, fetchMaxObservedOperations),
		AcceptedCreateTags:       make([]string, 64),
	}
	for i := range fetchRequest.AcceptedCreateTags {
		fetchRequest.AcceptedCreateTags[i] = strings.Repeat("t", 64)
	}
	reportRequest := &codespacev1.ReportInstancesRequest{
		ProtocolVersion:     1,
		InventoryGeneration: 1<<62 - 1,
		Instances:           make([]*codespacev1.RuntimeInstanceRef, 0, fetchMaxObservedOperations),
	}
	reportResponse := &codespacev1.ReportInstancesResponse{
		Results: make([]*codespacev1.RuntimeInstanceResult, 0, fetchMaxObservedOperations),
	}
	for i := range fetchMaxObservedOperations {
		uuid := fmt.Sprintf("%08x-ffff-4fff-8fff-ffffffffffff", i)
		fetchRequest.ObservedOperations = append(fetchRequest.ObservedOperations, &codespacev1.ObservedOperation{
			CodespaceUuid:     uuid,
			OperationRversion: 1<<62 - 1,
		})
		reportRequest.Instances = append(reportRequest.Instances, &codespacev1.RuntimeInstanceRef{
			CodespaceUuid:             uuid,
			RuntimeState:              codespacev1.RuntimeState_RUNTIME_STATE_RUNNING,
			ObservedOperationRversion: 1<<62 - 1,
		})
		reportResponse.Results = append(reportResponse.Results, &codespacev1.RuntimeInstanceResult{
			CodespaceUuid:            uuid,
			RuntimeSettings:          maxRuntimeSettings,
			Action:                   codespacev1.RuntimeReconcileAction_RUNTIME_RECONCILE_ACTION_STOP_LOCAL_RUNTIME,
			CurrentOperationRversion: 1<<62 - 1,
		})
	}
	track("FetchOperationsRequest", fetchRequest)
	track("ReportInstancesRequest", reportRequest)
	track("ReportInstancesResponse", reportResponse)

	createOperation := &codespacev1.OperationPayload{
		OperationRversion:         1<<62 - 1,
		CodespaceUuid:             maxUUID,
		LogOffset:                 1<<62 - 1,
		LeaseValidForMilliseconds: int64(setting.Codespace.OperationLeaseTimeout / time.Millisecond),
		Command: &codespacev1.OperationPayload_Create{Create: &codespacev1.CreateOperationPayload{
			RepoFullName:     maxString,
			RepoCloneHttpUrl: maxString,
			RepoCloneSshUrl:  maxString,
			GitProtocol:      codespacev1.GitProtocol_GIT_PROTOCOL_SSH,
			StartRef:         maxString,
			CommitSha:        strings.Repeat("f", 64),
			EnvironmentTag:   strings.Repeat("t", 64),
			RuntimeSettings:  maxRuntimeSettings,
			Username:         maxName,
			GitUserEmail:     maxString,
			DevContainer: &codespacev1.DevContainerConfiguration{
				RepositoryPath:          strings.Repeat("p", 512),
				RepositoryContentSha256: strings.Repeat("f", 64),
			},
		}},
	}
	fetchResponse := &codespacev1.FetchOperationsResponse{
		Operations:    make([]*codespacev1.OperationPayload, 0, fetchMaxOperations),
		RenewedLeases: make([]*codespacev1.RenewedOperationLease, 0, fetchMaxObservedOperations),
	}
	for range fetchMaxOperations {
		fetchResponse.Operations = append(fetchResponse.Operations, createOperation)
	}
	for i := range fetchMaxObservedOperations {
		fetchResponse.RenewedLeases = append(fetchResponse.RenewedLeases, &codespacev1.RenewedOperationLease{
			CodespaceUuid:             fmt.Sprintf("%08x-ffff-4fff-8fff-ffffffffffff", i),
			OperationRversion:         1<<62 - 1,
			LeaseValidForMilliseconds: int64(setting.Codespace.OperationLeaseTimeout / time.Millisecond),
		})
	}
	if size := int64(proto.Size(fetchResponse)); size > minSize {
		minSize = size
		minName = "FetchOperationsResponse"
	}
	track("UpdateLogRequest", &codespacev1.UpdateLogRequest{
		ProtocolVersion:   1,
		CodespaceUuid:     maxUUID,
		OperationRversion: 1<<62 - 1,
		Offset:            1<<62 - 1,
		Lines: []*codespacev1.LogLine{{
			TimestampUnixNano: 1<<62 - 1,
			Message:           strings.Repeat("l", int(codespaceLogMaxLineSize)),
		}},
	})
	metadataEndpoints := make([]*codespacev1.RuntimeEndpoint, 0, 64)
	metadataEndpoints = append(metadataEndpoints, &codespacev1.RuntimeEndpoint{EndpointId: workspaceEndpointID, Label: workspaceEndpointLabel})
	for i := range 63 {
		metadataEndpoints = append(metadataEndpoints, &codespacev1.RuntimeEndpoint{
			EndpointId: fmt.Sprintf("app-%02d", i),
			Label:      strings.Repeat("m", 64),
			Public:     i%2 == 0,
		})
	}
	track("ReportRuntimeMetadataRequest", &codespacev1.ReportRuntimeMetadataRequest{
		ProtocolVersion: 1,
		CodespaceUuid:   maxUUID,
		Metadata: &codespacev1.RuntimeMetadata{
			Endpoints: metadataEndpoints,
			Boot: &codespacev1.RuntimeBoot{
				OperationRversion: 1<<62 - 1,
				Stage:             codespacev1.RuntimeBootStage_RUNTIME_BOOT_STAGE_READY,
				StartedUnix:       1<<62 - 1,
				LastUpdateUnix:    1<<62 - 1,
			},
			ResourceUsage: &codespacev1.RuntimeResourceUsage{
				Cpu:          &codespacev1.RuntimeCPUUsage{UsedMillicores: 1<<62 - 1, LimitMillicores: 1<<62 - 1},
				Memory:       &codespacev1.RuntimeMemoryUsage{UsedBytes: 1<<62 - 1, LimitBytes: 1<<62 - 1},
				Disk:         &codespacev1.RuntimeDiskUsage{UsedBytes: 1<<62 - 1, LimitBytes: 1<<62 - 1},
				ObservedUnix: 1<<62 - 1,
			},
		},
		MetadataGeneration: 1<<62 - 1,
	})
	return minSize, minName
}
