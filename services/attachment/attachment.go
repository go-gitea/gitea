// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package attachment

import (
	"bytes"
	"fmt"
	"io"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/storage"

	"github.com/google/uuid"
)

// NewAttachment creates a new attachment object.
func NewAttachment(attach *models.Attachment, buf []byte, file io.Reader) (*models.Attachment, error) {
	if attach.RepoID == 0 {
		return nil, fmt.Errorf("attachment %s should belong to a repository", attach.Name)
	}

	err := models.WithTx(func(ctx models.DBContext) error {
		attach.UUID = uuid.New().String()
		size, err := storage.Attachments.Save(attach.RelativePath(), io.MultiReader(bytes.NewReader(buf), file), -1)
		if err != nil {
			return fmt.Errorf("Create: %v", err)
		}
		attach.Size = size

		return models.Insert(ctx, attach)
	})

	return attach, err
}
