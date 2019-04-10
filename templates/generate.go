// Copyright 2019-present The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build bindata

//go:generate go run github.com/jteeuwen/go-bindata/go-bindata -tags "bindata" -ignore "\\.go" -pkg "templates" -o "../modules/templates/bindata.go" ./...
//go:generate go fmt ../modules/templates/bindata.go

package generate
