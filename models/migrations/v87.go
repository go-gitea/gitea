package migrations

import (
	"os"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"

	"github.com/go-xorm/xorm"
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

	err := sess.BufferSize(setting.IterateBufferSize).
		Where("comment_id = ? AND release_id = ?", 0, 0).Cols("uuid").
		Iterate(new(Attachment),
			func(idx int, bean interface{}) error {
				attachment := bean.(*Attachment)

				if err := os.RemoveAll(models.AttachmentLocalPath(attachment.UUID)); err != nil {
					return err
				}

				_, err := sess.Delete(attachment)
				return err
			})

	if err != nil {
		return err
	}

	return sess.Commit()
}
