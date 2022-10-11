// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package runner

import (
	"context"
	"strings"

	bots_model "code.gitea.io/gitea/models/bots"

	"github.com/bufbuild/connect-go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var WithRunner = connect.WithInterceptors(connect.UnaryInterceptorFunc(func(unaryFunc connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		if methodName(request) == "Register" {
			return unaryFunc(ctx, request)
		}
		token := request.Header().Get("X-Runner-Token") // TODO: shouldn't be X-Runner-Token, maybe X-Runner-UUID
		runner, err := bots_model.GetRunnerByToken(token)
		if err != nil {
			if _, ok := err.(*bots_model.ErrRunnerNotExist); ok {
				return nil, status.Error(codes.Unauthenticated, "unregistered runner")
			}
			return nil, status.Error(codes.Internal, err.Error())
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
