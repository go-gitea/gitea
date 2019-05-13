// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package uploader

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

func detectEncodingAndBOM(entry *git.TreeEntry, repo *models.Repository) (string, bool) {
	reader, err := entry.Blob().DataAsync()
	if err != nil {
		// return default
		return "UTF-8", false
	}
	defer reader.Close()
	buf := make([]byte, 1024)
	n, err := reader.Read(buf)
	if err != nil {
		// return default
		return "UTF-8", false
	}
	buf = buf[:n]

	if setting.LFS.StartServer {
		meta := lfs.IsPointerFile(&buf)
		if meta != nil {
			meta, err = repo.GetLFSMetaObjectByOid(meta.Oid)
			if err != nil && err != models.ErrLFSObjectNotExist {
				// return default
				return "UTF-8", false
			}
		}
		if meta != nil {
			dataRc, err := lfs.ReadMetaObject(meta)
			if err != nil {
				// return default
				return "UTF-8", false
			}
			defer dataRc.Close()
			buf = make([]byte, 1024)
			n, err = dataRc.Read(buf)
			if err != nil {
				// return default
				return "UTF-8", false
			}
			buf = buf[:n]
		}

	}

	encoding, err := base.DetectEncoding(buf)
	if err != nil {
		// just default to utf-8 and no bom
		return "UTF-8", false
	}
	if encoding == "UTF-8" {
		return encoding, bytes.Equal(buf[0:3], base.UTF8BOM)
	}
	charsetEncoding, _ := charset.Lookup(encoding)
	if charsetEncoding == nil {
		return "UTF-8", false
	}

	result, n, err := transform.String(charsetEncoding.NewDecoder(), string(buf))

	if n > 2 {
		return encoding, bytes.Equal([]byte(result)[0:3], base.UTF8BOM)
	}

	return encoding, false
}

// UpdateRepoFileOptions holds the repository file update options
type UpdateRepoFileOptions struct {
	LastCommitID string
	OldBranch    string
	NewBranch    string
	OldTreeName  string
	NewTreeName  string
	Message      string
	Content      string
	IsNewFile    bool
}

// UpdateRepoFile adds or updates a file in the given repository
func UpdateRepoFile(repo *models.Repository, doer *models.User, opts *UpdateRepoFileOptions) error {
	t, err := NewTemporaryUploadRepository(repo)
	defer t.Close()
	if err != nil {
		return err
	}
	if err := t.Clone(opts.OldBranch); err != nil {
		return err
	}
	if err := t.SetDefaultIndex(); err != nil {
		return err
	}

	filesInIndex, err := t.LsFiles(opts.NewTreeName, opts.OldTreeName)
	if err != nil {
		return fmt.Errorf("UpdateRepoFile: %v", err)
	}

	encoding := "UTF-8"
	bom := false

	if opts.IsNewFile {
		for _, file := range filesInIndex {
			if file == opts.NewTreeName {
				return models.ErrRepoFileAlreadyExist{FileName: opts.NewTreeName}
			}
		}
	} else {
		gitRepo, err := git.OpenRepository(t.basePath)
		if err != nil {
			return err
		}
		tree, err := gitRepo.GetTree("HEAD")
		if err != nil {
			return err
		}
		entry, err := tree.GetTreeEntryByPath(opts.OldTreeName)
		if err != nil {
			return err
		}
		encoding, bom = detectEncodingAndBOM(entry, repo)
	}

	//var stdout string
	if opts.OldTreeName != opts.NewTreeName && len(filesInIndex) > 0 {
		for _, file := range filesInIndex {
			if file == opts.OldTreeName {
				if err := t.RemoveFilesFromIndex(opts.OldTreeName); err != nil {
					return err
				}
			}
		}

	}

	// Check there is no way this can return multiple infos
	filename2attribute2info, err := t.CheckAttribute("filter", opts.NewTreeName)
	if err != nil {
		return err
	}

	content := opts.Content
	if bom {
		content = string(base.UTF8BOM) + content
	}
	if encoding != "UTF-8" {
		charsetEncoding, _ := charset.Lookup(encoding)
		if charsetEncoding != nil {
			result, _, err := transform.String(charsetEncoding.NewEncoder(), string(content))
			if err != nil {
				// Look if we can't encode back in to the original we should just stick with utf-8
				log.Error(4, "Error re-encoding %s (%s) as %s - will stay as UTF-8: %v", opts.NewTreeName, opts.OldTreeName, encoding, err)
				result = content
			}
			content = result
		} else {
			log.Error(4, "Unknown encoding: %s", encoding)
		}
	}
	// Reset the opts.Content with the re-encoded and BOM'd content
	opts.Content = content
	var lfsMetaObject *models.LFSMetaObject

	if setting.LFS.StartServer && filename2attribute2info[opts.NewTreeName] != nil && filename2attribute2info[opts.NewTreeName]["filter"] == "lfs" {
		// OK so we are supposed to LFS this data!
		oid, err := models.GenerateLFSOid(strings.NewReader(opts.Content))
		if err != nil {
			return err
		}
		lfsMetaObject = &models.LFSMetaObject{Oid: oid, Size: int64(len(opts.Content)), RepositoryID: repo.ID}
		content = lfsMetaObject.Pointer()
	}

	// Add the object to the database
	objectHash, err := t.HashObject(strings.NewReader(content))
	if err != nil {
		return err
	}

	// Add the object to the index
	if err := t.AddObjectToIndex("100644", objectHash, opts.NewTreeName); err != nil {
		return err
	}

	// Now write the tree
	treeHash, err := t.WriteTree()
	if err != nil {
		return err
	}

	// Now commit the tree
	commitHash, err := t.CommitTree(doer, treeHash, opts.Message)
	if err != nil {
		return err
	}

	if lfsMetaObject != nil {
		// We have an LFS object - create it
		lfsMetaObject, err = models.NewLFSMetaObject(lfsMetaObject)
		if err != nil {
			return err
		}
		contentStore := &lfs.ContentStore{BasePath: setting.LFS.ContentPath}
		if !contentStore.Exists(lfsMetaObject) {
			if err := contentStore.Put(lfsMetaObject, strings.NewReader(opts.Content)); err != nil {
				if err2 := repo.RemoveLFSMetaObjectByOid(lfsMetaObject.Oid); err2 != nil {
					return fmt.Errorf("Error whilst removing failed inserted LFS object %s: %v (Prev Error: %v)", lfsMetaObject.Oid, err2, err)
				}
				return err
			}
		}
	}

	// Then push this tree to NewBranch
	if err := t.Push(doer, commitHash, opts.NewBranch); err != nil {
		return err
	}

	// Simulate push event.
	oldCommitID := opts.LastCommitID
	if opts.NewBranch != opts.OldBranch {
		oldCommitID = git.EmptySHA
	}

	if err = repo.GetOwner(); err != nil {
		return fmt.Errorf("GetOwner: %v", err)
	}
	err = models.PushUpdate(
		opts.NewBranch,
		models.PushUpdateOptions{
			PusherID:     doer.ID,
			PusherName:   doer.Name,
			RepoUserName: repo.Owner.Name,
			RepoName:     repo.Name,
			RefFullName:  git.BranchPrefix + opts.NewBranch,
			OldCommitID:  oldCommitID,
			NewCommitID:  commitHash,
		},
	)
	if err != nil {
		return fmt.Errorf("PushUpdate: %v", err)
	}
	models.UpdateRepoIndexer(repo)

	return nil
}
