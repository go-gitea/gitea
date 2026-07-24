// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package manager

import (
	"errors"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	codespace_service "gitea.dev/services/codespace"
)

func managerRuntimeState(state codespacev1.ManagerRuntimeState) string {
	switch state {
	case codespacev1.ManagerRuntimeState_MANAGER_RUNTIME_STATE_ONLINE:
		return codespace_model.ManagerRuntimeStateOnline
	case codespacev1.ManagerRuntimeState_MANAGER_RUNTIME_STATE_RECOVERING:
		return codespace_model.ManagerRuntimeStateRecovering
	default:
		return ""
	}
}

func finalStatus(status codespacev1.FinalStatus) (string, error) {
	switch status {
	case codespacev1.FinalStatus_FINAL_STATUS_DONE:
		return codespace_service.FinalStatusDone, nil
	case codespacev1.FinalStatus_FINAL_STATUS_FAILED:
		return codespace_service.FinalStatusFailed, nil
	default:
		return "", errors.New("invalid final status")
	}
}

func operationType(operationType codespacev1.OperationType) (string, error) {
	switch operationType {
	case codespacev1.OperationType_OPERATION_TYPE_CREATE:
		return codespace_model.OperationCreate, nil
	case codespacev1.OperationType_OPERATION_TYPE_RESUME:
		return codespace_model.OperationResume, nil
	case codespacev1.OperationType_OPERATION_TYPE_STOP:
		return codespace_model.OperationStop, nil
	case codespacev1.OperationType_OPERATION_TYPE_DELETE:
		return codespace_model.OperationDelete, nil
	default:
		return "", errors.New("invalid operation type")
	}
}

func runtimeTransitionState(state codespacev1.RuntimeState) (string, error) {
	switch state {
	case codespacev1.RuntimeState_RUNTIME_STATE_STOPPED:
		return codespace_service.RuntimeTransitionStopped, nil
	case codespacev1.RuntimeState_RUNTIME_STATE_FAILED:
		return codespace_service.RuntimeTransitionFailed, nil
	default:
		return "", errors.New("invalid runtime transition state")
	}
}

func runtimeInstanceState(state codespacev1.RuntimeState) (string, error) {
	switch state {
	case codespacev1.RuntimeState_RUNTIME_STATE_CREATING:
		return codespace_service.RuntimeInstanceStateCreating, nil
	case codespacev1.RuntimeState_RUNTIME_STATE_RUNNING:
		return codespace_service.RuntimeInstanceStateRunning, nil
	case codespacev1.RuntimeState_RUNTIME_STATE_STOPPED:
		return codespace_service.RuntimeInstanceStateStopped, nil
	case codespacev1.RuntimeState_RUNTIME_STATE_FAILED:
		return codespace_service.RuntimeInstanceStateFailed, nil
	default:
		return "", errors.New("invalid runtime instance state")
	}
}

func acceptedOperationTypes(values []codespacev1.AcceptedOperationType) ([]string, error) {
	acceptedTypes := make([]string, 0, len(values))
	for _, value := range values {
		switch value {
		case codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_CREATE:
			acceptedTypes = append(acceptedTypes, codespace_service.AcceptedOperationCreate)
		case codespacev1.AcceptedOperationType_ACCEPTED_OPERATION_TYPE_RESUME:
			acceptedTypes = append(acceptedTypes, codespace_service.AcceptedOperationResume)
		default:
			return nil, errors.New("invalid accepted operation type")
		}
	}
	return acceptedTypes, nil
}

func revalidateGatewaySessionOptions(msg *codespacev1.RevalidateGatewaySessionRequest) (codespace_service.RevalidateGatewaySessionOptions, error) {
	if endpoint := msg.GetEndpoint(); endpoint != nil {
		return codespace_service.RevalidateGatewaySessionOptions{
			Kind:          codespace_service.RevalidateSessionEndpoint,
			UserID:        endpoint.GetUserId(),
			CodespaceUUID: endpoint.GetCodespaceUuid(),
			EndpointID:    endpoint.GetEndpointId(),
		}, nil
	}
	if sshSession := msg.GetSsh(); sshSession != nil {
		return codespace_service.RevalidateGatewaySessionOptions{
			Kind:          codespace_service.RevalidateSessionSSH,
			UserID:        sshSession.GetUserId(),
			CodespaceUUID: sshSession.GetCodespaceUuid(),
		}, nil
	}
	return codespace_service.RevalidateGatewaySessionOptions{}, errors.New("session is required")
}

func fetchOperationsResponse(result *codespace_service.FetchOperationsResult) (*codespacev1.FetchOperationsResponse, error) {
	response := &codespacev1.FetchOperationsResponse{
		Operations:    make([]*codespacev1.OperationPayload, 0, len(result.Operations)),
		RenewedLeases: make([]*codespacev1.RenewedOperationLease, 0, len(result.RenewedLeases)),
	}
	for _, operation := range result.Operations {
		payload, err := operationPayload(operation)
		if err != nil {
			return nil, err
		}
		response.Operations = append(response.Operations, payload)
	}
	for _, lease := range result.RenewedLeases {
		response.RenewedLeases = append(response.RenewedLeases, &codespacev1.RenewedOperationLease{
			CodespaceUuid:             lease.CodespaceUUID,
			OperationRversion:         lease.OperationRVersion,
			LeaseValidForMilliseconds: lease.LeaseValidForMilliseconds,
		})
	}
	return response, nil
}

func reportInstancesResponse(result *codespace_service.ReportInstancesResult) (*codespacev1.ReportInstancesResponse, error) {
	response := &codespacev1.ReportInstancesResponse{
		Results: make([]*codespacev1.RuntimeInstanceResult, 0, len(result.Results)),
	}
	for _, item := range result.Results {
		resultItem := &codespacev1.RuntimeInstanceResult{
			CodespaceUuid: item.CodespaceUUID,
		}
		if item.RuntimeSettings != nil {
			resultItem.RuntimeSettings = runtimeSettings(*item.RuntimeSettings)
		}
		switch item.Action {
		case "":
		case codespace_service.InventoryActionCleanupLocalRuntime:
			resultItem.Action = &codespacev1.RuntimeInstanceResult_CleanupLocalRuntime{CleanupLocalRuntime: &codespacev1.CleanupLocalRuntime{}}
		case codespace_service.InventoryActionReportRuntimeTransition:
			resultItem.Action = &codespacev1.RuntimeInstanceResult_ReportRuntimeTransition{
				ReportRuntimeTransition: &codespacev1.ReportRuntimeTransitionAction{CurrentOperationRversion: item.CurrentOperationRVersion},
			}
		case codespace_service.InventoryActionRefetchOperation:
			resultItem.Action = &codespacev1.RuntimeInstanceResult_RefetchOperation{
				RefetchOperation: &codespacev1.RefetchOperation{CurrentOperationRversion: item.CurrentOperationRVersion},
			}
		case codespace_service.InventoryActionStopLocalRuntime:
			resultItem.Action = &codespacev1.RuntimeInstanceResult_StopLocalRuntime{
				StopLocalRuntime: &codespacev1.StopLocalRuntime{CurrentOperationRversion: item.CurrentOperationRVersion},
			}
		case codespace_service.InventoryActionClearOperationContext:
			resultItem.Action = &codespacev1.RuntimeInstanceResult_ClearOperationContext{
				ClearOperationContext: &codespacev1.ClearOperationContext{CurrentOperationRversion: item.CurrentOperationRVersion},
			}
		default:
			return nil, errors.New("unsupported inventory action")
		}
		response.Results = append(response.Results, resultItem)
	}
	return response, nil
}

func operationPayload(operation codespace_service.OperationPayload) (*codespacev1.OperationPayload, error) {
	payload := &codespacev1.OperationPayload{
		OperationRversion:         operation.OperationRVersion,
		CodespaceUuid:             operation.CodespaceUUID,
		LogOffset:                 operation.LogOffset,
		LeaseValidForMilliseconds: operation.LeaseValidForMilliseconds,
	}
	switch operation.Command {
	case codespace_model.OperationCreate:
		payload.Command = &codespacev1.OperationPayload_Create{Create: createPayload(operation.Create)}
	case codespace_model.OperationResume:
		payload.Command = &codespacev1.OperationPayload_Resume{Resume: resumePayload(operation.Resume)}
	case codespace_model.OperationStop:
		payload.Command = &codespacev1.OperationPayload_Stop{Stop: &codespacev1.StopOperationPayload{}}
	case codespace_model.OperationDelete:
		payload.Command = &codespacev1.OperationPayload_Delete{Delete: &codespacev1.DeleteOperationPayload{}}
	case codespace_service.OperationCommandAbortCreate:
		payload.Command = &codespacev1.OperationPayload_AbortCreate{AbortCreate: &codespacev1.AbortCreateOperationPayload{}}
	case codespace_service.OperationCommandAbortResume:
		payload.Command = &codespacev1.OperationPayload_AbortResume{AbortResume: &codespacev1.AbortResumeOperationPayload{}}
	default:
		return nil, errors.New("unsupported operation command")
	}
	return payload, nil
}

func createPayload(payload *codespace_service.CreateOperationPayload) *codespacev1.CreateOperationPayload {
	if payload == nil {
		return nil
	}
	return &codespacev1.CreateOperationPayload{
		RepoId:             payload.RepoID,
		RepoFullName:       payload.RepoFullName,
		RepoName:           payload.RepoName,
		RepoCloneHttpUrl:   payload.RepoCloneHTTPURL,
		RepoWebUrl:         payload.RepoWebURL,
		OwnerId:            payload.OwnerID,
		OwnerName:          payload.OwnerName,
		OwnerType:          payload.OwnerType,
		OwnerDisplayName:   payload.OwnerDisplayName,
		CodespaceOwnerName: payload.CodespaceOwnerName,
		StartRef:           payload.StartRef,
		RefType:            payload.RefType,
		RefName:            payload.RefName,
		CommitSha:          payload.CommitSHA,
		RepoTag:            payload.RepoTag,
		RuntimeSettings:    runtimeSettings(payload.RuntimeSettings),
		GitProtocol:        gitProtocol(payload.GitProtocol),
		RepoCloneSshUrl:    payload.RepoCloneSSHURL,
	}
}

func resumePayload(payload *codespace_service.ResumeOperationPayload) *codespacev1.ResumeOperationPayload {
	if payload == nil {
		return nil
	}
	return &codespacev1.ResumeOperationPayload{
		RuntimeSettings: runtimeSettings(payload.RuntimeSettings),
		GitProtocol:     gitProtocol(payload.GitProtocol),
	}
}

func runtimeSettings(settings codespace_service.RuntimeSettings) *codespacev1.EffectiveCodespaceRuntimeSettings {
	return &codespacev1.EffectiveCodespaceRuntimeSettings{
		AutoStopEnabled:       settings.AutoStopEnabled,
		IdleTimeoutSeconds:    settings.IdleTimeoutSeconds,
		InteractionGeneration: settings.InteractionGeneration,
	}
}

func gitProtocol(protocol string) codespacev1.GitProtocol {
	switch protocol {
	case codespace_model.GitProtocolSSH:
		return codespacev1.GitProtocol_GIT_PROTOCOL_SSH
	default:
		return codespacev1.GitProtocol_GIT_PROTOCOL_HTTP
	}
}

func requestIdleStopResponse(result *codespace_service.RequestIdleStopResult) (*codespacev1.RequestIdleStopResponse, error) {
	switch result.Outcome {
	case codespace_service.IdleStopOutcomePending:
		return &codespacev1.RequestIdleStopResponse{
			Outcome: &codespacev1.RequestIdleStopResponse_Pending{
				Pending: &codespacev1.IdleStopPending{
					OperationRversion: result.OperationRVersion,
				},
			},
		}, nil
	case codespace_service.IdleStopOutcomeObservationChanged:
		return &codespacev1.RequestIdleStopResponse{
			Outcome: &codespacev1.RequestIdleStopResponse_ObservationChanged{
				ObservationChanged: &codespacev1.IdleStopObservationChanged{
					RuntimeSettings: runtimeSettings(result.RuntimeSettings),
				},
			},
		}, nil
	case codespace_service.IdleStopOutcomeNotApplicable:
		return &codespacev1.RequestIdleStopResponse{
			Outcome: &codespacev1.RequestIdleStopResponse_NotApplicable{
				NotApplicable: &codespacev1.IdleStopNotApplicable{
					Reason: idleStopNotApplicableReason(result.NotApplicableReason),
				},
			},
		}, nil
	default:
		return nil, errors.New("unsupported idle stop outcome")
	}
}

func idleStopNotApplicableReason(reason string) codespacev1.IdleStopNotApplicableReason {
	switch reason {
	case codespace_service.IdleStopReasonOperationConflict:
		return codespacev1.IdleStopNotApplicableReason_IDLE_STOP_NOT_APPLICABLE_REASON_OPERATION_CONFLICT
	case codespace_service.IdleStopReasonAlreadyStopped:
		return codespacev1.IdleStopNotApplicableReason_IDLE_STOP_NOT_APPLICABLE_REASON_ALREADY_STOPPED
	default:
		return codespacev1.IdleStopNotApplicableReason_IDLE_STOP_NOT_APPLICABLE_REASON_STATE_UNAVAILABLE
	}
}

func validatePublicEndpointResponse(result *codespace_service.ValidatePublicEndpointResult) *codespacev1.ValidatePublicEndpointResponse {
	if result.Allowed {
		return &codespacev1.ValidatePublicEndpointResponse{
			Outcome: &codespacev1.ValidatePublicEndpointResponse_Allowed{
				Allowed: &codespacev1.PublicEndpointAllowed{},
			},
		}
	}
	return &codespacev1.ValidatePublicEndpointResponse{
		Outcome: &codespacev1.ValidatePublicEndpointResponse_Denied{
			Denied: &codespacev1.FailureDetail{Category: result.DeniedCategory},
		},
	}
}

func validateOpenTokenResponse(result *codespace_service.ValidateOpenTokenResult) *codespacev1.ValidateOpenTokenResponse {
	if result.Allowed {
		return &codespacev1.ValidateOpenTokenResponse{
			Outcome: &codespacev1.ValidateOpenTokenResponse_Allowed{
				Allowed: &codespacev1.OpenTokenBinding{
					UserId:                result.UserID,
					CodespaceUuid:         result.CodespaceUUID,
					EndpointId:            result.EndpointID,
					InteractionGeneration: result.InteractionGeneration,
				},
			},
		}
	}
	return &codespacev1.ValidateOpenTokenResponse{
		Outcome: &codespacev1.ValidateOpenTokenResponse_Denied{
			Denied: &codespacev1.FailureDetail{Category: result.DeniedCategory},
		},
	}
}

func verifySSHPublicKeyResponse(result *codespace_service.VerifySSHPublicKeyResult) *codespacev1.VerifySSHPublicKeyResponse {
	if result.Allowed {
		return &codespacev1.VerifySSHPublicKeyResponse{
			Outcome: &codespacev1.VerifySSHPublicKeyResponse_Allowed{
				Allowed: &codespacev1.SSHAuthBinding{
					UserId:                result.UserID,
					InteractionGeneration: result.InteractionGeneration,
				},
			},
		}
	}
	return &codespacev1.VerifySSHPublicKeyResponse{
		Outcome: &codespacev1.VerifySSHPublicKeyResponse_Denied{
			Denied: &codespacev1.FailureDetail{Category: result.DeniedCategory},
		},
	}
}

func revalidateGatewaySessionResponse(result *codespace_service.RevalidateGatewaySessionResult) *codespacev1.RevalidateGatewaySessionResponse {
	if result.Allowed {
		return &codespacev1.RevalidateGatewaySessionResponse{
			Outcome: &codespacev1.RevalidateGatewaySessionResponse_Allowed{
				Allowed: &codespacev1.SessionAllowed{},
			},
		}
	}
	return &codespacev1.RevalidateGatewaySessionResponse{
		Outcome: &codespacev1.RevalidateGatewaySessionResponse_Denied{
			Denied: &codespacev1.FailureDetail{Category: result.DeniedCategory},
		},
	}
}

func finalizeResponse(outcome codespace_service.FinalizeOutcome) *codespacev1.FinalizeOperationResponse {
	switch outcome {
	case codespace_service.FinalizeOutcomeAccepted:
		return &codespacev1.FinalizeOperationResponse{
			Outcome: &codespacev1.FinalizeOperationResponse_FinalAccepted{FinalAccepted: &codespacev1.FinalAccepted{}},
		}
	case codespace_service.FinalizeOutcomeIdempotent:
		return &codespacev1.FinalizeOperationResponse{
			Outcome: &codespacev1.FinalizeOperationResponse_IdempotentDone{IdempotentDone: &codespacev1.IdempotentDone{}},
		}
	case codespace_service.FinalizeOutcomeResourceAbsent:
		return &codespacev1.FinalizeOperationResponse{
			Outcome: &codespacev1.FinalizeOperationResponse_ResourceAbsent{ResourceAbsent: &codespacev1.ResourceAbsent{}},
		}
	default:
		return &codespacev1.FinalizeOperationResponse{
			Outcome: &codespacev1.FinalizeOperationResponse_StaleOperation{StaleOperation: &codespacev1.StaleOperation{}},
		}
	}
}
