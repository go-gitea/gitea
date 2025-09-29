// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"context"
	"fmt"
	"os"
	"path"
	"sync"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
)

// UploadRepoFileOptions contains the uploaded repository file options
type UploadRepoFileOptions struct {
	LastCommitID string
	OldBranch    string
	NewBranch    string
	TreePath     string
	Message      string
	Files        []string // In UUID format.
	Signoff      bool
	Author       *IdentityOptions
	Committer    *IdentityOptions
}

type lazyLocalFileReader struct {
	*os.File
	localFilename string
	counter       int
	mu            sync.Mutex
}

var _ LazyReadSeeker = (*lazyLocalFileReader)(nil)

func (l *lazyLocalFileReader) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.counter > 0 {
		l.counter--
		if l.counter == 0 {
			if err := l.File.Close(); err != nil {
				return fmt.Errorf("close file %s: %w", l.localFilename, err)
			}
			l.File = nil
		}
		return nil
	}
	return fmt.Errorf("file %s already closed", l.localFilename)
}

func (l *lazyLocalFileReader) OpenLazyReader() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.File != nil {
		l.counter++
		return nil
	}

	file, err := os.Open(l.localFilename)
	if err != nil {
		return err
	}
	l.File = file
	l.counter = 1
	return nil
}

// UploadRepoFiles uploads files to the given repository
func UploadRepoFiles(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, opts *UploadRepoFileOptions) error {
	if len(opts.Files) == 0 {
		return nil
	}

	uploads, err := repo_model.GetUploadsByUUIDs(ctx, opts.Files)
	if err != nil {
		return fmt.Errorf("GetUploadsByUUIDs [uuids: %v]: %w", opts.Files, err)
	}

	changeOpts := &ChangeRepoFilesOptions{
		LastCommitID: opts.LastCommitID,
		OldBranch:    opts.OldBranch,
		NewBranch:    opts.NewBranch,
		Message:      opts.Message,
		Signoff:      opts.Signoff,
		Author:       opts.Author,
		Committer:    opts.Committer,
	}
	for _, upload := range uploads {
		changeOpts.Files = append(changeOpts.Files, &ChangeRepoFile{
			Operation:     "upload",
			TreePath:      path.Join(opts.TreePath, upload.Name),
			ContentReader: &lazyLocalFileReader{localFilename: upload.LocalPath()},
		})
	}

	_, err = ChangeRepoFiles(ctx, repo, doer, changeOpts)
	if err != nil {
		return err
	}
	if err := repo_model.DeleteUploads(ctx, uploads...); err != nil {
		log.Error("DeleteUploads: %v", err)
	}
	return nil
}
