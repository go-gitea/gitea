// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package runner

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"time"

	"gitea.dev/actionslib/pkg/protocol"
	actions_model "gitea.dev/models/actions"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/modules/log"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"connectrpc.com/connect/v2"
	lru "github.com/hashicorp/golang-lru/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	uuidHeaderKey  = protocol.UUIDHeader
	tokenHeaderKey = protocol.TokenHeader
)

type runnerTokenHash struct{ salt, hash string }

var verifiedRunnerTokens, _ = lru.New[runnerTokenHash, [sha256.Size]byte](10000) // not auth_model.TokenCache, whose small default size thrashes once more runners poll in turn

// withErrorMessages sends plain errors to runners as CodeUnknown with their message, like connect v1 did.
func withErrorMessages(next connect.ServerFunc) connect.ServerFunc {
	return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
		err := next(ctx, spec, stream)
		if _, ok := errors.AsType[*connect.Error](err); err == nil || ok {
			return err
		}
		return connect.NewError(connect.CodeUnknown, err.Error()).WithCause(err)
	}
}

func withRunner(next connect.ServerFunc) connect.ServerFunc {
	// A plain gRPC status.Error is treated as unknown by Connect and becomes HTTP 500
	// To respond an HTTP status code, use connect.Error instead
	errUnregisteredRunner := connect.NewError(connect.CodeUnauthenticated, "unregistered runner")
	return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
		methodName := getMethodName(spec)
		if methodName == "Register" {
			return next(ctx, spec, stream)
		}
		info, _ := connect.CallInfoForServerContext(ctx)
		uuid := info.RequestHeader().Get(uuidHeaderKey)
		token := info.RequestHeader().Get(tokenHeaderKey)

		runner, err := actions_model.GetRunnerByUUID(ctx, uuid)
		if err != nil {
			if errors.Is(err, util.ErrNotExist) {
				return errUnregisteredRunner
			}
			return status.Error(codes.Internal, err.Error())
		}
		if !verifyRunnerToken(runner, token) {
			return errUnregisteredRunner
		}

		now := time.Now()
		cols := make([]string, 0, 2)
		// Debounce last_active too: while a runner streams logs, UpdateLog fires
		// many times per second and writing on each is a major source of DB load.
		// Persist only when stale enough to affect the active/idle status.
		if (methodName == "UpdateTask" || methodName == "UpdateLog") &&
			actions_model.ShouldPersistLastActive(runner.LastActive, now) {
			runner.LastActive = timeutil.TimeStamp(now.Unix())
			cols = append(cols, "last_active")
		}
		// Debounce last_online: writing on every poll is a major source of DB load
		// with many runners. Persist only when stale enough to affect offline status.
		if actions_model.ShouldPersistLastOnline(runner.LastOnline, now) {
			runner.LastOnline = timeutil.TimeStamp(now.Unix())
			cols = append(cols, "last_online")
		}
		if len(cols) > 0 {
			if err := actions_model.UpdateRunner(ctx, runner, cols...); err != nil {
				log.Error("can't update runner status: %v", err)
			}
		}

		ctx = context.WithValue(ctx, runnerCtxKey{}, runner)
		return next(ctx, spec, stream)
	}
}

func verifyRunnerToken(runner *actions_model.ActionRunner, token string) bool {
	key, digest := runnerTokenHash{runner.TokenSalt, runner.TokenHash}, sha256.Sum256([]byte(token))
	if cached, ok := verifiedRunnerTokens.Get(key); ok && util.CryptoConstTimeEqual(cached[:], digest[:]) {
		return true
	}
	if !util.CryptoConstTimeEqual(runner.TokenHash, auth_model.HashToken(token, runner.TokenSalt)) {
		return false
	}
	verifiedRunnerTokens.Add(key, digest)
	return true
}

func getMethodName(spec connect.Spec) string {
	splits := strings.Split(spec.Procedure, "/")
	if len(splits) > 0 {
		return splits[len(splits)-1]
	}
	return ""
}

type runnerCtxKey struct{}

func GetRunner(ctx context.Context) *actions_model.ActionRunner {
	if v := ctx.Value(runnerCtxKey{}); v != nil {
		if r, ok := v.(*actions_model.ActionRunner); ok {
			return r
		}
	}
	return nil
}
