// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"flag"
	"testing"
)

var (
	uploadToken = flag.String("upload-token", "", "token for uploading")
)

func TestMain(m *testing.M) {
	m.Run()
}
