// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"os"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func deleteOrphanedAttachments(x *xorm.Engine) error {

	type Attachment struct {
		ID        int64  `xorm:"pk autoincr"`
		UUID      string `xorm:"uuid UNIQUE"`
		IssueID   int64  `xorm:"INDEX"`
		ReleaseID int64  `xorm:"INDEX"`
		CommentID int64
	}

	sess := x.NewSession()
	defer sess.Close()

	err := sess.BufferSize(setting.Database.IterateBufferSize).
		Where("`issue_id` = 0 and (`release_id` = 0 or `release_id` not in (select `id` from `release`))").Cols("uuid").
		Iterate(new(Attachment),
			func(idx int, bean interface{}) error {
				attachment := bean.(*Attachment)

				if err := os.RemoveAll(models.AttachmentLocalPath(attachment.UUID)); err != nil {
					return err
				}

				_, err := sess.ID(attachment.ID).NoAutoCondition().Delete(attachment)
				return err
			})

	if err != nil {
		return err
	}

	return sess.Commit()
}
