// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"fmt"
	"path"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

// Attachment represent a attachment of issue/comment/release.
type Attachment struct {
	ID            int64  `xorm:"pk autoincr"`
	UUID          string `xorm:"uuid UNIQUE"`
	RepoID        int64  `xorm:"INDEX"`           // this should not be zero
	IssueID       int64  `xorm:"INDEX"`           // maybe zero when creating
	ReleaseID     int64  `xorm:"INDEX"`           // maybe zero when creating
	UploaderID    int64  `xorm:"INDEX DEFAULT 0"` // Notice: will be zero before this column added
	CommentID     int64
	Name          string
	DownloadCount int64              `xorm:"DEFAULT 0"`
	Size          int64              `xorm:"DEFAULT 0"`
	CreatedUnix   timeutil.TimeStamp `xorm:"created"`
}

func init() {
	db.RegisterModel(new(Attachment))
}

// IncreaseDownloadCount is update download count + 1
func (a *Attachment) IncreaseDownloadCount() error {
	// Update download count.
	if _, err := db.GetEngine(db.DefaultContext).Exec("UPDATE `attachment` SET download_count=download_count+1 WHERE id=?", a.ID); err != nil {
		return fmt.Errorf("increase attachment count: %v", err)
	}

	return nil
}

// AttachmentRelativePath returns the relative path
func AttachmentRelativePath(uuid string) string {
	return path.Join(uuid[0:1], uuid[1:2], uuid)
}

// RelativePath returns the relative path of the attachment
func (a *Attachment) RelativePath() string {
	return AttachmentRelativePath(a.UUID)
}

// DownloadURL returns the download url of the attached file
func (a *Attachment) DownloadURL() string {
	return fmt.Sprintf("%sattachments/%s", setting.AppURL, a.UUID)
}

// LinkedRepository returns the linked repo if any
func (a *Attachment) LinkedRepository() (*Repository, unit.Type, error) {
	if a.IssueID != 0 {
		iss, err := GetIssueByID(a.IssueID)
		if err != nil {
			return nil, unit.TypeIssues, err
		}
		repo, err := GetRepositoryByID(iss.RepoID)
		unitType := unit.TypeIssues
		if iss.IsPull {
			unitType = unit.TypePullRequests
		}
		return repo, unitType, err
	} else if a.ReleaseID != 0 {
		rel, err := GetReleaseByID(a.ReleaseID)
		if err != nil {
			return nil, unit.TypeReleases, err
		}
		repo, err := GetRepositoryByID(rel.RepoID)
		return repo, unit.TypeReleases, err
	}
	return nil, -1, nil
}

// GetAttachmentByID returns attachment by given id
func GetAttachmentByID(id int64) (*Attachment, error) {
	return getAttachmentByID(db.GetEngine(db.DefaultContext), id)
}

func getAttachmentByID(e db.Engine, id int64) (*Attachment, error) {
	attach := &Attachment{}
	if has, err := e.ID(id).Get(attach); err != nil {
		return nil, err
	} else if !has {
		return nil, ErrAttachmentNotExist{ID: id, UUID: ""}
	}
	return attach, nil
}

func getAttachmentByUUID(e db.Engine, uuid string) (*Attachment, error) {
	attach := &Attachment{}
	has, err := e.Where("uuid=?", uuid).Get(attach)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrAttachmentNotExist{0, uuid}
	}
	return attach, nil
}

// GetAttachmentsByUUIDs returns attachment by given UUID list.
func GetAttachmentsByUUIDs(ctx context.Context, uuids []string) ([]*Attachment, error) {
	return getAttachmentsByUUIDs(db.GetEngine(ctx), uuids)
}

func getAttachmentsByUUIDs(e db.Engine, uuids []string) ([]*Attachment, error) {
	if len(uuids) == 0 {
		return []*Attachment{}, nil
	}

	// Silently drop invalid uuids.
	attachments := make([]*Attachment, 0, len(uuids))
	return attachments, e.In("uuid", uuids).Find(&attachments)
}

// GetAttachmentByUUID returns attachment by given UUID.
func GetAttachmentByUUID(uuid string) (*Attachment, error) {
	return getAttachmentByUUID(db.GetEngine(db.DefaultContext), uuid)
}

// ExistAttachmentsByUUID returns true if attachment is exist by given UUID
func ExistAttachmentsByUUID(uuid string) (bool, error) {
	return db.GetEngine(db.DefaultContext).Where("`uuid`=?", uuid).Exist(new(Attachment))
}

// GetAttachmentByReleaseIDFileName returns attachment by given releaseId and fileName.
func GetAttachmentByReleaseIDFileName(releaseID int64, fileName string) (*Attachment, error) {
	return getAttachmentByReleaseIDFileName(db.GetEngine(db.DefaultContext), releaseID, fileName)
}

func getAttachmentsByIssueID(e db.Engine, issueID int64) ([]*Attachment, error) {
	attachments := make([]*Attachment, 0, 10)
	return attachments, e.Where("issue_id = ? AND comment_id = 0", issueID).Find(&attachments)
}

// GetAttachmentsByIssueID returns all attachments of an issue.
func GetAttachmentsByIssueID(issueID int64) ([]*Attachment, error) {
	return getAttachmentsByIssueID(db.GetEngine(db.DefaultContext), issueID)
}

// GetAttachmentsByCommentID returns all attachments if comment by given ID.
func GetAttachmentsByCommentID(commentID int64) ([]*Attachment, error) {
	return getAttachmentsByCommentID(db.GetEngine(db.DefaultContext), commentID)
}

func getAttachmentsByCommentID(e db.Engine, commentID int64) ([]*Attachment, error) {
	attachments := make([]*Attachment, 0, 10)
	return attachments, e.Where("comment_id=?", commentID).Find(&attachments)
}

// getAttachmentByReleaseIDFileName return a file based on the the following infos:
func getAttachmentByReleaseIDFileName(e db.Engine, releaseID int64, fileName string) (*Attachment, error) {
	attach := &Attachment{ReleaseID: releaseID, Name: fileName}
	has, err := e.Get(attach)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, err
	}
	return attach, nil
}

// DeleteAttachment deletes the given attachment and optionally the associated file.
func DeleteAttachment(a *Attachment, remove bool) error {
	_, err := DeleteAttachments(db.DefaultContext, []*Attachment{a}, remove)
	return err
}

// DeleteAttachments deletes the given attachments and optionally the associated files.
func DeleteAttachments(ctx context.Context, attachments []*Attachment, remove bool) (int, error) {
	if len(attachments) == 0 {
		return 0, nil
	}

	ids := make([]int64, 0, len(attachments))
	for _, a := range attachments {
		ids = append(ids, a.ID)
	}

	cnt, err := db.GetEngine(ctx).In("id", ids).NoAutoCondition().Delete(attachments[0])
	if err != nil {
		return 0, err
	}

	if remove {
		for i, a := range attachments {
			if err := storage.Attachments.Delete(a.RelativePath()); err != nil {
				return i, err
			}
		}
	}
	return int(cnt), nil
}

// DeleteAttachmentsByIssue deletes all attachments associated with the given issue.
func DeleteAttachmentsByIssue(issueID int64, remove bool) (int, error) {
	attachments, err := GetAttachmentsByIssueID(issueID)
	if err != nil {
		return 0, err
	}

	return DeleteAttachments(db.DefaultContext, attachments, remove)
}

// DeleteAttachmentsByComment deletes all attachments associated with the given comment.
func DeleteAttachmentsByComment(commentID int64, remove bool) (int, error) {
	attachments, err := GetAttachmentsByCommentID(commentID)
	if err != nil {
		return 0, err
	}

	return DeleteAttachments(db.DefaultContext, attachments, remove)
}

// UpdateAttachment updates the given attachment in database
func UpdateAttachment(atta *Attachment) error {
	return updateAttachment(db.GetEngine(db.DefaultContext), atta)
}

// UpdateAttachmentByUUID Updates attachment via uuid
func UpdateAttachmentByUUID(ctx context.Context, attach *Attachment, cols ...string) error {
	if attach.UUID == "" {
		return fmt.Errorf("attachment uuid should be not blank")
	}
	_, err := db.GetEngine(ctx).Where("uuid=?", attach.UUID).Cols(cols...).Update(attach)
	return err
}

func updateAttachment(e db.Engine, atta *Attachment) error {
	var sess *xorm.Session
	if atta.ID != 0 && atta.UUID == "" {
		sess = e.ID(atta.ID)
	} else {
		// Use uuid only if id is not set and uuid is set
		sess = e.Where("uuid = ?", atta.UUID)
	}
	_, err := sess.Cols("name", "issue_id", "release_id", "comment_id", "download_count").Update(atta)
	return err
}

// DeleteAttachmentsByRelease deletes all attachments associated with the given release.
func DeleteAttachmentsByRelease(releaseID int64) error {
	_, err := db.GetEngine(db.DefaultContext).Where("release_id = ?", releaseID).Delete(&Attachment{})
	return err
}

// IterateAttachment iterates attachments; it should not be used when Gitea is servicing users.
func IterateAttachment(f func(attach *Attachment) error) error {
	var start int
	const batchSize = 100
	for {
		attachments := make([]*Attachment, 0, batchSize)
		if err := db.GetEngine(db.DefaultContext).Limit(batchSize, start).Find(&attachments); err != nil {
			return err
		}
		if len(attachments) == 0 {
			return nil
		}
		start += len(attachments)

		for _, attach := range attachments {
			if err := f(attach); err != nil {
				return err
			}
		}
	}
}

// CountOrphanedAttachments returns the number of bad attachments
func CountOrphanedAttachments() (int64, error) {
	return db.GetEngine(db.DefaultContext).Where("(issue_id > 0 and issue_id not in (select id from issue)) or (release_id > 0 and release_id not in (select id from `release`))").
		Count(new(Attachment))
}

// DeleteOrphanedAttachments delete all bad attachments
func DeleteOrphanedAttachments() error {
	_, err := db.GetEngine(db.DefaultContext).Where("(issue_id > 0 and issue_id not in (select id from issue)) or (release_id > 0 and release_id not in (select id from `release`))").
		Delete(new(Attachment))
	return err
}
