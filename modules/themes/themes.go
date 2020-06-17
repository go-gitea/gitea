// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package themes

import (
	"reflect"

	"code.gitea.io/gitea/modules/setting"
)

// Themes lists available themes
var Themes []string

// Init initializes theme-related variables
func Init() {
	if reflect.DeepEqual(setting.UI.Themes, []string{"*"}) {
		Themes = Discover()
	} else {
		Themes = setting.UI.Themes
	}
}
