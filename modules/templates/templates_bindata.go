// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//+build bindata

package templates

//go:generate go run -mod=vendor ../../scripts/generate-bindata.go ../../templates templates bindata.go
