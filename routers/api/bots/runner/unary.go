// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package runner

import (
	"context"
	"strings"

	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"

	"github.com/bufbuild/connect-go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const runnerOnlineTimeDeltaSecs = 30

var WithRunner = connect.WithInterceptors(connect.UnaryInterceptorFunc(func(unaryFunc connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		if methodName(request) == "Register" {
			return unaryFunc(ctx, request)
		}
		token := request.Header().Get("X-Runner-Token") // TODO: shouldn't be X-Runner-Token, maybe X-Runner-UUID
		runner, err := bots_model.GetRunnerByToken(token)
		if err != nil {
			if _, ok := err.(bots_model.ErrRunnerNotExist); ok {
				return nil, status.Error(codes.Unauthenticated, "unregistered runner")
			}
			return nil, status.Error(codes.Internal, err.Error())
		}

		// update runner online status
		if runner.Status == runnerv1.RunnerStatus_RUNNER_STATUS_OFFLINE {
			runner.LastOnline = timeutil.TimeStampNow()
			runner.Status = runnerv1.RunnerStatus_RUNNER_STATUS_ACTIVE
			if err := bots_model.UpdateRunner(ctx, runner, "last_online", "status"); err != nil {
				log.Error("can't update runner status: %v", err)
			}
		}
		if timeutil.TimeStampNow()-runner.LastOnline >= runnerOnlineTimeDeltaSecs {
			runner.LastOnline = timeutil.TimeStampNow()
			if err := bots_model.UpdateRunner(ctx, runner, "last_online"); err != nil {
				log.Error("can't update runner last_online: %v", err)
			}
		}

		ctx = context.WithValue(ctx, runnerCtxKey{}, runner)
		return unaryFunc(ctx, request)
	}
}))

func methodName(req connect.AnyRequest) string {
	splits := strings.Split(req.Spec().Procedure, "/")
	if len(splits) > 0 {
		return splits[len(splits)-1]
	}
	return ""
}

type runnerCtxKey struct{}

func GetRunner(ctx context.Context) *bots_model.Runner {
	if v := ctx.Value(runnerCtxKey{}); v != nil {
		if r, ok := v.(*bots_model.Runner); ok {
			return r
		}
	}
	return nil
}
