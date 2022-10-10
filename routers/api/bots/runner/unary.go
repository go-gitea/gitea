// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package runner

import (
	"context"
	"strings"

	"code.gitea.io/gitea/models/bots"

	"github.com/bufbuild/connect-go"
)

var WithRunner = connect.WithInterceptors(connect.UnaryInterceptorFunc(func(unaryFunc connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		if methodName(request) == "Register" {
			return unaryFunc(ctx, request)
		}
		uuid := request.Header().Get("X-Runner-Token") // TODO: shouldn't be X-Runner-Token, maybe X-Runner-UUID
		// TODO: get runner from db, refuse request if it doesn't exist
		r := &bots.Runner{
			UUID: uuid,
		}
		ctx = context.WithValue(ctx, runnerCtxKey{}, r)
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

func GetRunner(ctx context.Context) *bots.Runner {
	if v := ctx.Value(runnerCtxKey{}); v != nil {
		if r, ok := v.(*bots.Runner); ok {
			return r
		}
	}
	return nil
}
