// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//+build bindata

package options

//go:generate go run -mod=vendor ../../scripts/generate-bindata.go ../../options options bindata.go
