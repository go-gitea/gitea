// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package build

import (
	// for lint
	_ "github.com/BurntSushi/toml"
	_ "github.com/mgechev/dots"
	_ "github.com/mgechev/revive/formatter"
	_ "github.com/mgechev/revive/lint"
	_ "github.com/mgechev/revive/rule"
	_ "github.com/mitchellh/go-homedir"

	// for embed
	_ "github.com/shurcooL/vfsgen"

	// for cover merge
	_ "golang.org/x/tools/cover"
)
