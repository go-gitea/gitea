// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"

	base "gitea.dev/modules/migration"
	"gitea.dev/modules/structs"
)

func init() {
	RegisterDownloaderFactory(&ForgejoDownloaderFactory{})
}

// ForgejoDownloaderFactory uses the Gitea downloader, as Forgejo keeps a Gitea compatible API
type ForgejoDownloaderFactory struct{}

func (f *ForgejoDownloaderFactory) New(ctx context.Context, opts base.MigrateOptions) (base.Downloader, error) {
	return (&GiteaDownloaderFactory{}).New(ctx, opts)
}

func (f *ForgejoDownloaderFactory) GitServiceType() structs.GitServiceType {
	return structs.ForgejoService
}
