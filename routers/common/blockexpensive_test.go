// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBlockExpensive(t *testing.T) {
	cases := []struct {
		expensive bool
		routePath string
	}{
		{false, "/user/xxx"},
		{false, "/login/xxx"},
		{true, "/{username}/{reponame}/archive/xxx"},
		{true, "/{username}/{reponame}/graph"},
		{true, "/{username}/{reponame}/src/xxx"},
		{false, "/{username}/{reponame}/media/branch/*"},
		{false, "/{username}/{reponame}/raw/branch/*"},
		{true, "/{username}/{reponame}/media/commit/*"},
		{true, "/{username}/{reponame}/raw/blob/{sha}"},
		{true, "/{username}/{reponame}/wiki/xxx"},
		{true, "/{username}/{reponame}/activity/xxx"},
	}
	for _, c := range cases {
		assert.Equal(t, c.expensive, isRoutePathExpensive(c.routePath), "routePath: %s", c.routePath)
	}
}
