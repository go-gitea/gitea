// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package templates

//go:generate packr2
//go:generate sh -c "echo '// +build bindata' | cat - packrd/packed-packr.go > packrd/packed-packr.go.bak"
//go:generate sh -c "echo 'package packrd' > packrd/packr.go"
//go:generate sh -c "echo '// +build bindata' | cat - templates-packr.go > templates-packr.go.bak"
//go:generate sh -c "mv packrd/packed-packr.go.bak packrd/packed-packr.go"
//go:generate sh -c "mv templates-packr.go.bak templates-packr.go"
