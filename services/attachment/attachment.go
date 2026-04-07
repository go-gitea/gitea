// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package attachment

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context/upload"

	"github.com/google/uuid"
)

// NewAttachmentOrReleaseAttachment creates a new attachment object, but do not verify.
func NewAttachmentOrReleaseAttachment(ctx context.Context, attach *repo_model.Attachment, file io.Reader, size int64, isRelease bool) (*repo_model.Attachment, error) {
	if attach.RepoID == 0 {
		return nil, fmt.Errorf("attachment %s should belong to a repository", attach.Name)
	}

	err := db.WithTx(ctx, func(ctx context.Context) error {
		var storageChoice storage.ObjectStorage

		if isRelease {
			storageChoice = storage.ReleaseAttachments
		} else {
			storageChoice = storage.Attachments
		}

		attach.UUID = uuid.New().String()
		size, err := storageChoice.Save(attach.RelativePath(), file, size)
		if err != nil {
			return fmt.Errorf("Attachments.Save: %w", err)
		}
		attach.Size = size
		return db.Insert(ctx, attach)
	})

	return attach, err
}

type UploaderFile struct {
	rd         io.ReadCloser
	size       int64
	respWriter http.ResponseWriter
}

func NewLimitedUploaderKnownSize(r io.Reader, size int64) *UploaderFile {
	return &UploaderFile{rd: io.NopCloser(r), size: size}
}

func NewLimitedUploaderMaxBytesReader(r io.ReadCloser, w http.ResponseWriter) *UploaderFile {
	return &UploaderFile{rd: r, size: -1, respWriter: w}
}

func UploadAttachmentOrReleaseAttachmentSizeLimit(ctx context.Context, file *UploaderFile, attach *repo_model.Attachment, isRelease bool) (*repo_model.Attachment, error) {
	if isRelease {
		return uploadAttachmentOrReleaseAttachment(ctx, file, setting.Repository.Release.AllowedTypes, setting.Repository.Release.FileMaxSize<<20, attach, isRelease)
	}

	return uploadAttachmentOrReleaseAttachment(ctx, file, setting.Attachment.AllowedTypes, setting.Attachment.MaxSize<<20, attach, isRelease)
}

func uploadAttachmentOrReleaseAttachment(ctx context.Context, file *UploaderFile, allowedTypes string, maxFileSize int64, attach *repo_model.Attachment, isRelease bool) (*repo_model.Attachment, error) {
	src := file.rd
	if file.size < 0 {
		src = http.MaxBytesReader(file.respWriter, src, maxFileSize)
	}
	buf := make([]byte, 1024)
	n, _ := util.ReadAtMost(src, buf)
	buf = buf[:n]

	if err := upload.Verify(buf, attach.Name, allowedTypes); err != nil {
		return nil, err
	}

	if maxFileSize >= 0 && file.size > maxFileSize {
		return nil, util.ErrorWrap(util.ErrContentTooLarge, "attachment exceeds limit %d", maxFileSize)
	}

	attach, err := NewAttachmentOrReleaseAttachment(ctx, attach, io.MultiReader(bytes.NewReader(buf), src), file.size, isRelease)
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		return nil, util.ErrorWrap(util.ErrContentTooLarge, "attachment exceeds limit %d", maxFileSize)
	}
	return attach, err
}

// UpdateAttachment updates an attachment, verifying that its name is among the allowed types.
func UpdateAttachment(ctx context.Context, allowedTypes string, attach *repo_model.Attachment) error {
	if err := upload.Verify(nil, attach.Name, allowedTypes); err != nil {
		return err
	}

	return repo_model.UpdateAttachment(ctx, attach)
}
