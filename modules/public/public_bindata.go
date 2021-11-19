// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build bindata
// +build bindata

package public

//go:generate go run -mod=vendor  ../../build/generate-bindata.go ../../public public bindata.go
