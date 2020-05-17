// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"

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
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "type, t",
			Value: "",
			Usage: "Files type to migrate, currently should be attachments",
		},
		cli.StringFlag{
			Name:  "store, s",
			Value: "local",
			Usage: "New storage type, local or minio",
		},
		cli.StringFlag{
			Name:  "path, p",
			Value: "",
			Usage: "New storage placement if store is local",
		},
		cli.StringFlag{
			Name:  "minio-endpoint",
			Value: "",
			Usage: "New minio storage endpoint",
		},
		cli.StringFlag{
			Name:  "minio-access-key-id",
			Value: "",
			Usage: "New minio storage accessKeyID",
		},
		cli.StringFlag{
			Name:  "minio-secret-access-key",
			Value: "",
			Usage: "New minio storage secretAccessKey",
		},
		cli.StringFlag{
			Name:  "minio-bucket",
			Value: "",
			Usage: "New minio storage bucket",
		},
		cli.StringFlag{
			Name:  "minio-location",
			Value: "",
			Usage: "New minio storage location to create bucket",
		},
		cli.StringFlag{
			Name:  "minio-base-path",
			Value: "",
			Usage: "New minio storage basepath on the bucket",
		},
		cli.BoolFlag{
			Name:  "minio-use-ssl",
			Usage: "New minio storage SSL enabled",
		},
	},
}

func migrateAttachments(dstStorage storage.ObjectStorage) error {
	return models.IterateAttachment(func(attach *models.Attachment) error {
		_, err := storage.Copy(dstStorage, attach.RelativePath(), storage.Attachments, attach.RelativePath())
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

	if err := storage.Init(); err != nil {
		return err
	}

	tp := ctx.String("type")
	switch tp {
	case "attachments":
		var dstStorage storage.ObjectStorage
		var err error
		switch ctx.String("store") {
		case "local":
			dstStorage, err = storage.NewLocalStorage(ctx.String("path"))
		case "minio":
			dstStorage, err = storage.NewMinioStorage(
				ctx.String("minio-endpoint"),
				ctx.String("minio-access-key-id"),
				ctx.String("minio-secret-access-key"),
				ctx.String("minio-bucket"),
				ctx.String("minio-location"),
				ctx.String("minio-base-path"),
				ctx.Bool("minio-use-ssl"),
			)
		default:
			return fmt.Errorf("Unsupported attachments store type: %s", ctx.String("store"))
		}

		if err != nil {
			return err
		}
		return migrateAttachments(dstStorage)
	}

	return nil
}
