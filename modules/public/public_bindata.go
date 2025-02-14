// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build bindata

package public

//go:generate go run ../../build/generate-bindata.go ../../public public bindata.go true
