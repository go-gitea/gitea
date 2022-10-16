// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package install

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoutes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	routes := Routes(ctx)
	assert.NotNil(t, routes)
	assert.EqualValues(t, "/", routes.R.Routes()[0].Pattern)
	assert.Nil(t, routes.R.Routes()[0].SubRoutes)
	assert.Len(t, routes.R.Routes()[0].Handlers, 2)
}
