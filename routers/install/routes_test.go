// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package install

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoutes(t *testing.T) {
	// TODO: this test seems not really testing the handlers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	base := Routes(ctx)
	assert.NotNil(t, base)
	r := base.R.Routes()[1]
	routes := r.SubRoutes.Routes()[0]
	assert.EqualValues(t, "/", routes.Pattern)
	assert.Nil(t, routes.SubRoutes)
	assert.Len(t, routes.Handlers, 2)
}
