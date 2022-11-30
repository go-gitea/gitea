// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_17 // nolint

// This migration added non-ideal indices to the action table which on larger datasets slowed things down
// it has been superceded by v218.go
