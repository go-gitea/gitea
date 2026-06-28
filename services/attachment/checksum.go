// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package attachment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/log"
	"gitea.dev/modules/storage"
)

// EnsureSHA256 backfills missing attachment sha256 values on demand.
func EnsureSHA256(ctx context.Context, attachments ...*repo_model.Attachment) {
	for _, attachment := range attachments {
		if attachment == nil || attachment.ID == 0 || attachment.HashSHA256 != "" {
			continue
		}
		if err := backfillSHA256(ctx, attachment); err != nil {
			log.Warn("Unable to backfill attachment sha256 for %s: %v", attachment.UUID, err)
		}
	}
}

func backfillSHA256(ctx context.Context, attachment *repo_model.Attachment) error {
	fr, err := storage.Attachments.Open(attachment.RelativePath())
	if err != nil {
		return err
	}
	defer fr.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, fr); err != nil {
		return err
	}

	attach := &repo_model.Attachment{
		ID:         attachment.ID,
		HashSHA256: hex.EncodeToString(hasher.Sum(nil)),
	}
	if _, err := db.GetEngine(ctx).ID(attachment.ID).Cols("hash_sha256").Update(attach); err != nil {
		return err
	}
	attachment.HashSHA256 = attach.HashSHA256
	return nil
}
