// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package runner

import (
	"context"
	"crypto/subtle"
	"strings"

	auth_model "code.gitea.io/gitea/models/auth"
	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/bufbuild/connect-go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	uuidHeaderKey  = "x-runner-uuid"
	tokenHeaderKey = "x-runner-token"
)

var WithRunner = connect.WithInterceptors(connect.UnaryInterceptorFunc(func(unaryFunc connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		methodName := getMethodName(request)
		if methodName == "Register" {
			return unaryFunc(ctx, request)
		}
		uuid := request.Header().Get(uuidHeaderKey)
		token := request.Header().Get(tokenHeaderKey)
		runner, err := bots_model.GetRunnerByUUID(uuid)
		if err != nil {
			if _, ok := err.(bots_model.ErrRunnerNotExist); ok {
				return nil, status.Error(codes.Unauthenticated, "unregistered runner")
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
		if subtle.ConstantTimeCompare([]byte(runner.TokenHash), []byte(auth_model.HashToken(token, runner.TokenSalt))) != 1 {
			return nil, status.Error(codes.Unauthenticated, "unregistered runner")
		}

		cols := []string{"last_online"}
		runner.LastOnline = timeutil.TimeStampNow()
		if methodName == "UpdateTask" || methodName == "UpdateLog" {
			runner.LastActive = timeutil.TimeStampNow()
			cols = append(cols, "last_active")
		}
		if err := bots_model.UpdateRunner(ctx, runner, cols...); err != nil {
			log.Error("can't update runner status: %v", err)
		}

		ctx = context.WithValue(ctx, runnerCtxKey{}, runner)
		return unaryFunc(ctx, request)
	}
}))

func getMethodName(req connect.AnyRequest) string {
	splits := strings.Split(req.Spec().Procedure, "/")
	if len(splits) > 0 {
		return splits[len(splits)-1]
	}
	return ""
}

type runnerCtxKey struct{}

func GetRunner(ctx context.Context) *bots_model.BotRunner {
	if v := ctx.Value(runnerCtxKey{}); v != nil {
		if r, ok := v.(*bots_model.BotRunner); ok {
			return r
		}
	}
	return nil
}
