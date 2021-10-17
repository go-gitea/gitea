// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"path/filepath"

	"code.gitea.io/gitea/modules/appstate"
	"code.gitea.io/gitea/modules/log"
)

// AppState contains the state items for the app
var AppState appstate.StateStore

func newAppState() {
	var err error
	appStatePath := filepath.Join(AppDataPath, "appstate")
	AppState, err = appstate.NewFileStore(appStatePath)
	if err != nil {
		log.Fatal("failed to init AppState, err = %v", err)
	}
}
