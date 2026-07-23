// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package manager

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/modules/setting"
	codespace_service "gitea.dev/services/codespace"

	"connectrpc.com/connect"
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

var withManager = connect.WithInterceptors(connect.UnaryInterceptorFunc(func(unaryFunc connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		methodName := getMethodName(request)
		if methodName != "RegisterManager" {
			manager, err := authenticate(ctx, request.Header())
			if err != nil {
				switch {
				case errors.Is(err, codespace_service.ErrManagerUnregistered):
					return nil, failureError(connect.CodeUnauthenticated, "manager_unregistered", err)
				default:
					return nil, failureError(connect.CodeUnauthenticated, "unauthenticated", err)
				}
			}
			ctx = context.WithValue(ctx, managerCtxKey{}, manager)
		}
		if err := validateProtocolVersion(request.Any()); err != nil {
			return nil, failureError(connect.CodeFailedPrecondition, "protocol_mismatch", err)
		}
		ctx, cancel := context.WithTimeout(ctx, setting.Codespace.ControlPlaneTimeout)
		defer cancel()
		response, err := unaryFunc(ctx, request)
		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
				return nil, connect.NewError(connect.CodeDeadlineExceeded, context.DeadlineExceeded)
			}
			if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
				return nil, connect.NewError(connect.CodeCanceled, context.Canceled)
			}
		}
		return response, err
	}
}))

// GetManager returns the Manager authenticated by the interceptor.
func GetManager(ctx context.Context) *codespace_model.Manager {
	if value := ctx.Value(managerCtxKey{}); value != nil {
		if manager, ok := value.(*codespace_model.Manager); ok {
			return manager
		}
	}
	return nil
}

func authenticate(ctx context.Context, header http.Header) (*codespace_model.Manager, error) {
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

func getMethodName(req connect.AnyRequest) string {
	parts := strings.Split(req.Spec().Procedure, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func failureError(code connect.Code, category string, err error) error {
	connectErr := connect.NewError(code, err)
	detail, detailErr := connect.NewErrorDetail(&codespacev1.FailureDetail{Category: category})
	if detailErr == nil {
		connectErr.AddDetail(detail)
	}
	return connectErr
}

func failureErrorWithStaleGeneration(code connect.Code, category string, currentGeneration int64, err error) error {
	connectErr := connect.NewError(code, err)
	failureDetail, failureDetailErr := connect.NewErrorDetail(&codespacev1.FailureDetail{Category: category})
	if failureDetailErr == nil {
		connectErr.AddDetail(failureDetail)
	}
	staleDetail, staleDetailErr := connect.NewErrorDetail(&codespacev1.StaleGenerationDetail{CurrentGeneration: currentGeneration})
	if staleDetailErr == nil {
		connectErr.AddDetail(staleDetail)
	}
	return connectErr
}

func failureErrorWithLogOffset(code connect.Code, category string, currentOffset int64, err error) error {
	connectErr := connect.NewError(code, err)
	failureDetail, failureDetailErr := connect.NewErrorDetail(&codespacev1.FailureDetail{Category: category})
	if failureDetailErr == nil {
		connectErr.AddDetail(failureDetail)
	}
	offsetDetail, offsetDetailErr := connect.NewErrorDetail(&codespacev1.LogOffsetDetail{CurrentOffset: currentOffset})
	if offsetDetailErr == nil {
		connectErr.AddDetail(offsetDetail)
	}
	return connectErr
}
