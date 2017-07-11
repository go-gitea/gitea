// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path"
	"time"

	api "code.gitea.io/sdk/gitea"
	"github.com/go-xorm/xorm"
	gouuid "github.com/satori/go.uuid"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// Attachment represent a attachment of issue/comment/release.
type Attachment struct {
	ID            int64  `xorm:"pk autoincr"`
	UUID          string `xorm:"uuid UNIQUE"`
	IssueID       int64  `xorm:"INDEX"`
	ReleaseID     int64  `xorm:"INDEX"`
	CommentID     int64
	Name          string
	DownloadCount int64     `xorm:"DEFAULT 0"`
	Created       time.Time `xorm:"-"`
	CreatedUnix   int64
}

// BeforeInsert is invoked from XORM before inserting an object of this type.
func (a *Attachment) BeforeInsert() {
	a.CreatedUnix = time.Now().Unix()
}

// AfterSet is invoked from XORM after setting the value of a field of
// this object.
func (a *Attachment) AfterSet(colName string, _ xorm.Cell) {
	switch colName {
	case "created_unix":
		a.Created = time.Unix(a.CreatedUnix, 0).Local()
	}
}

// IncreaseDownloadCount is update download count + 1
func (a *Attachment) IncreaseDownloadCount() error {
	sess := x.NewSession()
	defer sess.Close()

	// Update download count.
	if _, err := sess.Exec("UPDATE `attachment` SET download_count=download_count+1 WHERE id=?", a.ID); err != nil {
		return fmt.Errorf("increase attachment count: %v", err)
	}

	return nil
}

// GetSize gets the size of the attachment in bytes
func (a *Attachment) GetSize() (int64, error) {
	info, err := os.Stat(a.LocalPath())
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// AttachmentLocalPath returns where attachment is stored in local file
// system based on given UUID.
func AttachmentLocalPath(uuid string) string {
	return path.Join(setting.AttachmentPath, uuid[0:1], uuid[1:2], uuid)
}

// LocalPath returns where attachment is stored in local file system.
func (a *Attachment) LocalPath() string {
	return AttachmentLocalPath(a.UUID)
}

// APIFormat converts a Attachment to an api.Attachment
func (a *Attachment) APIFormat() *api.Attachment {
	apiAttachment := &api.Attachment{
		ID:            a.ID,
		Created:       a.Created,
		Name:          a.Name,
		UUID:          a.UUID,
		DownloadURL:   setting.AppURL + "attachments/" + a.UUID,
		DownloadCount: a.DownloadCount,
	}
	fileSize, err := a.GetSize()
	log.Warn("Error getting the file size for attachment %s. ", a.UUID, err)
	if err == nil {
		apiAttachment.Size = fileSize
	}
	return apiAttachment
}

// NewAttachment creates a new attachment object.
func NewAttachment(name string, buf []byte, file multipart.File) (_ *Attachment, err error) {
	attach := &Attachment{
		UUID: gouuid.NewV4().String(),
		Name: name,
	}

	localPath := attach.LocalPath()
	if err = os.MkdirAll(path.Dir(localPath), os.ModePerm); err != nil {
		return nil, fmt.Errorf("MkdirAll: %v", err)
	}

	fw, err := os.Create(localPath)
	if err != nil {
		return nil, fmt.Errorf("Create: %v", err)
	}
	defer fw.Close()

	if _, err = fw.Write(buf); err != nil {
		return nil, fmt.Errorf("Write: %v", err)
	} else if _, err = io.Copy(fw, file); err != nil {
		return nil, fmt.Errorf("Copy: %v", err)
	}

	if _, err := x.Insert(attach); err != nil {
		return nil, err
	}

	return attach, nil
}

func getAttachmentByUUID(e Engine, uuid string) (*Attachment, error) {
	attach := &Attachment{UUID: uuid}
	has, err := e.Get(attach)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrAttachmentNotExist{0, uuid}
	}
	return attach, nil
}

func getAttachmentsByUUIDs(e Engine, uuids []string) ([]*Attachment, error) {
	if len(uuids) == 0 {
		return []*Attachment{}, nil
	}

	// Silently drop invalid uuids.
	attachments := make([]*Attachment, 0, len(uuids))
	return attachments, e.In("uuid", uuids).Find(&attachments)
}

// GetAttachmentByUUID returns attachment by given UUID.
func GetAttachmentByUUID(uuid string) (*Attachment, error) {
	return getAttachmentByUUID(x, uuid)
}

// GetAttachmentByID returns attachment by given ID.
func GetAttachmentByID(id int64) (*Attachment, error) {
	attach := &Attachment{ID: id}
	has, err := x.Get(attach)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrAttachmentNotExist{id, ""}
	}
	return attach, nil
}

func getAttachmentsByIssueID(e Engine, issueID int64) ([]*Attachment, error) {
	attachments := make([]*Attachment, 0, 10)
	return attachments, e.Where("issue_id = ? AND comment_id = 0", issueID).Find(&attachments)
}

// GetAttachmentsByIssueID returns all attachments of an issue.
func GetAttachmentsByIssueID(issueID int64) ([]*Attachment, error) {
	return getAttachmentsByIssueID(x, issueID)
}

// GetAttachmentsByCommentID returns all attachments if comment by given ID.
func GetAttachmentsByCommentID(commentID int64) ([]*Attachment, error) {
	attachments := make([]*Attachment, 0, 10)
	return attachments, x.Where("comment_id=?", commentID).Find(&attachments)
}

// GetAttachmentsByReleaseID returns all attachments of a release
func GetAttachmentsByReleaseID(releaseID int64) ([]*Attachment, error) {
	attachments := make([]*Attachment, 0, 10)
	return attachments, x.Where("release_id=?", releaseID).Find(&attachments)
}

// DeleteAttachment deletes the given attachment and optionally the associated file.
func DeleteAttachment(a *Attachment, remove bool) error {
	_, err := DeleteAttachments([]*Attachment{a}, remove)
	return err
}

// DeleteAttachments deletes the given attachments and optionally the associated files.
func DeleteAttachments(attachments []*Attachment, remove bool) (int, error) {
	for i, a := range attachments {
		if remove {
			if err := os.Remove(a.LocalPath()); err != nil {
				return i, err
			}
		}

		if _, err := x.Delete(a); err != nil {
			return i, err
		}
	}

	return len(attachments), nil
}

// DeleteAttachmentsByIssue deletes all attachments associated with the given issue.
func DeleteAttachmentsByIssue(issueID int64, remove bool) (int, error) {
	attachments, err := GetAttachmentsByIssueID(issueID)

	if err != nil {
		return 0, err
	}

	return DeleteAttachments(attachments, remove)
}

// DeleteAttachmentsByComment deletes all attachments associated with the given comment.
func DeleteAttachmentsByComment(commentID int64, remove bool) (int, error) {
	attachments, err := GetAttachmentsByCommentID(commentID)

	if err != nil {
		return 0, err
	}

	return DeleteAttachments(attachments, remove)
}
