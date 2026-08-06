// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package manager

import (
	"context"
	"errors"
	"net/http"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	"gitea.dev/codespace-proto-go/codespace/v1/codespacev1connect"
	codespace_service "gitea.dev/services/codespace"

	"connectrpc.com/connect"
)

// NewManagerServiceHandler returns the Codespace ManagerService Connect handler.
func NewManagerServiceHandler() (string, http.Handler) {
	return codespacev1connect.NewManagerServiceHandler(
		&Service{},
		connect.WithCompressMinBytes(1024),
		connect.WithReadMaxBytes(codespace_service.ManagerServiceMaxMessageSize),
		connect.WithSendMaxBytes(codespace_service.ManagerServiceMaxMessageSize),
		withManager,
	)
}

var _ codespacev1connect.ManagerServiceHandler = (*Service)(nil)

// Service implements the Codespace ManagerService RPC entrypoint.
type Service struct {
	codespacev1connect.UnimplementedManagerServiceHandler
}

// DeclareManager stores the authenticated Manager's current declaration.
func (s *Service) DeclareManager(
	ctx context.Context,
	req *connect.Request[codespacev1.DeclareManagerRequest],
) (*connect.Response[codespacev1.DeclareManagerResponse], error) {
	manager := GetManager(ctx)
	if err := codespace_service.DeclareManager(ctx, manager, codespace_service.DeclareManagerOptions{
		GatewayURL:                         req.Msg.GetGatewayUrl(),
		GatewaySSHAddr:                     req.Msg.GetGatewaySshAddr(),
		Environments:                       req.Msg.GetEnvironments(),
		Version:                            req.Msg.GetVersion(),
		Name:                               req.Msg.GetName(),
		RuntimeState:                       req.Msg.GetManagerRuntimeState(),
		GatewaySSHHostKeyAlgorithm:         req.Msg.GetGatewaySshHostKeyAlgorithm(),
		GatewaySSHHostKeyFingerprintSHA256: req.Msg.GetGatewaySshHostKeyFingerprintSha256(),
		GatewaySSHHostKeyUpdatedUnix:       req.Msg.GetGatewaySshHostKeyUpdatedUnix(),
	}); err != nil {
		return nil, serviceFailureError(err, "invalid_declaration", nil)
	}
	heartbeatMillis, metadataRefreshMillis, maxMessageBytes, giteaWebURL := codespace_service.ManagerServiceTimings()
	return connect.NewResponse(&codespacev1.DeclareManagerResponse{
		HeartbeatIntervalMilliseconds:              heartbeatMillis,
		RuntimeMetadataRefreshIntervalMilliseconds: metadataRefreshMillis,
		ControlPlaneMaxMessageSizeBytes:            maxMessageBytes,
		GiteaWebUrl:                                giteaWebURL,
	}), nil
}

// FetchOperations returns operation payloads and renewed leases for the authenticated Manager.
func (s *Service) FetchOperations(
	ctx context.Context,
	req *connect.Request[codespacev1.FetchOperationsRequest],
) (*connect.Response[codespacev1.FetchOperationsResponse], error) {
	manager := GetManager(ctx)
	result, err := codespace_service.FetchOperations(ctx, manager, codespace_service.FetchOperationsOptions{
		StartupCapacityAvailable: req.Msg.GetStartupCapacityAvailable(),
		AcceptedOperationTypes:   req.Msg.GetAcceptedOperationTypes(),
		AcceptedCreateTags:       req.Msg.GetAcceptedCreateTags(),
		ObservedOperations:       req.Msg.GetObservedOperations(),
		CleanupCapacityAvailable: req.Msg.GetCleanupCapacityAvailable(),
	})
	if err != nil {
		return nil, serviceFailureError(err, "invalid_argument", []serviceErrorCase{
			{target: codespace_service.ErrFetchStateHistoryConflict, code: connect.CodeFailedPrecondition, category: "state_history_conflict"},
			{target: codespace_service.ErrFetchManagerUnavailable, code: connect.CodeUnavailable, category: "manager_offline"},
		})
	}
	return connect.NewResponse(result), nil
}

// BindRuntimeIdentity stores the Manager-allocated runtime UUID for an active create operation.
func (s *Service) BindRuntimeIdentity(
	ctx context.Context,
	req *connect.Request[codespacev1.BindRuntimeIdentityRequest],
) (*connect.Response[codespacev1.BindRuntimeIdentityResponse], error) {
	manager := GetManager(ctx)
	runtimeUUID, err := codespace_service.BindRuntimeIdentity(ctx, manager, codespace_service.BindRuntimeIdentityOptions{
		CodespaceID:       req.Msg.GetCodespaceId(),
		OperationRVersion: req.Msg.GetOperationRversion(),
		RuntimeUUID:       req.Msg.GetRuntimeUuid(),
	})
	if err != nil {
		return nil, serviceFailureError(err, "invalid_argument", []serviceErrorCase{
			{target: codespace_service.ErrBindRuntimeIdentityNotFound, code: connect.CodeNotFound, category: "codespace_not_found"},
			{target: codespace_service.ErrBindRuntimeIdentityStateConflict, code: connect.CodeFailedPrecondition, category: "state_conflict"},
			{target: codespace_service.ErrBindRuntimeIdentityConflict, code: connect.CodeFailedPrecondition, category: "runtime_uuid_conflict"},
		})
	}
	return connect.NewResponse(&codespacev1.BindRuntimeIdentityResponse{RuntimeUuid: runtimeUUID}), nil
}

// ReportInstances accepts a complete Runtime inventory snapshot.
func (s *Service) ReportInstances(
	ctx context.Context,
	req *connect.Request[codespacev1.ReportInstancesRequest],
) (*connect.Response[codespacev1.ReportInstancesResponse], error) {
	manager := GetManager(ctx)
	result, err := codespace_service.ReportInstances(ctx, manager, codespace_service.ReportInstancesOptions{
		InventoryGeneration: req.Msg.GetInventoryGeneration(),
		Instances:           req.Msg.GetInstances(),
	})
	if err != nil {
		var staleGeneration *codespace_service.StaleGenerationError
		switch {
		case errors.As(err, &staleGeneration):
			return nil, failureErrorWithStaleGeneration(connect.CodeFailedPrecondition, "stale_generation", staleGeneration.CurrentGeneration, err)
		case errors.Is(err, codespace_service.ErrReportInstancesStateHistoryConflict):
			return nil, failureError(connect.CodeFailedPrecondition, "state_history_conflict", err)
		case errors.Is(err, codespace_service.ErrReportInstancesManagerUnavailable):
			return nil, failureError(connect.CodeUnavailable, "manager_offline", err)
		default:
			return nil, failureError(connect.CodeInvalidArgument, "invalid_argument", err)
		}
	}
	return connect.NewResponse(result), nil
}

// FinalizeOperation reports the authenticated Manager's final operation result.
func (s *Service) FinalizeOperation(
	ctx context.Context,
	req *connect.Request[codespacev1.FinalizeOperationRequest],
) (*connect.Response[codespacev1.FinalizeOperationResponse], error) {
	manager := GetManager(ctx)
	response, err := codespace_service.FinalizeOperation(ctx, manager, codespace_service.FinalizeOperationOptions{
		CodespaceUUID:     req.Msg.GetRuntimeUuid(),
		OperationRVersion: req.Msg.GetOperationRversion(),
		OperationType:     req.Msg.GetOperationType(),
		FinalStatus:       req.Msg.GetStatus(),
	})
	if err != nil {
		return nil, serviceFailureError(err, "invalid_argument", []serviceErrorCase{
			{target: codespace_service.ErrFinalizeGiteaTokenRequired, code: connect.CodeFailedPrecondition, category: "gitea_token_required"},
			{target: codespace_service.ErrFinalizeMetadataRequired, code: connect.CodeFailedPrecondition, category: "metadata_required"},
		})
	}
	return connect.NewResponse(response), nil
}

// UpdateLog appends sanitized log lines for the authenticated Manager's active operation.
func (s *Service) UpdateLog(
	ctx context.Context,
	req *connect.Request[codespacev1.UpdateLogRequest],
) (*connect.Response[codespacev1.UpdateLogResponse], error) {
	manager := GetManager(ctx)
	result, err := codespace_service.UpdateLog(ctx, manager, codespace_service.UpdateLogOptions{
		CodespaceUUID:     req.Msg.GetRuntimeUuid(),
		OperationRVersion: req.Msg.GetOperationRversion(),
		Offset:            req.Msg.GetOffset(),
		Lines:             req.Msg.GetLines(),
	})
	if err != nil {
		var offsetErr *codespace_service.LogOffsetError
		switch {
		case errors.As(err, &offsetErr) && errors.Is(err, codespace_service.ErrUpdateLogOffsetConflict):
			return nil, failureErrorWithLogOffset(connect.CodeAborted, "offset_conflict", offsetErr.CurrentOffset, err)
		case errors.As(err, &offsetErr) && errors.Is(err, codespace_service.ErrUpdateLogOffsetGap):
			return nil, failureErrorWithLogOffset(connect.CodeAborted, "offset_gap", offsetErr.CurrentOffset, err)
		case errors.Is(err, codespace_service.ErrUpdateLogNotFound):
			return nil, failureError(connect.CodeNotFound, "codespace_not_found", err)
		case errors.Is(err, codespace_service.ErrUpdateLogStaleOperation):
			return nil, failureError(connect.CodeFailedPrecondition, "stale_operation", err)
		case errors.Is(err, codespace_service.ErrUpdateLogSizeExceeded):
			return nil, failureError(connect.CodeResourceExhausted, "log_size_exceeded", err)
		default:
			return nil, failureError(connect.CodeInvalidArgument, "invalid_argument", err)
		}
	}
	return connect.NewResponse(result), nil
}

// ReportRuntimeMetadata stores the authenticated Manager's current Runtime Metadata snapshot.
func (s *Service) ReportRuntimeMetadata(
	ctx context.Context,
	req *connect.Request[codespacev1.ReportRuntimeMetadataRequest],
) (*connect.Response[codespacev1.ReportRuntimeMetadataResponse], error) {
	manager := GetManager(ctx)
	err := codespace_service.ReportRuntimeMetadata(ctx, manager, codespace_service.ReportRuntimeMetadataOptions{
		CodespaceUUID:      req.Msg.GetRuntimeUuid(),
		Metadata:           req.Msg.GetMetadata(),
		MetadataGeneration: req.Msg.GetMetadataGeneration(),
	})
	if err != nil {
		return nil, reportRuntimeMetadataError(err)
	}
	return connect.NewResponse(&codespacev1.ReportRuntimeMetadataResponse{}), nil
}

// ReportRuntimeTransition stores the authenticated Manager's local stopped or failed fact.
func (s *Service) ReportRuntimeTransition(
	ctx context.Context,
	req *connect.Request[codespacev1.ReportRuntimeTransitionRequest],
) (*connect.Response[codespacev1.ReportRuntimeTransitionResponse], error) {
	manager := GetManager(ctx)
	err := codespace_service.ReportRuntimeTransition(ctx, manager, codespace_service.ReportRuntimeTransitionOptions{
		CodespaceUUID:             req.Msg.GetRuntimeUuid(),
		RuntimeGeneration:         req.Msg.GetRuntimeGeneration(),
		ObservedOperationRVersion: req.Msg.GetObservedOperationRversion(),
		RuntimeState:              req.Msg.GetRuntimeState(),
	})
	if err != nil {
		return nil, reportRuntimeTransitionError(err)
	}
	return connect.NewResponse(&codespacev1.ReportRuntimeTransitionResponse{}), nil
}

func reportRuntimeMetadataError(err error) error {
	return reportRuntimeError(err, []serviceErrorCase{
		{target: codespace_service.ErrRuntimeMetadataGenerationConflict, code: connect.CodeFailedPrecondition, category: "generation_conflict"},
		{target: codespace_service.ErrRuntimeMetadataVersionExhausted, code: connect.CodeFailedPrecondition, category: "version_exhausted"},
		{target: codespace_service.ErrRuntimeMetadataManagerMismatch, code: connect.CodeFailedPrecondition, category: "manager_mismatch"},
		{target: codespace_service.ErrRuntimeMetadataStaleOperation, code: connect.CodeFailedPrecondition, category: "stale_operation"},
		{target: codespace_service.ErrRuntimeMetadataManagerOffline, code: connect.CodeUnavailable, category: "manager_offline"},
		{target: codespace_service.ErrRuntimeMetadataStateUnavailable, code: connect.CodeFailedPrecondition, category: "state_unavailable"},
	})
}

func reportRuntimeTransitionError(err error) error {
	return reportRuntimeError(err, []serviceErrorCase{
		{target: codespace_service.ErrRuntimeTransitionNotFound, code: connect.CodeNotFound, category: "codespace_not_found"},
		{target: codespace_service.ErrRuntimeTransitionManagerMismatch, code: connect.CodeFailedPrecondition, category: "manager_mismatch"},
		{target: codespace_service.ErrRuntimeTransitionCurrentOperationConflict, code: connect.CodeAborted, category: "current_operation_conflict"},
		{target: codespace_service.ErrRuntimeTransitionManagerOffline, code: connect.CodeUnavailable, category: "manager_offline"},
		{target: codespace_service.ErrRuntimeTransitionStaleOperation, code: connect.CodeFailedPrecondition, category: "stale_operation"},
		{target: codespace_service.ErrRuntimeTransitionGenerationConflict, code: connect.CodeFailedPrecondition, category: "generation_conflict"},
	})
}

type serviceErrorCase struct {
	target   error
	code     connect.Code
	category string
}

func reportRuntimeError(err error, cases []serviceErrorCase) error {
	var staleGeneration *codespace_service.StaleGenerationError
	if errors.As(err, &staleGeneration) {
		return failureErrorWithStaleGeneration(connect.CodeFailedPrecondition, "stale_generation", staleGeneration.CurrentGeneration, err)
	}
	return serviceFailureError(err, "invalid_argument", cases)
}

func serviceFailureError(err error, fallbackCategory string, cases []serviceErrorCase) error {
	for _, errCase := range cases {
		if errors.Is(err, errCase.target) {
			return failureError(errCase.code, errCase.category, err)
		}
	}
	return failureError(connect.CodeInvalidArgument, fallbackCategory, err)
}

// RequestRuntimeAccess returns the authenticated Manager's current runtime access material.
func (s *Service) RequestRuntimeAccess(
	ctx context.Context,
	req *connect.Request[codespacev1.RequestRuntimeAccessRequest],
) (*connect.Response[codespacev1.RequestRuntimeAccessResponse], error) {
	manager := GetManager(ctx)
	result, err := codespace_service.RequestRuntimeAccess(ctx, manager, codespace_service.RequestRuntimeAccessOptions{
		CodespaceUUID:     req.Msg.GetRuntimeUuid(),
		OperationRVersion: req.Msg.GetOperationRversion(),
		GitSSHPublicKey:   req.Msg.GetGitSshKey().GetPublicKey(),
	})
	if err != nil {
		return nil, serviceFailureError(err, "invalid_argument", []serviceErrorCase{
			{target: codespace_service.ErrRequestRuntimeAccessNotFound, code: connect.CodeNotFound, category: "codespace_not_found"},
			{target: codespace_service.ErrRequestRuntimeAccessManagerMismatch, code: connect.CodeFailedPrecondition, category: "manager_mismatch"},
			{target: codespace_service.ErrRequestRuntimeAccessStateUnavailable, code: connect.CodeFailedPrecondition, category: "state_unavailable"},
			{target: codespace_service.ErrRequestRuntimeAccessManagerOffline, code: connect.CodeUnavailable, category: "manager_offline"},
			{target: codespace_service.ErrRequestRuntimeAccessUserNotFound, code: connect.CodeFailedPrecondition, category: "user_not_found"},
			{target: codespace_service.ErrRuntimeGitSSHKeyLoginRestricted, code: connect.CodeFailedPrecondition, category: "login_restricted"},
			{target: codespace_service.ErrRuntimeGitSSHKeyInvalidPublicKey, code: connect.CodeInvalidArgument, category: "invalid_public_key"},
			{target: codespace_service.ErrRuntimeGitSSHKeyConflict, code: connect.CodeFailedPrecondition, category: "key_conflict"},
			{target: codespace_service.ErrRuntimeGitSSHKeyIntegrity, code: connect.CodeInternal, category: "internal_error"},
		})
	}
	secrets := make([]*codespacev1.RuntimeSecretEnvironmentVariable, 0, len(result.Secrets))
	for _, secret := range result.Secrets {
		secrets = append(secrets, &codespacev1.RuntimeSecretEnvironmentVariable{Name: secret.Name, Value: secret.Value})
	}
	return connect.NewResponse(&codespacev1.RequestRuntimeAccessResponse{
		Access: &codespacev1.RuntimeAccessBundle{
			GiteaToken:     result.Token,
			GiteaServerUrl: result.ServerURL,
			Secrets:        secrets,
			GitSshTrust:    &codespacev1.GitSSHTrust{KnownHostsLines: result.GitSSHKnownHosts},
		},
	}), nil
}

// RequestIdleStop authorizes an idle-triggered stop against current Gitea state.
func (s *Service) RequestIdleStop(
	ctx context.Context,
	req *connect.Request[codespacev1.RequestIdleStopRequest],
) (*connect.Response[codespacev1.RequestIdleStopResponse], error) {
	manager := GetManager(ctx)
	settings := req.Msg.GetObservedSettings()
	result, err := codespace_service.RequestIdleStop(ctx, manager, codespace_service.RequestIdleStopOptions{
		CodespaceUUID:                 req.Msg.GetRuntimeUuid(),
		ObservedAutoStopEnabled:       settings.GetAutoStopEnabled(),
		ObservedIdleTimeoutSeconds:    settings.GetIdleTimeoutSeconds(),
		ObservedInteractionGeneration: settings.GetInteractionGeneration(),
	})
	if err != nil {
		return nil, serviceFailureError(err, "invalid_argument", []serviceErrorCase{
			{target: codespace_service.ErrRequestIdleStopNotFound, code: connect.CodeNotFound, category: "codespace_not_found"},
			{target: codespace_service.ErrRequestIdleStopManagerMismatch, code: connect.CodeFailedPrecondition, category: "manager_mismatch"},
			{target: codespace_service.ErrRequestIdleStopManagerUnavailable, code: connect.CodeUnavailable, category: "manager_offline"},
			{target: codespace_service.ErrRequestIdleStopVersionExhausted, code: connect.CodeFailedPrecondition, category: "version_exhausted"},
		})
	}
	return connect.NewResponse(result), nil
}

// ValidatePublicEndpoint authorizes unauthenticated access to one public Endpoint.
func (s *Service) ValidatePublicEndpoint(
	ctx context.Context,
	req *connect.Request[codespacev1.ValidatePublicEndpointRequest],
) (*connect.Response[codespacev1.ValidatePublicEndpointResponse], error) {
	manager := GetManager(ctx)
	result, err := codespace_service.ValidatePublicEndpoint(ctx, manager, codespace_service.ValidatePublicEndpointOptions{
		CodespaceUUID: req.Msg.GetRuntimeUuid(),
		EndpointID:    req.Msg.GetEndpointId(),
	})
	if err != nil {
		return nil, failureError(connect.CodeInvalidArgument, "invalid_argument", err)
	}
	return connect.NewResponse(result), nil
}

// ValidateOpenToken authorizes one Gateway Open Token exchange.
func (s *Service) ValidateOpenToken(
	ctx context.Context,
	req *connect.Request[codespacev1.ValidateOpenTokenRequest],
) (*connect.Response[codespacev1.ValidateOpenTokenResponse], error) {
	manager := GetManager(ctx)
	result, err := codespace_service.ValidateOpenToken(ctx, manager, codespace_service.ValidateOpenTokenOptions{
		Code: req.Msg.GetCode(),
	})
	if err != nil {
		return nil, failureError(connect.CodeInternal, "internal_error", err)
	}
	return connect.NewResponse(result), nil
}

// VerifySSHPublicKey authorizes one new Gateway SSH transport.
func (s *Service) VerifySSHPublicKey(
	ctx context.Context,
	req *connect.Request[codespacev1.VerifySSHPublicKeyRequest],
) (*connect.Response[codespacev1.VerifySSHPublicKeyResponse], error) {
	manager := GetManager(ctx)
	result, err := codespace_service.VerifySSHPublicKey(ctx, manager, codespace_service.VerifySSHPublicKeyOptions{
		CodespaceUUID: req.Msg.GetRuntimeUuid(),
		PublicKey:     req.Msg.GetPublicKey(),
	})
	if err != nil {
		return nil, failureError(connect.CodeInvalidArgument, "invalid_argument", err)
	}
	return connect.NewResponse(result), nil
}

// RevalidateGatewaySession checks whether an existing Gateway session remains authorized.
func (s *Service) RevalidateGatewaySession(
	ctx context.Context,
	req *connect.Request[codespacev1.RevalidateGatewaySessionRequest],
) (*connect.Response[codespacev1.RevalidateGatewaySessionResponse], error) {
	manager := GetManager(ctx)
	result, err := codespace_service.RevalidateGatewaySession(ctx, manager, req.Msg)
	if err != nil {
		return nil, failureError(connect.CodeInvalidArgument, "invalid_argument", err)
	}
	return connect.NewResponse(result), nil
}
