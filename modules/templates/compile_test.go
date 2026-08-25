// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"path/filepath"
	"runtime"
	"testing"

	"gitea.dev/modules/setting"
)

func TestTemplatesCompile(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	giteaRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))

	oldStaticRootPath := setting.StaticRootPath
	oldCustomPath := setting.CustomPath
	setting.StaticRootPath = giteaRoot
	setting.CustomPath = t.TempDir()
	t.Cleanup(func() {
		setting.StaticRootPath = oldStaticRootPath
		setting.CustomPath = oldCustomPath
	})

	if err := ReloadAllTemplates(); err != nil {
		t.Fatal(err)
	}
}
