// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"context"
)

type envType string

// WithEnvs with envs on context
func WithEnvs(ctx context.Context, envs []string) context.Context {
	return context.WithValue(ctx, envType("envs"), envs)
}

// GetEnvs returns the envs
func GetEnvs(ctx context.Context) interface{} {
	return ctx.Value(envType("envs"))
}
