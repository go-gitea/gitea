// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import "gitea.dev/modules/structs"

func init() {
	RegisterDownloaderFactory(&ForgejoDownloaderFactory{})
}

// ForgejoDownloaderFactory uses the Gitea downloader, as Forgejo keeps a Gitea compatible API
type ForgejoDownloaderFactory struct {
	GiteaDownloaderFactory
}

func (f *ForgejoDownloaderFactory) GitServiceType() structs.GitServiceType {
	return structs.ForgejoService
}
