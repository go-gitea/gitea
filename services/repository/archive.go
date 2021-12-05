// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"context"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
)

func deleteOldRepoArchiver(ctx context.Context, archiver *repo_model.RepoArchiver) error {
	p, err := archiver.RelativePath()
	if err != nil {
		return err
	}
	if err := repo_model.DeleteRepoArchiver(ctx, archiver); err != nil {
		return err
	}
	if err := storage.RepoArchives.Delete(p); err != nil {
		log.Error("delete repo archive file failed: %v", err)
	}
	return nil
}

// DeleteOldRepositoryArchives deletes old repository archives.
func DeleteOldRepositoryArchives(ctx context.Context, olderThan time.Duration) error {
	log.Trace("Doing: ArchiveCleanup")

	for {
		archivers, err := repo_model.FindRepoArchives(repo_model.FindRepoArchiversOption{
			ListOptions: db.ListOptions{
				PageSize: 100,
				Page:     1,
			},
			OlderThan: olderThan,
		})
		if err != nil {
			log.Trace("Error: ArchiveClean: %v", err)
			return err
		}

		for _, archiver := range archivers {
			if err := deleteOldRepoArchiver(ctx, archiver); err != nil {
				return err
			}
		}
		if len(archivers) < 100 {
			break
		}
	}

	log.Trace("Finished: ArchiveCleanup")
	return nil
}

// DeleteRepositoryArchives deletes all repositories' archives.
func DeleteRepositoryArchives(ctx context.Context) error {
	if err := repo_model.DeleteAllRepoArchives(); err != nil {
		return err
	}
	return storage.Clean(storage.RepoArchives)
}
