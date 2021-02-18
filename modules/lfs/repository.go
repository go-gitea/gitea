// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/storage"
)

func StoreMissingLfsObjectsInRepository(client *Client, repository *context.Repository) error {
	contentStore := &ContentStore{ObjectStorage: storage.LFS}

	pointers, err := SearchPointerFiles(repository.GitRepo)
	if err != nil {
		return err
	}

	for _, pointer := range pointers {
		_, err := repository.Repository.GetLFSMetaObjectByOid(pointer.Oid)
		if err != models.ErrLFSObjectNotExist {
			continue
		}

		// TODO What to do if file is too big / quota

		meta := &models.LFSMetaObject{Oid: pointer.Oid, Size: pointer.Size, RepositoryID: repository.Repository.ID}

		meta, err = models.NewLFSMetaObject(meta)

		exist, err := contentStore.Exists(meta)
		if err != nil {
			return err
		}
		if !exist {
			lfsBaseUrl := "" // TODO
			stream, err := client.Download(lfsBaseUrl, pointer.Oid, pointer.Size)
			if err != nil {
				return err
			}
			defer stream.Close()

			if err := contentStore.Put(meta, stream); err != nil {
				if _, err2 := repository.Repository.RemoveLFSMetaObjectByOid(meta.Oid); err2 != nil {
					return err2
				}
				return err
			}
		}
	}

	return nil
}