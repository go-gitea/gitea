// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package attachment

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context/upload"

	"github.com/google/uuid"
)

// NewAttachment creates a new attachment object, but do not verify.
func NewAttachment(ctx context.Context, attach *repo_model.Attachment, file io.Reader, size int64) (*repo_model.Attachment, error) {
	if attach.RepoID == 0 {
		return nil, fmt.Errorf("attachment %s should belong to a repository", attach.Name)
	}

	err := db.WithTx(ctx, func(ctx context.Context) error {
		attach.UUID = uuid.New().String()
		size, err := storage.Attachments.Save(attach.RelativePath(), file, size)
		if err != nil {
			return fmt.Errorf("Create: %w", err)
		}
		attach.Size = size

		return db.Insert(ctx, attach)
	})

	return attach, err
}

// UploadAttachment upload new attachment into storage and update database
func UploadAttachment(ctx context.Context, file io.Reader, allowedTypes string, fileSize int64, attach *repo_model.Attachment) (*repo_model.Attachment, error) {
	buf := make([]byte, 1024)
	n, _ := util.ReadAtMost(file, buf)
	buf = buf[:n]

	if err := upload.Verify(buf, attach.Name, allowedTypes); err != nil {
		return nil, err
	}

	return NewAttachment(ctx, attach, io.MultiReader(bytes.NewReader(buf), file), fileSize)
}

// UpdateAttachment updates an attachment, verifying that its name is among the allowed types.
func UpdateAttachment(ctx context.Context, allowedTypes string, attach *repo_model.Attachment) error {
	if err := upload.Verify(nil, attach.Name, allowedTypes); err != nil {
		return err
	}

	return repo_model.UpdateAttachment(ctx, attach)
}

// DeleteAttachment deletes the given attachment and optionally the associated file.
func DeleteAttachment(ctx context.Context, a *repo_model.Attachment) error {
	_, err := DeleteAttachments(ctx, []*repo_model.Attachment{a})
	return err
}

// DeleteAttachments deletes the given attachments and optionally the associated files.
func DeleteAttachments(ctx context.Context, attachments []*repo_model.Attachment) (int, error) {
	cnt, err := repo_model.DeleteAttachments(ctx, attachments)
	if err != nil {
		return 0, err
	}

	for _, a := range attachments {
		if err := storage.Attachments.Delete(a.RelativePath()); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				// Even delete files failed, but the attachments has been removed from database, so we
				// should not return error but only record the error on logs.
				// users have to delete this attachments manually or we should have a
				// synchronize between database attachment table and attachment storage
				log.Error("delete attachment[uuid: %s] failed: %v", a.UUID, err)
			} else {
				log.Warn("Attachment file not found when deleting: %s", a.RelativePath())
			}
		}
	}
	return int(cnt), nil
}
