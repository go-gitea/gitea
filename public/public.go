// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package public

//go:generate go-bindata -tags bindata -ignore "\\.go|\\.less" -pkg public -o bindata.go ./...
//go:generate go fmt bindata.go

type Options struct {
	Directory   string
	SkipLogging bool
}
