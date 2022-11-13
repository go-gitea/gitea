// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package attachment

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/upload"
	"code.gitea.io/gitea/modules/util"

	"github.com/google/uuid"
)

// NewAttachment creates a new attachment object, but do not verify.
func NewAttachment(attach *repo_model.Attachment, file io.Reader) (*repo_model.Attachment, error) {
	if attach.RepoID == 0 {
		return nil, fmt.Errorf("attachment %s should belong to a repository", attach.Name)
	}

	err := db.WithTx(db.DefaultContext, func(ctx context.Context) error {
		attach.UUID = uuid.New().String()
		size, err := storage.Attachments.Save(attach.RelativePath(), file, -1)
		if err != nil {
			return fmt.Errorf("Create: %w", err)
		}
		attach.Size = size

		return db.Insert(ctx, attach)
	})

	return attach, err
}

// UploadAttachment upload new attachment into storage and update database
func UploadAttachment(file io.Reader, actorID, repoID, releaseID int64, fileName, allowedTypes string) (*repo_model.Attachment, error) {
	buf := make([]byte, 1024)
	n, _ := util.ReadAtMost(file, buf)
	buf = buf[:n]

	if err := upload.Verify(buf, fileName, allowedTypes); err != nil {
		return nil, err
	}

	return NewAttachment(&repo_model.Attachment{
		RepoID:     repoID,
		UploaderID: actorID,
		ReleaseID:  releaseID,
		Name:       fileName,
	}, io.MultiReader(bytes.NewReader(buf), file))
}
