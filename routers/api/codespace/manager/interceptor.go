// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package manager

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/modules/setting"
	codespace_service "gitea.dev/services/codespace"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connectproto"
)

const (
	managerIDHeader     = "x-codespace-manager-id"
	managerSecretHeader = "x-codespace-manager-secret"
	protocolVersion     = 1
)

type managerCtxKey struct{}

type versionedRequest interface {
	GetProtocolVersion() int32
}

func withManager(next connect.ServerFunc) connect.ServerFunc {
	return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
		info, _ := connect.CallInfoForServerContext(ctx)
		manager, err := authenticate(ctx, info.RequestHeader())
		if err != nil {
			switch {
			case errors.Is(err, codespace_service.ErrManagerUnregistered):
				return failureError(connect.CodeUnauthenticated, "manager_unregistered", err)
			default:
				return failureError(connect.CodeUnauthenticated, "unauthenticated", err)
			}
		}
		ctx = context.WithValue(ctx, managerCtxKey{}, manager)
		ctx, cancel := context.WithTimeout(ctx, setting.Codespace.ControlPlaneTimeout)
		defer cancel()
		err = next(ctx, spec, versionedStream{stream})
		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
				return connect.NewError(connect.CodeDeadlineExceeded, context.DeadlineExceeded.Error()).WithCause(context.DeadlineExceeded)
			}
			if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
				return connect.NewError(connect.CodeCanceled, context.Canceled.Error()).WithCause(context.Canceled)
			}
		}
		return err
	}
}

type versionedStream struct{ connect.ServerStream }

func (s versionedStream) Receive(message any) error {
	if err := s.ServerStream.Receive(message); err != nil {
		return err
	}
	if err := validateProtocolVersion(message); err != nil {
		return failureError(connect.CodeFailedPrecondition, "protocol_mismatch", err)
	}
	return nil
}

// GetManager returns the Manager authenticated before each ManagerService RPC.
func GetManager(ctx context.Context) *codespace_model.Manager {
	if value := ctx.Value(managerCtxKey{}); value != nil {
		if manager, ok := value.(*codespace_model.Manager); ok {
			return manager
		}
	}
	return nil
}

func authenticate(ctx context.Context, header *connect.Header) (*codespace_model.Manager, error) {
	managerID, err := strconv.ParseInt(header.Get(managerIDHeader), 10, 64)
	if err != nil {
		return nil, errors.New("invalid manager id")
	}
	manager, err := codespace_service.AuthenticateManager(ctx, managerID, header.Get(managerSecretHeader))
	if err != nil {
		return nil, err
	}
	return manager, nil
}

func validateProtocolVersion(message any) error {
	request, ok := message.(versionedRequest)
	if !ok {
		return errors.New("request does not carry protocol version")
	}
	if request.GetProtocolVersion() != protocolVersion {
		return fmt.Errorf("unsupported protocol version %d", request.GetProtocolVersion())
	}
	return nil
}

func failureError(code connect.Code, category string, err error) *connect.Error {
	connectErr := connect.NewError(code, err.Error()).WithCause(err)
	detail, detailErr := connectproto.NewErrorDetail(&codespacev1.FailureDetail{Category: category})
	if detailErr == nil {
		connectErr = connectErr.WithDetail(detail)
	}
	return connectErr
}

func failureErrorWithStaleGeneration(code connect.Code, category string, currentGeneration int64, err error) error {
	connectErr := failureError(code, category, err)
	staleDetail, staleDetailErr := connectproto.NewErrorDetail(&codespacev1.StaleGenerationDetail{CurrentGeneration: currentGeneration})
	if staleDetailErr == nil {
		connectErr = connectErr.WithDetail(staleDetail)
	}
	return connectErr
}

func failureErrorWithLogOffset(code connect.Code, category string, currentOffset int64, err error) error {
	connectErr := failureError(code, category, err)
	offsetDetail, offsetDetailErr := connectproto.NewErrorDetail(&codespacev1.LogOffsetDetail{CurrentOffset: currentOffset})
	if offsetDetailErr == nil {
		connectErr = connectErr.WithDetail(offsetDetail)
	}
	return connectErr
}
