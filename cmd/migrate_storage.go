// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/migrations"
	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	packages_module "code.gitea.io/gitea/modules/packages"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"

	"github.com/urfave/cli"
)

// CmdMigrateStorage represents the available migrate storage sub-command.
var CmdMigrateStorage = cli.Command{
	Name:        "migrate-storage",
	Usage:       "Migrate the storage",
	Description: "Copies stored files from storage configured in app.ini to parameter-configured storage",
	Action:      runMigrateStorage,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "type, t",
			Value: "",
			Usage: "Type of stored files to copy.  Allowed types: 'attachments', 'lfs', 'avatars', 'repo-avatars', 'repo-archivers', 'packages', 'actions-log'",
		},
		cli.StringFlag{
			Name:  "storage, s",
			Value: "",
			Usage: "New storage type: local (default) or minio",
		},
		cli.StringFlag{
			Name:  "path, p",
			Value: "",
			Usage: "New storage placement if store is local (leave blank for default)",
		},
		cli.StringFlag{
			Name:  "minio-endpoint",
			Value: "",
			Usage: "Minio storage endpoint",
		},
		cli.StringFlag{
			Name:  "minio-access-key-id",
			Value: "",
			Usage: "Minio storage accessKeyID",
		},
		cli.StringFlag{
			Name:  "minio-secret-access-key",
			Value: "",
			Usage: "Minio storage secretAccessKey",
		},
		cli.StringFlag{
			Name:  "minio-bucket",
			Value: "",
			Usage: "Minio storage bucket",
		},
		cli.StringFlag{
			Name:  "minio-location",
			Value: "",
			Usage: "Minio storage location to create bucket",
		},
		cli.StringFlag{
			Name:  "minio-base-path",
			Value: "",
			Usage: "Minio storage base path on the bucket",
		},
		cli.BoolFlag{
			Name:  "minio-use-ssl",
			Usage: "Enable SSL for minio",
		},
		cli.BoolFlag{
			Name:  "minio-insecure-skip-verify",
			Usage: "Skip SSL verification",
		},
		cli.StringFlag{
			Name:  "minio-checksum-algorithm",
			Value: "",
			Usage: "Minio checksum algorithm (default/md5)",
		},
	},
}

func migrateAttachments(ctx context.Context, dstStorage storage.ObjectStorage) error {
	return db.Iterate(ctx, nil, func(ctx context.Context, attach *repo_model.Attachment) error {
		_, err := storage.Copy(dstStorage, attach.RelativePath(), storage.Attachments, attach.RelativePath())
		return err
	})
}

func migrateLFS(ctx context.Context, dstStorage storage.ObjectStorage) error {
	return db.Iterate(ctx, nil, func(ctx context.Context, mo *git_model.LFSMetaObject) error {
		_, err := storage.Copy(dstStorage, mo.RelativePath(), storage.LFS, mo.RelativePath())
		return err
	})
}

func migrateAvatars(ctx context.Context, dstStorage storage.ObjectStorage) error {
	return db.Iterate(ctx, nil, func(ctx context.Context, user *user_model.User) error {
		_, err := storage.Copy(dstStorage, user.CustomAvatarRelativePath(), storage.Avatars, user.CustomAvatarRelativePath())
		return err
	})
}

func migrateRepoAvatars(ctx context.Context, dstStorage storage.ObjectStorage) error {
	return db.Iterate(ctx, nil, func(ctx context.Context, repo *repo_model.Repository) error {
		_, err := storage.Copy(dstStorage, repo.CustomAvatarRelativePath(), storage.RepoAvatars, repo.CustomAvatarRelativePath())
		return err
	})
}

func migrateRepoArchivers(ctx context.Context, dstStorage storage.ObjectStorage) error {
	return db.Iterate(ctx, nil, func(ctx context.Context, archiver *repo_model.RepoArchiver) error {
		p := archiver.RelativePath()
		_, err := storage.Copy(dstStorage, p, storage.RepoArchives, p)
		return err
	})
}

func migratePackages(ctx context.Context, dstStorage storage.ObjectStorage) error {
	return db.Iterate(ctx, nil, func(ctx context.Context, pb *packages_model.PackageBlob) error {
		p := packages_module.KeyToRelativePath(packages_module.BlobHash256Key(pb.HashSHA256))
		_, err := storage.Copy(dstStorage, p, storage.Packages, p)
		return err
	})
}

func migrateActionsLog(ctx context.Context, dstStorage storage.ObjectStorage) error {
	return db.Iterate(ctx, nil, func(ctx context.Context, task *actions_model.ActionTask) error {
		if task.LogExpired {
			// the log has been cleared
			return nil
		}
		if !task.LogInStorage {
			// running tasks store logs in DBFS
			return nil
		}
		p := task.LogFilename
		_, err := storage.Copy(dstStorage, p, storage.Actions, p)
		return err
	})
}

func runMigrateStorage(ctx *cli.Context) error {
	stdCtx, cancel := installSignals()
	defer cancel()

	if err := initDB(stdCtx); err != nil {
		return err
	}

	log.Info("AppPath: %s", setting.AppPath)
	log.Info("AppWorkPath: %s", setting.AppWorkPath)
	log.Info("Custom path: %s", setting.CustomPath)
	log.Info("Log path: %s", setting.Log.RootPath)
	log.Info("Configuration file: %s", setting.CustomConf)

	if err := db.InitEngineWithMigration(context.Background(), migrations.Migrate); err != nil {
		log.Fatal("Failed to initialize ORM engine: %v", err)
		return err
	}

	if err := storage.Init(); err != nil {
		return err
	}

	var dstStorage storage.ObjectStorage
	var err error
	switch strings.ToLower(ctx.String("storage")) {
	case "":
		fallthrough
	case string(storage.LocalStorageType):
		p := ctx.String("path")
		if p == "" {
			log.Fatal("Path must be given when storage is loal")
			return nil
		}
		dstStorage, err = storage.NewLocalStorage(
			stdCtx,
			storage.LocalStorageConfig{
				Path: p,
			})
	case string(storage.MinioStorageType):
		dstStorage, err = storage.NewMinioStorage(
			stdCtx,
			storage.MinioStorageConfig{
				Endpoint:           ctx.String("minio-endpoint"),
				AccessKeyID:        ctx.String("minio-access-key-id"),
				SecretAccessKey:    ctx.String("minio-secret-access-key"),
				Bucket:             ctx.String("minio-bucket"),
				Location:           ctx.String("minio-location"),
				BasePath:           ctx.String("minio-base-path"),
				UseSSL:             ctx.Bool("minio-use-ssl"),
				InsecureSkipVerify: ctx.Bool("minio-insecure-skip-verify"),
				ChecksumAlgorithm:  ctx.String("minio-checksum-algorithm"),
			})
	default:
		return fmt.Errorf("unsupported storage type: %s", ctx.String("storage"))
	}
	if err != nil {
		return err
	}

	migratedMethods := map[string]func(context.Context, storage.ObjectStorage) error{
		"attachments":    migrateAttachments,
		"lfs":            migrateLFS,
		"avatars":        migrateAvatars,
		"repo-avatars":   migrateRepoAvatars,
		"repo-archivers": migrateRepoArchivers,
		"packages":       migratePackages,
		"actions-log":    migrateActionsLog,
	}

	tp := strings.ToLower(ctx.String("type"))
	if m, ok := migratedMethods[tp]; ok {
		if err := m(stdCtx, dstStorage); err != nil {
			return err
		}
		log.Info("%s files have successfully been copied to the new storage.", tp)
		return nil
	}

	return fmt.Errorf("unsupported storage: %s", ctx.String("type"))
}
