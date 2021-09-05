// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

// IssueContentHistory save issue/comment content history revisions.
type IssueContentHistory struct {
	ID             int64              `xorm:"pk autoincr"`
	PosterID       int64              `xorm:""`
	IssueID        int64              `xorm:"INDEX"`
	CommentID      int64              `xorm:"INDEX"`
	EditedUnix     timeutil.TimeStamp `xorm:"INDEX"`
	ContentText    string             `xorm:"LONGTEXT"`
	IsFirstCreated bool               `xorm:""`
	IsDeleted      bool               `xorm:""`
}

// SaveIssueContentHistory save history
func SaveIssueContentHistory(posterID, issueID, commentID int64, editTime timeutil.TimeStamp, contentText string, isFirstCreated bool) {
	ch := &IssueContentHistory{
		PosterID:       posterID,
		IssueID:        issueID,
		CommentID:      commentID,
		ContentText:    contentText,
		EditedUnix:     editTime,
		IsFirstCreated: isFirstCreated,
	}
	_, err := x.Insert(ch)
	if err != nil {
		log.Error("can not save issue content history. err=%v", err)
	}
}

// QueryIssueContentHistoryEditedCountMap query related history count of each comment (comment_id = 0 means the main issue)
// only return the count map for "edited" (history revision count > 1) issues or comments.
func QueryIssueContentHistoryEditedCountMap(issueID int64) map[int64]int {
	type HistoryCountRecord struct {
		CommentID    int64
		HistoryCount int
	}
	records := make([]*HistoryCountRecord, 0)

	err := x.GroupBy("comment_id").
		Select("comment_id, COUNT(1) as history_count").
		Table("issue_content_history").
		Where(builder.Eq{"issue_id": issueID}).
		Having("history_count > 1").
		Find(&records)
	if err != nil {
		log.Error("can not query issue content history count map. err=%v", err)
	}

	res := map[int64]int{}
	for _, r := range records {
		res[r.CommentID] = r.HistoryCount
	}
	return res
}

// IssueContentListItem the list for web ui
type IssueContentListItem struct {
	UserID          int64
	UserName        string
	UserAvatar      string
	UserAvatarEmail string
	UseCustomAvatar bool
	UserAvatarLink  string

	HistoryID      int64
	EditedUnix     timeutil.TimeStamp
	IsFirstCreated bool
	IsDeleted      bool
}

// FetchIssueContentHistoryList fetch list
func FetchIssueContentHistoryList(issueID int64, commentID int64) []*IssueContentListItem {
	res := make([]*IssueContentListItem, 0)
	err := x.Select("u.id as user_id, u.name as user_name,"+
		"u.avatar as user_avatar, u.avatar_email as user_avatar_email, u.use_custom_avatar,"+
		"h.id as history_id, h.edited_unix, h.is_first_created, h.is_deleted").
		Table([]string{"issue_content_history", "h"}).
		Join("INNER", []string{"user", "u"}, "h.poster_id = u.id").
		Where(builder.Eq{"issue_id": issueID, "comment_id": commentID}).
		OrderBy("edited_unix DESC").
		Find(&res)

	if err != nil {
		log.Error("can not fetch issue content history list. err=%v", err)
	}

	for _, item := range res {
		u := &User{
			ID:              item.UserID,
			Name:            item.UserName,
			Avatar:          item.UserAvatar,
			AvatarEmail:     item.UserAvatarEmail,
			UseCustomAvatar: item.UseCustomAvatar,
		}
		item.UserAvatarLink = u.AvatarLink()
	}
	return res
}

//SoftDeleteIssueContentHistory soft delete
func SoftDeleteIssueContentHistory(historyID int64) {
	if _, err := x.ID(historyID).Cols("is_deleted", "content_text").Update(&IssueContentHistory{
		IsDeleted:   true,
		ContentText: "",
	}); err != nil {
		log.Error("failed to soft delete issue content history. err=%v", err)
	}
}

// ErrIssueContentHistoryNotExist not exist error
type ErrIssueContentHistoryNotExist struct {
	ID int64
}

// Error error string
func (err ErrIssueContentHistoryNotExist) Error() string {
	return fmt.Sprintf("issue content history does not exist [id: %d]", err.ID)
}

// GetIssueContentHistoryByID get issue content history
func GetIssueContentHistoryByID(id int64) (*IssueContentHistory, error) {
	h := &IssueContentHistory{}
	has, err := x.ID(id).Get(h)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrIssueContentHistoryNotExist{id}
	}
	return h, nil
}

// GetIssueContentHistoryAndPrev get a history and the previous non-deleted history (to compare)
func GetIssueContentHistoryAndPrev(id int64) (history, prevHistory *IssueContentHistory) {
	history = &IssueContentHistory{}
	has, err := x.ID(id).Get(history)
	if err != nil {
		log.Error("failed to get issue content history %v. err=%v", id, err)
		return nil, nil
	} else if !has {
		log.Error("issue content history does not exist. id=%v. err=%v", id, err)
		return nil, nil
	}

	prevHistory = &IssueContentHistory{}
	has, err = x.Where(builder.Eq{"issue_id": history.IssueID, "comment_id": history.CommentID, "is_deleted": false}).
		And(builder.Lt{"edited_unix": history.EditedUnix}).
		OrderBy("edited_unix DESC").Limit(1).
		Get(prevHistory)

	if err != nil {
		log.Error("failed to get issue content history %v. err=%v", id, err)
		return nil, nil
	} else if !has {
		return history, nil
	}

	return history, prevHistory
}
