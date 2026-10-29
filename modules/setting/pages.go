// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// Pages contains the storage configuration for Pages content.
var Pages = struct{ Storage *Storage }{}

func loadPagesFrom(rootCfg ConfigProvider) error {
	var err error
	Pages.Storage, err = getStorage(rootCfg, "pages", "", nil)
	return err
}
