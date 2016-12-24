// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package templates

//go:generate go-bindata -tags "bindata" -ignore "\\.go" -pkg "templates" -o "bindata.go" ../../templates/...
//go:generate go fmt bindata.go
//go:generate sed -i.bak s/..\/..\/templates\/// bindata.go
//go:generate rm -f bindata.go.bak
