// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/migrations"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"

	"github.com/urfave/cli"
)

// CmdMigrateStorage represents the available migrate storage sub-command.
var CmdMigrateStorage = cli.Command{
	Name:        "migrate-storage",
	Usage:       "Migrate the storage",
	Description: "This is a command for migrating storage.",
	Action:      runMigrateStorage,
}

func migrateAttachments(dstStorage storage.ObjectStorage) error {
	return models.IterateAttachment(func(attach *models.Attachment) error {
		_, err := storage.Copy(dstStorage, attach.UUID, storage.Attachments, attach.RelativePath())
		return err
	})
}

func runMigrateStorage(ctx *cli.Context) error {
	if err := initDB(); err != nil {
		return err
	}

	log.Trace("AppPath: %s", setting.AppPath)
	log.Trace("AppWorkPath: %s", setting.AppWorkPath)
	log.Trace("Custom path: %s", setting.CustomPath)
	log.Trace("Log path: %s", setting.LogRootPath)
	setting.InitDBConfig()

	if err := models.NewEngine(context.Background(), migrations.Migrate); err != nil {
		log.Fatal("Failed to initialize ORM engine: %v", err)
		return err
	}

	tp := ctx.String("type")

	// TODO: init setting

	if err := storage.Init(); err != nil {
		return err
	}

	switch tp {
	case "attachments":
		dstStorage, err := storage.NewLocalStorage(ctx.String("dst"))
		if err != nil {
			return err
		}
		return migrateAttachments(dstStorage)
	}

	return nil
}
