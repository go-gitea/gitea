// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package graphql

import (
	"context"
	"fmt"

	api "code.gitea.io/gitea/modules/context"
)

// Query query port
type Query struct {
}

// Viewer return current user message
func (v *Query) Viewer(ctx context.Context) (*User, error) {
	apiCtx := ctx.Value("default_api_context").(*api.APIContext)
	if apiCtx == nil {
		return nil, fmt.Errorf("ctx is empty")
	}

	if !apiCtx.IsSigned {
		return nil, fmt.Errorf("user is not login")
	}

	return convertUser(apiCtx.User, true), nil
}
