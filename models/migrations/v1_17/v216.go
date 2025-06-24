// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_17 //nolint:revive // underscore in migration packages isn't a large issue

// This migration added non-ideal indices to the action table which on larger datasets slowed things down
// it has been superseded by v218.go
