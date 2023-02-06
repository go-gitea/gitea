// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build bindata

package migration

//go:generate go run ../../build/generate-bindata.go ../../modules/migration/schemas migration bindata.go
