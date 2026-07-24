// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package manager

import (
	"context"
	"errors"
	"net/http"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	"gitea.dev/codespace-proto-go/codespace/v1/codespacev1connect"
	"gitea.dev/modules/setting"
	codespace_service "gitea.dev/services/codespace"

	"connectrpc.com/connect"
)

// NewManagerServiceHandler returns the Codespace ManagerService Connect handler.
func NewManagerServiceHandler() (string, http.Handler) {
	return codespacev1connect.NewManagerServiceHandler(
		&Service{},
		connect.WithCompressMinBytes(1024),
		connect.WithReadMaxBytes(int(setting.Codespace.ControlPlaneMaxSize)),
		connect.WithSendMaxBytes(int(setting.Codespace.ControlPlaneMaxSize)),
		withManager,
	)
}

var _ codespacev1connect.ManagerServiceHandler = (*Service)(nil)

// Service implements the Codespace ManagerService RPC entrypoint.
type Service struct {
	codespacev1connect.UnimplementedManagerServiceHandler
}

// RegisterManager exchanges a registration token for a Manager identity.
func (s *Service) RegisterManager(
	ctx context.Context,
	req *connect.Request[codespacev1.RegisterManagerRequest],
) (*connect.Response[codespacev1.RegisterManagerResponse], error) {
	manager, secret, err := codespace_service.RegisterManager(ctx, req.Msg.GetRegistrationToken())
	if err != nil {
		if errors.Is(err, codespace_service.ErrRegistrationUnauthenticated) {
			return nil, failureError(connect.CodeUnauthenticated, "unauthenticated", err)
		}
		if errors.Is(err, codespace_service.ErrRegistrationStateUnavailable) {
			return nil, failureError(connect.CodeFailedPrecondition, "state_unavailable", err)
		}
		return nil, failureError(connect.CodeInternal, "internal_error", err)
	}
	return connect.NewResponse(&codespacev1.RegisterManagerResponse{
		ManagerId:     manager.ID,
		ManagerSecret: secret,
	}), nil
}

// DeclareManager stores the authenticated Manager's current declaration.
func (s *Service) DeclareManager(
	ctx context.Context,
	req *connect.Request[codespacev1.DeclareManagerRequest],
) (*connect.Response[codespacev1.DeclareManagerResponse], error) {
	manager, err := requireManager(ctx)
	if err != nil {
		return nil, failureError(connect.CodeUnauthenticated, "unauthenticated", err)
	}
	if err := codespace_service.DeclareManager(ctx, manager, codespace_service.DeclareManagerOptions{
		GatewayURL:                         req.Msg.GetGatewayUrl(),
		GatewaySSHAddr:                     req.Msg.GetGatewaySshAddr(),
		Tags:                               req.Msg.GetTags(),
		Version:                            req.Msg.GetVersion(),
		Name:                               req.Msg.GetName(),
		RuntimeState:                       managerRuntimeState(req.Msg.GetManagerRuntimeState()),
		GatewaySSHHostKeyAlgorithm:         req.Msg.GetGatewaySshHostKeyAlgorithm(),
		GatewaySSHHostKeyFingerprintSHA256: req.Msg.GetGatewaySshHostKeyFingerprintSha256(),
		GatewaySSHHostKeyUpdatedUnix:       req.Msg.GetGatewaySshHostKeyUpdatedUnix(),
		CapacityTotal:                      req.Msg.GetCapacityTotal(),
		CapacityAvailable:                  req.Msg.GetCapacityAvailable(),
	}); err != nil {
		switch {
		case errors.Is(err, codespace_service.ErrDeclareGatewayURLConflict):
			return nil, failureError(connect.CodeFailedPrecondition, "gateway_url_conflict", err)
		case errors.Is(err, codespace_service.ErrDeclareGatewaySSHAddrConflict):
			return nil, failureError(connect.CodeFailedPrecondition, "gateway_ssh_addr_conflict", err)
		case errors.Is(err, codespace_service.ErrDeclareGatewayCookieScopeConflict):
			return nil, failureError(connect.CodeFailedPrecondition, "gateway_cookie_scope_conflict", err)
		}
		return nil, failureError(connect.CodeInvalidArgument, "invalid_declaration", err)
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
	manager, err := requireManager(ctx)
	if err != nil {
		return nil, failureError(connect.CodeUnauthenticated, "unauthenticated", err)
	}
	acceptedTypes, err := acceptedOperationTypes(req.Msg.GetAcceptedOperationTypes())
	if err != nil {
		return nil, failureError(connect.CodeInvalidArgument, "invalid_argument", err)
	}
	observed := make([]codespace_service.ObservedOperation, 0, len(req.Msg.GetObservedOperations()))
	for _, item := range req.Msg.GetObservedOperations() {
		observed = append(observed, codespace_service.ObservedOperation{
			CodespaceUUID:     item.GetCodespaceUuid(),
			OperationRVersion: item.GetOperationRversion(),
		})
	}
	result, err := codespace_service.FetchOperations(ctx, manager, codespace_service.FetchOperationsOptions{
		CapacityAvailable:        req.Msg.GetCapacityAvailable(),
		AcceptedOperationTypes:   acceptedTypes,
		MaxOperations:            req.Msg.GetMaxOperations(),
		ObservedOperations:       observed,
		CleanupCapacityAvailable: req.Msg.GetCleanupCapacityAvailable(),
	})
	if err != nil {
		switch {
		case errors.Is(err, codespace_service.ErrFetchStateHistoryConflict):
			return nil, failureError(connect.CodeFailedPrecondition, "state_history_conflict", err)
		case errors.Is(err, codespace_service.ErrFetchManagerUnavailable):
			return nil, failureError(connect.CodeUnavailable, "manager_offline", err)
		default:
			return nil, failureError(connect.CodeInvalidArgument, "invalid_argument", err)
		}
	}
	response, err := fetchOperationsResponse(result)
	if err != nil {
		return nil, failureError(connect.CodeInternal, "internal_error", err)
	}
	return connect.NewResponse(response), nil
}

// ReportInstances accepts a complete Runtime inventory snapshot.
func (s *Service) ReportInstances(
	ctx context.Context,
	req *connect.Request[codespacev1.ReportInstancesRequest],
) (*connect.Response[codespacev1.ReportInstancesResponse], error) {
	manager, err := requireManager(ctx)
	if err != nil {
		return nil, failureError(connect.CodeUnauthenticated, "unauthenticated", err)
	}
	instances := make([]codespace_service.RuntimeInstanceRef, 0, len(req.Msg.GetInstances()))
	for _, instance := range req.Msg.GetInstances() {
		runtimeState, err := runtimeInstanceState(instance.GetRuntimeState())
		if err != nil {
			return nil, failureError(connect.CodeInvalidArgument, "invalid_argument", err)
		}
		instances = append(instances, codespace_service.RuntimeInstanceRef{
			CodespaceUUID:             instance.GetCodespaceUuid(),
			RuntimeState:              runtimeState,
			ObservedOperationRVersion: instance.GetObservedOperationRversion(),
		})
	}
	result, err := codespace_service.ReportInstances(ctx, manager, codespace_service.ReportInstancesOptions{
		InventoryGeneration: req.Msg.GetInventoryGeneration(),
		Instances:           instances,
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
	response, err := reportInstancesResponse(result)
	if err != nil {
		return nil, failureError(connect.CodeInternal, "internal_error", err)
	}
	return connect.NewResponse(response), nil
}

// FinalizeOperation reports the authenticated Manager's final operation result.
func (s *Service) FinalizeOperation(
	ctx context.Context,
	req *connect.Request[codespacev1.FinalizeOperationRequest],
) (*connect.Response[codespacev1.FinalizeOperationResponse], error) {
	manager, err := requireManager(ctx)
	if err != nil {
		return nil, failureError(connect.CodeUnauthenticated, "unauthenticated", err)
	}
	finalStatus, err := finalStatus(req.Msg.GetFinal().GetStatus())
	if err != nil {
		return nil, failureError(connect.CodeInvalidArgument, "invalid_argument", err)
	}
	operationType, err := operationType(req.Msg.GetFinal().GetOperationType())
	if err != nil {
		return nil, failureError(connect.CodeInvalidArgument, "invalid_argument", err)
	}
	outcome, err := codespace_service.FinalizeOperation(ctx, manager, codespace_service.FinalizeOperationOptions{
		CodespaceUUID:     req.Msg.GetCodespaceUuid(),
		OperationRVersion: req.Msg.GetOperationRversion(),
		OperationType:     operationType,
		FinalStatus:       finalStatus,
	})
	if err != nil {
		if errors.Is(err, codespace_service.ErrFinalizeGiteaTokenRequired) {
			return nil, failureError(connect.CodeFailedPrecondition, "gitea_token_required", err)
		}
		if errors.Is(err, codespace_service.ErrFinalizeMetadataRequired) {
			return nil, failureError(connect.CodeFailedPrecondition, "metadata_required", err)
		}
		return nil, failureError(connect.CodeInvalidArgument, "invalid_argument", err)
	}
	return connect.NewResponse(finalizeResponse(outcome)), nil
}

// UpdateLog appends sanitized log lines for the authenticated Manager's active operation.
func (s *Service) UpdateLog(
	ctx context.Context,
	req *connect.Request[codespacev1.UpdateLogRequest],
) (*connect.Response[codespacev1.UpdateLogResponse], error) {
	manager, err := requireManager(ctx)
	if err != nil {
		return nil, failureError(connect.CodeUnauthenticated, "unauthenticated", err)
	}
	lines := make([]codespace_service.LogLine, 0, len(req.Msg.GetLines()))
	for _, line := range req.Msg.GetLines() {
		lines = append(lines, codespace_service.LogLine{
			TimestampUnixNano: line.GetTimestampUnixNano(),
			Message:           line.GetMessage(),
		})
	}
	result, err := codespace_service.UpdateLog(ctx, manager, codespace_service.UpdateLogOptions{
		CodespaceUUID:     req.Msg.GetCodespaceUuid(),
		OperationRVersion: req.Msg.GetOperationRversion(),
		Offset:            req.Msg.GetOffset(),
		Lines:             lines,
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
	return connect.NewResponse(&codespacev1.UpdateLogResponse{
		NextOffset: result.NextOffset,
	}), nil
}

// ReportRuntimeMetadata stores the authenticated Manager's current Runtime Metadata snapshot.
func (s *Service) ReportRuntimeMetadata(
	ctx context.Context,
	req *connect.Request[codespacev1.ReportRuntimeMetadataRequest],
) (*connect.Response[codespacev1.ReportRuntimeMetadataResponse], error) {
	manager, err := requireManager(ctx)
	if err != nil {
		return nil, failureError(connect.CodeUnauthenticated, "unauthenticated", err)
	}
	err = codespace_service.ReportRuntimeMetadata(ctx, manager, codespace_service.ReportRuntimeMetadataOptions{
		CodespaceUUID:      req.Msg.GetCodespaceUuid(),
		MetadataJSON:       req.Msg.GetMetadataJson(),
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
	manager, err := requireManager(ctx)
	if err != nil {
		return nil, failureError(connect.CodeUnauthenticated, "unauthenticated", err)
	}
	runtimeState, err := runtimeTransitionState(req.Msg.GetRuntimeState())
	if err != nil {
		return nil, failureError(connect.CodeInvalidArgument, "invalid_argument", err)
	}
	err = codespace_service.ReportRuntimeTransition(ctx, manager, codespace_service.ReportRuntimeTransitionOptions{
		CodespaceUUID:             req.Msg.GetCodespaceUuid(),
		RuntimeGeneration:         req.Msg.GetRuntimeGeneration(),
		ObservedOperationRVersion: req.Msg.GetObservedOperationRversion(),
		RuntimeState:              runtimeState,
	})
	if err != nil {
		return nil, reportRuntimeTransitionError(err)
	}
	return connect.NewResponse(&codespacev1.ReportRuntimeTransitionResponse{}), nil
}

func reportRuntimeMetadataError(err error) error {
	return reportRuntimeError(err, []runtimeErrorCase{
		{target: codespace_service.ErrRuntimeMetadataGenerationConflict, code: connect.CodeFailedPrecondition, category: "generation_conflict"},
		{target: codespace_service.ErrRuntimeMetadataVersionExhausted, code: connect.CodeFailedPrecondition, category: "version_exhausted"},
		{target: codespace_service.ErrRuntimeMetadataManagerMismatch, code: connect.CodeFailedPrecondition, category: "manager_mismatch"},
		{target: codespace_service.ErrRuntimeMetadataStaleOperation, code: connect.CodeFailedPrecondition, category: "stale_operation"},
		{target: codespace_service.ErrRuntimeMetadataManagerOffline, code: connect.CodeUnavailable, category: "manager_offline"},
		{target: codespace_service.ErrRuntimeMetadataStateUnavailable, code: connect.CodeFailedPrecondition, category: "state_unavailable"},
	})
}

func reportRuntimeTransitionError(err error) error {
	return reportRuntimeError(err, []runtimeErrorCase{
		{target: codespace_service.ErrRuntimeTransitionNotFound, code: connect.CodeNotFound, category: "codespace_not_found"},
		{target: codespace_service.ErrRuntimeTransitionManagerMismatch, code: connect.CodeFailedPrecondition, category: "manager_mismatch"},
		{target: codespace_service.ErrRuntimeTransitionCurrentOperationConflict, code: connect.CodeAborted, category: "current_operation_conflict"},
		{target: codespace_service.ErrRuntimeTransitionManagerOffline, code: connect.CodeUnavailable, category: "manager_offline"},
		{target: codespace_service.ErrRuntimeTransitionStaleOperation, code: connect.CodeFailedPrecondition, category: "stale_operation"},
		{target: codespace_service.ErrRuntimeTransitionGenerationConflict, code: connect.CodeFailedPrecondition, category: "generation_conflict"},
	})
}

type runtimeErrorCase struct {
	target   error
	code     connect.Code
	category string
}

func reportRuntimeError(err error, cases []runtimeErrorCase) error {
	var staleGeneration *codespace_service.StaleGenerationError
	if errors.As(err, &staleGeneration) {
		return failureErrorWithStaleGeneration(connect.CodeFailedPrecondition, "stale_generation", staleGeneration.CurrentGeneration, err)
	}
	for _, errCase := range cases {
		if errors.Is(err, errCase.target) {
			return failureError(errCase.code, errCase.category, err)
		}
	}
	return failureError(connect.CodeInvalidArgument, "invalid_argument", err)
}

// RequestGiteaToken returns or issues the authenticated Manager's current Codespace Token.
func (s *Service) RequestGiteaToken(
	ctx context.Context,
	req *connect.Request[codespacev1.RequestGiteaTokenRequest],
) (*connect.Response[codespacev1.RequestGiteaTokenResponse], error) {
	manager, err := requireManager(ctx)
	if err != nil {
		return nil, failureError(connect.CodeUnauthenticated, "unauthenticated", err)
	}
	result, err := codespace_service.RequestGiteaToken(ctx, manager, codespace_service.RequestGiteaTokenOptions{
		CodespaceUUID: req.Msg.GetCodespaceUuid(),
	})
	if err != nil {
		switch {
		case errors.Is(err, codespace_service.ErrRequestGiteaTokenNotFound):
			return nil, failureError(connect.CodeNotFound, "codespace_not_found", err)
		case errors.Is(err, codespace_service.ErrRequestGiteaTokenManagerMismatch):
			return nil, failureError(connect.CodeFailedPrecondition, "manager_mismatch", err)
		case errors.Is(err, codespace_service.ErrRequestGiteaTokenStateUnavailable):
			return nil, failureError(connect.CodeFailedPrecondition, "state_unavailable", err)
		case errors.Is(err, codespace_service.ErrRequestGiteaTokenManagerOffline):
			return nil, failureError(connect.CodeUnavailable, "manager_offline", err)
		case errors.Is(err, codespace_service.ErrRequestGiteaTokenUserNotFound):
			return nil, failureError(connect.CodeFailedPrecondition, "user_not_found", err)
		default:
			return nil, failureError(connect.CodeInvalidArgument, "invalid_argument", err)
		}
	}
	return connect.NewResponse(&codespacev1.RequestGiteaTokenResponse{
		Token:     result.Token,
		ServerUrl: result.ServerURL,
	}), nil
}

// EnsureCodespaceGitSSHKey creates or confirms the Codespace Git SSH key.
func (s *Service) EnsureCodespaceGitSSHKey(
	ctx context.Context,
	req *connect.Request[codespacev1.EnsureCodespaceGitSSHKeyRequest],
) (*connect.Response[codespacev1.EnsureCodespaceGitSSHKeyResponse], error) {
	manager, err := requireManager(ctx)
	if err != nil {
		return nil, failureError(connect.CodeUnauthenticated, "unauthenticated", err)
	}
	result, err := codespace_service.EnsureGitSSHKey(ctx, manager, codespace_service.EnsureGitSSHKeyOptions{
		CodespaceUUID: req.Msg.GetCodespaceUuid(),
		PublicKey:     req.Msg.GetPublicKey(),
	})
	if err != nil {
		switch {
		case errors.Is(err, codespace_service.ErrEnsureGitSSHKeyNotFound):
			return nil, failureError(connect.CodeNotFound, "codespace_not_found", err)
		case errors.Is(err, codespace_service.ErrEnsureGitSSHKeyManagerMismatch):
			return nil, failureError(connect.CodeFailedPrecondition, "manager_mismatch", err)
		case errors.Is(err, codespace_service.ErrEnsureGitSSHKeyStateUnavailable):
			return nil, failureError(connect.CodeFailedPrecondition, "state_unavailable", err)
		case errors.Is(err, codespace_service.ErrEnsureGitSSHKeyManagerOffline):
			return nil, failureError(connect.CodeUnavailable, "manager_offline", err)
		case errors.Is(err, codespace_service.ErrEnsureGitSSHKeyUserNotFound):
			return nil, failureError(connect.CodeFailedPrecondition, "user_not_found", err)
		case errors.Is(err, codespace_service.ErrEnsureGitSSHKeyLoginRestricted):
			return nil, failureError(connect.CodeFailedPrecondition, "login_restricted", err)
		case errors.Is(err, codespace_service.ErrEnsureGitSSHKeyInvalidPublicKey):
			return nil, failureError(connect.CodeInvalidArgument, "invalid_public_key", err)
		case errors.Is(err, codespace_service.ErrEnsureGitSSHKeyConflict):
			return nil, failureError(connect.CodeFailedPrecondition, "key_conflict", err)
		case errors.Is(err, codespace_service.ErrEnsureGitSSHKeyIntegrity):
			return nil, failureError(connect.CodeInternal, "internal_error", err)
		default:
			return nil, failureError(connect.CodeInvalidArgument, "invalid_argument", err)
		}
	}
	return connect.NewResponse(&codespacev1.EnsureCodespaceGitSSHKeyResponse{
		KnownHostsLines: result.KnownHostsLines,
	}), nil
}

// RequestIdleStop authorizes an idle-triggered stop against current Gitea state.
func (s *Service) RequestIdleStop(
	ctx context.Context,
	req *connect.Request[codespacev1.RequestIdleStopRequest],
) (*connect.Response[codespacev1.RequestIdleStopResponse], error) {
	manager, err := requireManager(ctx)
	if err != nil {
		return nil, failureError(connect.CodeUnauthenticated, "unauthenticated", err)
	}
	result, err := codespace_service.RequestIdleStop(ctx, manager, codespace_service.RequestIdleStopOptions{
		CodespaceUUID:                 req.Msg.GetCodespaceUuid(),
		ObservedAutoStopEnabled:       req.Msg.GetObservedAutoStopEnabled(),
		ObservedIdleTimeoutSeconds:    req.Msg.GetObservedIdleTimeoutSeconds(),
		ObservedInteractionGeneration: req.Msg.GetObservedInteractionGeneration(),
	})
	if err != nil {
		switch {
		case errors.Is(err, codespace_service.ErrRequestIdleStopNotFound):
			return nil, failureError(connect.CodeNotFound, "codespace_not_found", err)
		case errors.Is(err, codespace_service.ErrRequestIdleStopManagerMismatch):
			return nil, failureError(connect.CodeFailedPrecondition, "manager_mismatch", err)
		case errors.Is(err, codespace_service.ErrRequestIdleStopManagerUnavailable):
			return nil, failureError(connect.CodeUnavailable, "manager_offline", err)
		case errors.Is(err, codespace_service.ErrRequestIdleStopVersionExhausted):
			return nil, failureError(connect.CodeFailedPrecondition, "version_exhausted", err)
		default:
			return nil, failureError(connect.CodeInvalidArgument, "invalid_argument", err)
		}
	}
	response, err := requestIdleStopResponse(result)
	if err != nil {
		return nil, failureError(connect.CodeInternal, "internal_error", err)
	}
	return connect.NewResponse(response), nil
}

// ValidatePublicEndpoint authorizes unauthenticated access to one public Endpoint.
func (s *Service) ValidatePublicEndpoint(
	ctx context.Context,
	req *connect.Request[codespacev1.ValidatePublicEndpointRequest],
) (*connect.Response[codespacev1.ValidatePublicEndpointResponse], error) {
	manager, err := requireManager(ctx)
	if err != nil {
		return nil, failureError(connect.CodeUnauthenticated, "unauthenticated", err)
	}
	result, err := codespace_service.ValidatePublicEndpoint(ctx, manager, codespace_service.ValidatePublicEndpointOptions{
		CodespaceUUID: req.Msg.GetCodespaceUuid(),
		EndpointID:    req.Msg.GetEndpointId(),
	})
	if err != nil {
		return nil, failureError(connect.CodeInvalidArgument, "invalid_argument", err)
	}
	return connect.NewResponse(validatePublicEndpointResponse(result)), nil
}

// ValidateOpenToken authorizes one Gateway Open Token exchange.
func (s *Service) ValidateOpenToken(
	ctx context.Context,
	req *connect.Request[codespacev1.ValidateOpenTokenRequest],
) (*connect.Response[codespacev1.ValidateOpenTokenResponse], error) {
	manager, err := requireManager(ctx)
	if err != nil {
		return nil, failureError(connect.CodeUnauthenticated, "unauthenticated", err)
	}
	result, err := codespace_service.ValidateOpenToken(ctx, manager, codespace_service.ValidateOpenTokenOptions{
		Code: req.Msg.GetCode(),
	})
	if err != nil {
		return nil, failureError(connect.CodeInternal, "internal_error", err)
	}
	return connect.NewResponse(validateOpenTokenResponse(result)), nil
}

// VerifySSHPublicKey authorizes one new Gateway SSH transport.
func (s *Service) VerifySSHPublicKey(
	ctx context.Context,
	req *connect.Request[codespacev1.VerifySSHPublicKeyRequest],
) (*connect.Response[codespacev1.VerifySSHPublicKeyResponse], error) {
	manager, err := requireManager(ctx)
	if err != nil {
		return nil, failureError(connect.CodeUnauthenticated, "unauthenticated", err)
	}
	result, err := codespace_service.VerifySSHPublicKey(ctx, manager, codespace_service.VerifySSHPublicKeyOptions{
		CodespaceUUID: req.Msg.GetCodespaceUuid(),
		PublicKey:     req.Msg.GetPublicKey(),
	})
	if err != nil {
		return nil, failureError(connect.CodeInvalidArgument, "invalid_argument", err)
	}
	return connect.NewResponse(verifySSHPublicKeyResponse(result)), nil
}

// RevalidateGatewaySession checks whether an existing Gateway session remains authorized.
func (s *Service) RevalidateGatewaySession(
	ctx context.Context,
	req *connect.Request[codespacev1.RevalidateGatewaySessionRequest],
) (*connect.Response[codespacev1.RevalidateGatewaySessionResponse], error) {
	manager, err := requireManager(ctx)
	if err != nil {
		return nil, failureError(connect.CodeUnauthenticated, "unauthenticated", err)
	}
	opts, err := revalidateGatewaySessionOptions(req.Msg)
	if err != nil {
		return nil, failureError(connect.CodeInvalidArgument, "invalid_argument", err)
	}
	result, err := codespace_service.RevalidateGatewaySession(ctx, manager, opts)
	if err != nil {
		return nil, failureError(connect.CodeInvalidArgument, "invalid_argument", err)
	}
	return connect.NewResponse(revalidateGatewaySessionResponse(result)), nil
}
