// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package attachment

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
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

	if err := db.WithTx(ctx, func(ctx context.Context) error {
		attach.UUID = uuid.New().String()
		size, err := storage.Attachments.Save(attach.RelativePath(), file, size)
		if err != nil {
			return fmt.Errorf("Create: %w", err)
		}
		attach.Size = size

		return db.Insert(ctx, attach)
	}); err != nil {
		return nil, err
	}

	return attach, nil
}

type ErrAttachmentSizeExceed struct {
	MaxSize int64
	Size    int64
}

func (e *ErrAttachmentSizeExceed) Error() string {
	if e.Size == 0 {
		return fmt.Sprintf("attachment size exceeds limit %d", e.MaxSize)
	}
	return fmt.Sprintf("attachment size %d exceeds limit %d", e.Size, e.MaxSize)
}

func (e *ErrAttachmentSizeExceed) Unwrap() error {
	return util.ErrContentTooLarge
}

func (e *ErrAttachmentSizeExceed) Is(target error) bool {
	_, ok := target.(*ErrAttachmentSizeExceed)
	return ok
}

// UploadAttachment upload new attachment into storage and update database
func UploadAttachment(ctx context.Context, file io.Reader, allowedTypes string, maxFileSize, fileSize int64, attach *repo_model.Attachment) (*repo_model.Attachment, error) {
	buf := make([]byte, 1024)
	n, _ := util.ReadAtMost(file, buf)
	buf = buf[:n]

	if err := upload.Verify(buf, attach.Name, allowedTypes); err != nil {
		return nil, err
	}

	reader := io.MultiReader(bytes.NewReader(buf), file)

	// enforce file size limit
	if maxFileSize >= 0 {
		if fileSize > maxFileSize {
			return nil, &ErrAttachmentSizeExceed{MaxSize: maxFileSize, Size: fileSize}
		}
		// limit reader to max file size with additional 1k more,
		// to allow side-cases where encoding tells us its exactly maxFileSize but the actual created file is bit more,
		// while still make sure the limit is enforced
		reader = attachmentLimitedReader(reader, maxFileSize+1024)
	}

	return NewAttachment(ctx, attach, reader, fileSize)
}

// UpdateAttachment updates an attachment, verifying that its name is among the allowed types.
func UpdateAttachment(ctx context.Context, allowedTypes string, attach *repo_model.Attachment) error {
	if err := upload.Verify(nil, attach.Name, allowedTypes); err != nil {
		return err
	}

	return repo_model.UpdateAttachment(ctx, attach)
}
