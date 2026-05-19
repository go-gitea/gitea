// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	packages_module "code.gitea.io/gitea/modules/packages"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/services/versioned_migration"

	"github.com/urfave/cli/v3"
)

func newMigrateStorageCommand() *cli.Command {
	return &cli.Command{
		Name:        "migrate-storage",
		Usage:       "Migrate the storage",
		Description: "Copies stored files from storage configured in app.ini to parameter-configured storage",
		Action:      runMigrateStorage,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "type",
				Aliases: []string{"t"},
				Value:   "",
				Usage:   "Type of stored files to copy.  Allowed types: 'attachments', 'lfs', 'avatars', 'repo-avatars', 'repo-archivers', 'packages', 'actions-log', 'actions-artifacts'",
			},
			&cli.StringFlag{
				Name:    "storage",
				Aliases: []string{"s"},
				Value:   "",
				Usage:   "New storage type: local (default), s3 or azureblob",
			},
			&cli.StringFlag{
				Name:    "path",
				Aliases: []string{"p"},
				Value:   "",
				Usage:   "New storage placement if store is local (leave blank for default)",
			},
			// S3-compatible storage configuration. The `minio-*` aliases are
			// deprecated and accepted for backward compatibility.
			&cli.StringFlag{
				Name:    "s3-endpoint",
				Aliases: []string{"minio-endpoint"},
				Value:   "",
				Usage:   "S3 storage endpoint",
			},
			&cli.StringFlag{
				Name:    "s3-access-key-id",
				Aliases: []string{"minio-access-key-id"},
				Value:   "",
				Usage:   "S3 storage accessKeyID",
			},
			&cli.StringFlag{
				Name:    "s3-secret-access-key",
				Aliases: []string{"minio-secret-access-key"},
				Value:   "",
				Usage:   "S3 storage secretAccessKey",
			},
			&cli.StringFlag{
				Name:    "s3-bucket",
				Aliases: []string{"minio-bucket"},
				Value:   "",
				Usage:   "S3 storage bucket",
			},
			&cli.StringFlag{
				Name:    "s3-location",
				Aliases: []string{"minio-location"},
				Value:   "",
				Usage:   "S3 storage location to create bucket",
			},
			&cli.StringFlag{
				Name:    "s3-base-path",
				Aliases: []string{"minio-base-path"},
				Value:   "",
				Usage:   "S3 storage base path on the bucket",
			},
			&cli.BoolFlag{
				Name:    "s3-use-ssl",
				Aliases: []string{"minio-use-ssl"},
				Usage:   "Enable SSL for S3",
			},
			&cli.BoolFlag{
				Name:    "s3-insecure-skip-verify",
				Aliases: []string{"minio-insecure-skip-verify"},
				Usage:   "Skip SSL verification",
			},
			&cli.StringFlag{
				Name:    "s3-checksum-algorithm",
				Aliases: []string{"minio-checksum-algorithm"},
				Value:   "",
				Usage:   "S3 checksum algorithm (default/md5)",
			},
			&cli.StringFlag{
				Name:    "s3-bucket-lookup-type",
				Aliases: []string{"minio-bucket-lookup-type"},
				Value:   "",
				Usage:   "S3 bucket lookup type",
			},
			// Azure Blob Storage special configurations
			&cli.StringFlag{
				Name:  "azureblob-endpoint",
				Value: "",
				Usage: "Azure Blob storage endpoint",
			},
			&cli.StringFlag{
				Name:  "azureblob-account-name",
				Value: "",
				Usage: "Azure Blob storage account name",
			},
			&cli.StringFlag{
				Name:  "azureblob-account-key",
				Value: "",
				Usage: "Azure Blob storage account key",
			},
			&cli.StringFlag{
				Name:  "azureblob-container",
				Value: "",
				Usage: "Azure Blob storage container",
			},
			&cli.StringFlag{
				Name:  "azureblob-base-path",
				Value: "",
				Usage: "Azure Blob storage base path",
			},
		},
	}
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
		if user.CustomAvatarRelativePath() == "" {
			return nil
		}
		_, err := storage.Copy(dstStorage, user.CustomAvatarRelativePath(), storage.Avatars, user.CustomAvatarRelativePath())
		return err
	})
}

func migrateRepoAvatars(ctx context.Context, dstStorage storage.ObjectStorage) error {
	return db.Iterate(ctx, nil, func(ctx context.Context, repo *repo_model.Repository) error {
		if repo.CustomAvatarRelativePath() == "" {
			return nil
		}
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

func migrateActionsArtifacts(ctx context.Context, dstStorage storage.ObjectStorage) error {
	return db.Iterate(ctx, nil, func(ctx context.Context, artifact *actions_model.ActionArtifact) error {
		if artifact.Status == actions_model.ArtifactStatusExpired {
			return nil
		}

		_, err := storage.Copy(dstStorage, artifact.StoragePath, storage.ActionsArtifacts, artifact.StoragePath)
		if err != nil {
			// ignore files that do not exist
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}

		return nil
	})
}

func runMigrateStorage(ctx context.Context, cmd *cli.Command) error {
	if err := initDB(ctx); err != nil {
		return err
	}

	log.Info("AppPath: %s", setting.AppPath)
	log.Info("AppWorkPath: %s", setting.AppWorkPath)
	log.Info("Custom path: %s", setting.CustomPath)
	log.Info("Log path: %s", setting.Log.RootPath)
	log.Info("Configuration file: %s", setting.CustomConf)

	if err := db.InitEngineWithMigration(context.Background(), versioned_migration.Migrate); err != nil {
		log.Fatal("Failed to initialize ORM engine: %v", err)
		return err
	}

	if err := storage.Init(); err != nil {
		return err
	}

	var dstStorage storage.ObjectStorage
	var err error
	switch strings.ToLower(cmd.String("storage")) {
	case "":
		fallthrough
	case string(setting.LocalStorageType):
		p := cmd.String("path")
		if p == "" {
			log.Fatal("Path must be given when storage is local")
			return nil
		}
		dstStorage, err = storage.NewLocalStorage(
			ctx,
			&setting.Storage{
				Path: p,
			})
	case "minio":
		log.Warn("Deprecation: --storage=minio is deprecated, please use --storage=s3 instead because this fallback will be removed in %s", setting.MinioToS3RemovalVersion)
		fallthrough
	case string(setting.S3StorageType):
		dstStorage, err = storage.NewS3Storage(
			ctx,
			&setting.Storage{
				S3Config: setting.S3StorageConfig{
					Endpoint:           cmd.String("s3-endpoint"),
					AccessKeyID:        cmd.String("s3-access-key-id"),
					SecretAccessKey:    cmd.String("s3-secret-access-key"),
					Bucket:             cmd.String("s3-bucket"),
					Location:           cmd.String("s3-location"),
					BasePath:           cmd.String("s3-base-path"),
					UseSSL:             cmd.Bool("s3-use-ssl"),
					InsecureSkipVerify: cmd.Bool("s3-insecure-skip-verify"),
					ChecksumAlgorithm:  cmd.String("s3-checksum-algorithm"),
					BucketLookUpType:   cmd.String("s3-bucket-lookup-type"),
				},
			})
	case string(setting.AzureBlobStorageType):
		dstStorage, err = storage.NewAzureBlobStorage(
			ctx,
			&setting.Storage{
				AzureBlobConfig: setting.AzureBlobStorageConfig{
					Endpoint:    cmd.String("azureblob-endpoint"),
					AccountName: cmd.String("azureblob-account-name"),
					AccountKey:  cmd.String("azureblob-account-key"),
					Container:   cmd.String("azureblob-container"),
					BasePath:    cmd.String("azureblob-base-path"),
				},
			})
	default:
		return fmt.Errorf("unsupported storage type: %s", cmd.String("storage"))
	}
	if err != nil {
		return err
	}

	migratedMethods := map[string]func(context.Context, storage.ObjectStorage) error{
		"attachments":       migrateAttachments,
		"lfs":               migrateLFS,
		"avatars":           migrateAvatars,
		"repo-avatars":      migrateRepoAvatars,
		"repo-archivers":    migrateRepoArchivers,
		"packages":          migratePackages,
		"actions-log":       migrateActionsLog,
		"actions-artifacts": migrateActionsArtifacts,
	}

	tp := strings.ToLower(cmd.String("type"))
	if m, ok := migratedMethods[tp]; ok {
		if err := m(ctx, dstStorage); err != nil {
			return err
		}
		log.Info("%s files have successfully been copied to the new storage.", tp)
		return nil
	}

	return fmt.Errorf("unsupported storage: %s", cmd.String("type"))
}
