// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !servedynamic

package schemas

import "embed"

// SchemasFS contains the schema files.
//
//go:embed *.json
var SchemasFS embed.FS
