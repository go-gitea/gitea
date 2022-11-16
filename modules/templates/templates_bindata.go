// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build bindata

package templates

//go:generate go run ../../build/generate-bindata.go ../../templates templates bindata.go true
