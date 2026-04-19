// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package openapi3gen converts Gitea's Swagger 2.0 spec to an OpenAPI 3.0
// spec. It discovers Go enum type names by scanning swagger:enum annotations
// in the source tree, then names extracted shared-enum schemas accordingly.
package openapi3gen

import (
	"fmt"
	"sort"
	"strings"
)

// EnumKey returns a canonical key for a set of enum values: values are
// stringified, sorted, and joined with "|". Used to match enum value sets
// across spec properties and scanned Go type declarations.
func EnumKey(values []any) string {
	strs := make([]string, len(values))
	for i, v := range values {
		strs[i] = fmt.Sprintf("%v", v)
	}
	sort.Strings(strs)
	return strings.Join(strs, "|")
}
