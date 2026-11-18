// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package manager

import (
	"context"
	"errors"
	"strconv"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	"gitea.dev/codespace-proto-go/codespace/v1/codespacev1connect"
	codespace_model "gitea.dev/models/codespace"
	codespace_service "gitea.dev/services/codespace"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connectproto"
)

const (
	managerIDHeader       = "x-codespace-manager-id"
	managerSecretHeader   = "x-codespace-manager-secret"
	managerRequestTimeout = 30 * time.Second
)

type managerCtxKey struct{}

func withManager(next connect.ServerFunc) connect.ServerFunc {
	return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
		ctx, cancel := context.WithTimeout(ctx, managerRequestTimeout)
		defer cancel()
		if spec.Procedure == codespacev1connect.ManagerServiceRegisterManagerProcedure {
			return next(ctx, spec, stream)
		}
		info, _ := connect.CallInfoForServerContext(ctx)
		manager, err := authenticate(ctx, info.RequestHeader())
		if err != nil {
			switch {
			case errors.Is(err, codespace_service.ErrManagerUnregistered):
				return failureError(connect.CodeUnauthenticated, "manager_unregistered", err)
			case errors.Is(err, codespace_service.ErrManagerUnauthenticated):
				return failureError(connect.CodeUnauthenticated, "unauthenticated", err)
			default:
				return serviceFailureError(err, "invalid_argument", nil)
			}
		}
		ctx = context.WithValue(ctx, managerCtxKey{}, manager)
		err = next(ctx, spec, stream)
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
		return nil, codespace_service.ErrManagerUnauthenticated
	}
	manager, err := codespace_service.AuthenticateManager(ctx, managerID, header.Get(managerSecretHeader))
	if err != nil {
		return nil, err
	}
	return manager, nil
}

func failureError(code connect.Code, category string, err error) *connect.Error {
	connectErr := connect.NewError(code, err.Error()).WithCause(err)
	detail, detailErr := connectproto.NewErrorDetail(&codespacev1.FailureDetail{Category: category})
	if detailErr == nil {
		connectErr = connectErr.WithDetail(detail)
	}
	return connectErr
}
