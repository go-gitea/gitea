// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"fmt"
	"os"
	"path"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/setting"
)

// UploadRepoFileOptions contains the uploaded repository file options
type UploadRepoFileOptions struct {
	LastCommitID string
	OldBranch    string
	NewBranch    string
	TreePath     string
	Message      string
	Files        []string // In UUID format.
}

type uploadInfo struct {
	upload        *models.Upload
	lfsMetaObject *models.LFSMetaObject
}

func cleanUpAfterFailure(infos *[]uploadInfo, t *TemporaryUploadRepository, original error) error {
	for _, info := range *infos {
		if info.lfsMetaObject == nil {
			continue
		}
		if !info.lfsMetaObject.Existing {
			if _, err := t.repo.RemoveLFSMetaObjectByOid(info.lfsMetaObject.Oid); err != nil {
				original = fmt.Errorf("%v, %v", original, err)
			}
		}
	}
	return original
}

// UploadRepoFiles uploads files to the given repository
func UploadRepoFiles(repo *models.Repository, doer *models.User, opts *UploadRepoFileOptions) error {
	if len(opts.Files) == 0 {
		return nil
	}

	uploads, err := models.GetUploadsByUUIDs(opts.Files)
	if err != nil {
		return fmt.Errorf("GetUploadsByUUIDs [uuids: %v]: %v", opts.Files, err)
	}

	names := make([]string, len(uploads))
	infos := make([]uploadInfo, len(uploads))
	for i, upload := range uploads {
		// Check file is not lfs locked, will return nil if lock setting not enabled
		filepath := path.Join(opts.TreePath, upload.Name)
		lfsLock, err := repo.GetTreePathLock(filepath)
		if err != nil {
			return err
		}
		if lfsLock != nil && lfsLock.OwnerID != doer.ID {
			return models.ErrLFSFileLocked{RepoID: repo.ID, Path: filepath, UserName: lfsLock.Owner.Name}
		}

		names[i] = upload.Name
		infos[i] = uploadInfo{upload: upload}
	}

	t, err := NewTemporaryUploadRepository(repo)
	if err != nil {
		return err
	}
	defer t.Close()
	if err := t.Clone(opts.OldBranch); err != nil {
		return err
	}
	if err := t.SetDefaultIndex(); err != nil {
		return err
	}

	var filename2attribute2info map[string]map[string]string
	if setting.LFS.StartServer {
		filename2attribute2info, err = t.CheckAttribute("filter", names...)
		if err != nil {
			return err
		}
	}

	// Copy uploaded files into repository.
	for i, uploadInfo := range infos {
		file, err := os.Open(uploadInfo.upload.LocalPath())
		if err != nil {
			return err
		}
		defer file.Close()

		var objectHash string
		if setting.LFS.StartServer && filename2attribute2info[uploadInfo.upload.Name] != nil && filename2attribute2info[uploadInfo.upload.Name]["filter"] == "lfs" {
			// Handle LFS
			// FIXME: Inefficient! this should probably happen in models.Upload
			oid, err := models.GenerateLFSOid(file)
			if err != nil {
				return err
			}
			fileInfo, err := file.Stat()
			if err != nil {
				return err
			}

			uploadInfo.lfsMetaObject = &models.LFSMetaObject{Oid: oid, Size: fileInfo.Size(), RepositoryID: t.repo.ID}

			if objectHash, err = t.HashObject(strings.NewReader(uploadInfo.lfsMetaObject.Pointer())); err != nil {
				return err
			}
			infos[i] = uploadInfo

		} else if objectHash, err = t.HashObject(file); err != nil {
			return err
		}

		// Add the object to the index
		if err := t.AddObjectToIndex("100644", objectHash, path.Join(opts.TreePath, uploadInfo.upload.Name)); err != nil {
			return err

		}
	}

	// Now write the tree
	treeHash, err := t.WriteTree()
	if err != nil {
		return err
	}

	// make author and committer the doer
	author := doer
	committer := doer

	// Now commit the tree
	commitHash, err := t.CommitTree(author, committer, treeHash, opts.Message)
	if err != nil {
		return err
	}

	// Now deal with LFS objects
	for _, uploadInfo := range infos {
		if uploadInfo.lfsMetaObject == nil {
			continue
		}
		uploadInfo.lfsMetaObject, err = models.NewLFSMetaObject(uploadInfo.lfsMetaObject)
		if err != nil {
			// OK Now we need to cleanup
			return cleanUpAfterFailure(&infos, t, err)
		}
		// Don't move the files yet - we need to ensure that
		// everything can be inserted first
	}

	// OK now we can insert the data into the store - there's no way to clean up the store
	// once it's in there, it's in there.
	contentStore := &lfs.ContentStore{BasePath: setting.LFS.ContentPath}
	for _, uploadInfo := range infos {
		if uploadInfo.lfsMetaObject == nil {
			continue
		}
		if !contentStore.Exists(uploadInfo.lfsMetaObject) {
			file, err := os.Open(uploadInfo.upload.LocalPath())
			if err != nil {
				return cleanUpAfterFailure(&infos, t, err)
			}
			defer file.Close()
			// FIXME: Put regenerates the hash and copies the file over.
			// I guess this strictly ensures the soundness of the store but this is inefficient.
			if err := contentStore.Put(uploadInfo.lfsMetaObject, file); err != nil {
				// OK Now we need to cleanup
				// Can't clean up the store, once uploaded there they're there.
				return cleanUpAfterFailure(&infos, t, err)
			}
		}
	}

	// Then push this tree to NewBranch
	if err := t.Push(doer, commitHash, opts.NewBranch); err != nil {
		return err
	}

	return models.DeleteUploads(uploads...)
}
