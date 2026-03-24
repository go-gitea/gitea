// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build tools

package build

// This blank import ensures kin-openapi stays in go.mod for use by
// build/generate-openapi.go (which has //go:build ignore).
import (
	_ "github.com/getkin/kin-openapi/openapi2"
	_ "github.com/getkin/kin-openapi/openapi2conv"
	_ "github.com/getkin/kin-openapi/openapi3"
)
