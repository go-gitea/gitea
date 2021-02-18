// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	lfs_module "code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

func StoreMissingLfsObjectsInRepository(repo *models.Repository, gitRepo *git.Repository, lfsAddr string) error {
	client := lfs_module.NewClient(&http.Client{})
	contentStore := lfs_module.NewContetStore()

	pointers, err := lfs_module.SearchPointerFiles(gitRepo)
	if err != nil {
		return err
	}

	for _, pointer := range pointers {
		meta := &models.LFSMetaObject{Oid: pointer.Oid, Size: pointer.Size, RepositoryID: repo.ID}
		meta, err = models.NewLFSMetaObject(meta)
		if err != nil {
			return err
		}
		if meta.Existing {
			continue
		}

		log.Trace("LFS OID[%s] not present in repository %v", pointer.Oid, repo)

		exist, err := contentStore.Exists(pointer)
		if err != nil {
			return err
		}
		if !exist {
			if setting.LFS.MaxFileSize > 0 && pointer.Size > setting.LFS.MaxFileSize {
				log.Info("LFS OID[%s] download denied because of LFS_MAX_FILE_SIZE=%d < size %d", pointer.Oid, setting.LFS.MaxFileSize, pointer.Size)
				continue
			}

			stream, err := client.Download(lfsAddr, pointer.Oid, pointer.Size)
			if err != nil {
				return err
			}
			defer stream.Close()

			if err := contentStore.Put(pointer, stream); err != nil {
				if _, err2 := repo.RemoveLFSMetaObjectByOid(meta.Oid); err2 != nil {
					return err2
				}
				return err
			}
		}
	}

	return nil
}