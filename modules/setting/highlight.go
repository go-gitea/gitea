// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"code.gitea.io/gitea/modules/highlight"
)

func newHighlightService() {
	keys := Cfg.Section("highlight.mapping").Keys()

	for i := range keys {
		highlight.HighlightMapping[keys[i].Name()] = keys[i].Value()
	}
}
