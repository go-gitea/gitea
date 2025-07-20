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
	"code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
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
		attach.Status = db.FileStatusNormal

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
	cnt, err := repo_model.MarkAttachmentsDeleted(ctx, attachments)
	if err != nil {
		return 0, err
	}

	AddAttachmentsToCleanQueue(ctx, attachments)

	return int(cnt), nil
}

var cleanQueue *queue.WorkerPoolQueue[int64]

func Init() error {
	cleanQueue = queue.CreateSimpleQueue(graceful.GetManager().ShutdownContext(), "attachments-clean", handler)
	if cleanQueue == nil {
		return errors.New("Unable to create attachments-clean queue")
	}
	return nil
}

// AddAttachmentsToCleanQueue adds the attachments to the clean queue for deletion.
func AddAttachmentsToCleanQueue(ctx context.Context, attachments []*repo_model.Attachment) {
	for _, a := range attachments {
		if err := cleanQueue.Push(a.ID); err != nil {
			log.Error("Failed to push attachment ID %d to clean queue: %v", a.ID, err)
			continue
		}
	}
}

func handler(attachmentIDs ...int64) []int64 {
	return cleanAttachments(graceful.GetManager().ShutdownContext(), attachmentIDs)
}

func cleanAttachments(ctx context.Context, attachmentIDs []int64) []int64 {
	var failed []int64
	for _, attachmentID := range attachmentIDs {
		attachment, exist, err := db.GetByID[repo_model.Attachment](ctx, attachmentID)
		if err != nil {
			log.Error("Failed to get attachment by ID %d: %v", attachmentID, err)
			continue
		}
		if !exist {
			continue
		}
		if attachment.Status != db.FileStatusToBeDeleted {
			log.Trace("Attachment %s is not marked for deletion, skipping", attachment.RelativePath())
			continue
		}

		if err := storage.Attachments.Delete(attachment.RelativePath()); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				log.Error("delete attachment[uuid: %s] failed: %v", attachment.UUID, err)
				failed = append(failed, attachment.ID)
				if attachment.DeleteFailedCount%3 == 0 {
					_ = system.CreateNotice(ctx, system.NoticeRepository, fmt.Sprintf("Failed to delete attachment %s (%d times): %v", attachment.RelativePath(), attachment.DeleteFailedCount+1, err))
				}
				if err := repo_model.UpdateMarkedAttachmentFailure(ctx, attachment, err); err != nil {
					log.Error("Failed to update attachment failure for ID %d: %v", attachment.ID, err)
				}
				continue
			}
		}
		if err := repo_model.DeleteMarkedAttachmentByID(ctx, attachment.ID); err != nil {
			log.Error("Failed to delete attachment by ID %d(will be tried later): %v", attachment.ID, err)
			failed = append(failed, attachment.ID)
		} else {
			log.Trace("Attachment %s deleted from database", attachment.RelativePath())
		}
	}
	return failed
}

// ScanToBeDeletedAttachments scans for attachments that are marked as to be deleted and send to
// clean queue
func ScanToBeDeletedAttachments(ctx context.Context) error {
	attachments := make([]*repo_model.Attachment, 0, 10)
	lastID := int64(0)
	for {
		if err := db.GetEngine(ctx).
			// use the status and id index to speed up the query
			Where("status = ? AND id > ?", db.FileStatusToBeDeleted, lastID).
			Asc("id").
			Limit(100).
			Find(&attachments); err != nil {
			return fmt.Errorf("scan to-be-deleted attachments: %w", err)
		}

		if len(attachments) == 0 {
			log.Trace("No more attachments to be deleted")
			break
		}
		AddAttachmentsToCleanQueue(ctx, attachments)
		lastID = attachments[len(attachments)-1].ID
	}

	return nil
}
