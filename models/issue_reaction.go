// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"bytes"
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

// Reaction represents a reactions on issues and comments.
type Reaction struct {
	ID               int64              `xorm:"pk autoincr"`
	Type             string             `xorm:"INDEX UNIQUE(s) NOT NULL"`
	IssueID          int64              `xorm:"INDEX UNIQUE(s) NOT NULL"`
	CommentID        int64              `xorm:"INDEX UNIQUE(s)"`
	UserID           int64              `xorm:"INDEX UNIQUE(s) NOT NULL"`
	OriginalAuthorID int64              `xorm:"INDEX UNIQUE(s) NOT NULL DEFAULT(0)"`
	OriginalAuthor   string             `xorm:"INDEX UNIQUE(s)"`
	User             *user_model.User   `xorm:"-"`
	CreatedUnix      timeutil.TimeStamp `xorm:"INDEX created"`
}

func init() {
	db.RegisterModel(new(Reaction))
}

// FindReactionsOptions describes the conditions to Find reactions
type FindReactionsOptions struct {
	db.ListOptions
	IssueID   int64
	CommentID int64
	UserID    int64
	Reaction  string
}

func (opts *FindReactionsOptions) toConds() builder.Cond {
	// If Issue ID is set add to Query
	cond := builder.NewCond()
	if opts.IssueID > 0 {
		cond = cond.And(builder.Eq{"reaction.issue_id": opts.IssueID})
	}
	// If CommentID is > 0 add to Query
	// If it is 0 Query ignore CommentID to select
	// If it is -1 it explicit search of Issue Reactions where CommentID = 0
	if opts.CommentID > 0 {
		cond = cond.And(builder.Eq{"reaction.comment_id": opts.CommentID})
	} else if opts.CommentID == -1 {
		cond = cond.And(builder.Eq{"reaction.comment_id": 0})
	}
	if opts.UserID > 0 {
		cond = cond.And(builder.Eq{
			"reaction.user_id":            opts.UserID,
			"reaction.original_author_id": 0,
		})
	}
	if opts.Reaction != "" {
		cond = cond.And(builder.Eq{"reaction.type": opts.Reaction})
	}

	return cond
}

// FindCommentReactions returns a ReactionList of all reactions from an comment
func FindCommentReactions(comment *Comment) (ReactionList, int64, error) {
	return findReactions(db.GetEngine(db.DefaultContext), FindReactionsOptions{
		IssueID:   comment.IssueID,
		CommentID: comment.ID,
	})
}

// FindIssueReactions returns a ReactionList of all reactions from an issue
func FindIssueReactions(issue *Issue, listOptions db.ListOptions) (ReactionList, int64, error) {
	return findReactions(db.GetEngine(db.DefaultContext), FindReactionsOptions{
		ListOptions: listOptions,
		IssueID:     issue.ID,
		CommentID:   -1,
	})
}

func findReactions(e db.Engine, opts FindReactionsOptions) ([]*Reaction, int64, error) {
	sess := e.
		Where(opts.toConds()).
		In("reaction.`type`", setting.UI.Reactions).
		Asc("reaction.issue_id", "reaction.comment_id", "reaction.created_unix", "reaction.id")
	if opts.Page != 0 {
		sess = db.SetSessionPagination(sess, &opts)

		reactions := make([]*Reaction, 0, opts.PageSize)
		count, err := sess.FindAndCount(&reactions)
		return reactions, count, err
	}

	reactions := make([]*Reaction, 0, 10)
	count, err := sess.FindAndCount(&reactions)
	return reactions, count, err
}

func createReaction(e db.Engine, opts *ReactionOptions) (*Reaction, error) {
	reaction := &Reaction{
		Type:    opts.Type,
		UserID:  opts.Doer.ID,
		IssueID: opts.Issue.ID,
	}
	findOpts := FindReactionsOptions{
		IssueID:   opts.Issue.ID,
		CommentID: -1, // reaction to issue only
		Reaction:  opts.Type,
		UserID:    opts.Doer.ID,
	}
	if opts.Comment != nil {
		reaction.CommentID = opts.Comment.ID
		findOpts.CommentID = opts.Comment.ID
	}

	existingR, _, err := findReactions(e, findOpts)
	if err != nil {
		return nil, err
	}
	if len(existingR) > 0 {
		return existingR[0], ErrReactionAlreadyExist{Reaction: opts.Type}
	}

	if _, err := e.Insert(reaction); err != nil {
		return nil, err
	}

	return reaction, nil
}

// ReactionOptions defines options for creating or deleting reactions
type ReactionOptions struct {
	Type    string
	Doer    *user_model.User
	Issue   *Issue
	Comment *Comment
}

// CreateReaction creates reaction for issue or comment.
func CreateReaction(opts *ReactionOptions) (*Reaction, error) {
	if !setting.UI.ReactionsMap[opts.Type] {
		return nil, ErrForbiddenIssueReaction{opts.Type}
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return nil, err
	}
	defer committer.Close()

	reaction, err := createReaction(db.GetEngine(ctx), opts)
	if err != nil {
		return reaction, err
	}

	if err := committer.Commit(); err != nil {
		return nil, err
	}
	return reaction, nil
}

// CreateIssueReaction creates a reaction on issue.
func CreateIssueReaction(doer *user_model.User, issue *Issue, content string) (*Reaction, error) {
	return CreateReaction(&ReactionOptions{
		Type:  content,
		Doer:  doer,
		Issue: issue,
	})
}

// CreateCommentReaction creates a reaction on comment.
func CreateCommentReaction(doer *user_model.User, issue *Issue, comment *Comment, content string) (*Reaction, error) {
	return CreateReaction(&ReactionOptions{
		Type:    content,
		Doer:    doer,
		Issue:   issue,
		Comment: comment,
	})
}

func deleteReaction(e db.Engine, opts *ReactionOptions) error {
	reaction := &Reaction{
		Type: opts.Type,
	}
	if opts.Doer != nil {
		reaction.UserID = opts.Doer.ID
	}
	if opts.Issue != nil {
		reaction.IssueID = opts.Issue.ID
	}
	if opts.Comment != nil {
		reaction.CommentID = opts.Comment.ID
	}
	_, err := e.Where("original_author_id = 0").Delete(reaction)
	return err
}

// DeleteReaction deletes reaction for issue or comment.
func DeleteReaction(opts *ReactionOptions) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := deleteReaction(db.GetEngine(ctx), opts); err != nil {
		return err
	}

	return committer.Commit()
}

// DeleteIssueReaction deletes a reaction on issue.
func DeleteIssueReaction(doer *user_model.User, issue *Issue, content string) error {
	return DeleteReaction(&ReactionOptions{
		Type:  content,
		Doer:  doer,
		Issue: issue,
	})
}

// DeleteCommentReaction deletes a reaction on comment.
func DeleteCommentReaction(doer *user_model.User, issue *Issue, comment *Comment, content string) error {
	return DeleteReaction(&ReactionOptions{
		Type:    content,
		Doer:    doer,
		Issue:   issue,
		Comment: comment,
	})
}

// LoadUser load user of reaction
func (r *Reaction) LoadUser() (*user_model.User, error) {
	if r.User != nil {
		return r.User, nil
	}
	user, err := user_model.GetUserByIDCtx(db.DefaultContext, r.UserID)
	if err != nil {
		return nil, err
	}
	r.User = user
	return user, nil
}

// ReactionList represents list of reactions
type ReactionList []*Reaction

// HasUser check if user has reacted
func (list ReactionList) HasUser(userID int64) bool {
	if userID == 0 {
		return false
	}
	for _, reaction := range list {
		if reaction.OriginalAuthor == "" && reaction.UserID == userID {
			return true
		}
	}
	return false
}

// GroupByType returns reactions grouped by type
func (list ReactionList) GroupByType() map[string]ReactionList {
	reactions := make(map[string]ReactionList)
	for _, reaction := range list {
		reactions[reaction.Type] = append(reactions[reaction.Type], reaction)
	}
	return reactions
}

func (list ReactionList) getUserIDs() []int64 {
	userIDs := make(map[int64]struct{}, len(list))
	for _, reaction := range list {
		if reaction.OriginalAuthor != "" {
			continue
		}
		if _, ok := userIDs[reaction.UserID]; !ok {
			userIDs[reaction.UserID] = struct{}{}
		}
	}
	return keysInt64(userIDs)
}

func (list ReactionList) loadUsers(e db.Engine, repo *repo_model.Repository) ([]*user_model.User, error) {
	if len(list) == 0 {
		return nil, nil
	}

	userIDs := list.getUserIDs()
	userMaps := make(map[int64]*user_model.User, len(userIDs))
	err := e.
		In("id", userIDs).
		Find(&userMaps)
	if err != nil {
		return nil, fmt.Errorf("find user: %v", err)
	}

	for _, reaction := range list {
		if reaction.OriginalAuthor != "" {
			reaction.User = user_model.NewReplaceUser(fmt.Sprintf("%s(%s)", reaction.OriginalAuthor, repo.OriginalServiceType.Name()))
		} else if user, ok := userMaps[reaction.UserID]; ok {
			reaction.User = user
		} else {
			reaction.User = user_model.NewGhostUser()
		}
	}
	return valuesUser(userMaps), nil
}

// LoadUsers loads reactions' all users
func (list ReactionList) LoadUsers(repo *repo_model.Repository) ([]*user_model.User, error) {
	return list.loadUsers(db.GetEngine(db.DefaultContext), repo)
}

// GetFirstUsers returns first reacted user display names separated by comma
func (list ReactionList) GetFirstUsers() string {
	var buffer bytes.Buffer
	rem := setting.UI.ReactionMaxUserNum
	for _, reaction := range list {
		if buffer.Len() > 0 {
			buffer.WriteString(", ")
		}
		buffer.WriteString(reaction.User.DisplayName())
		if rem--; rem == 0 {
			break
		}
	}
	return buffer.String()
}

// GetMoreUserCount returns count of not shown users in reaction tooltip
func (list ReactionList) GetMoreUserCount() int {
	if len(list) <= setting.UI.ReactionMaxUserNum {
		return 0
	}
	return len(list) - setting.UI.ReactionMaxUserNum
}

// RemapExternalUser ExternalUserRemappable interface
func (r *Reaction) RemapExternalUser(externalName string, externalID, userID int64) error {
	r.OriginalAuthor = externalName
	r.OriginalAuthorID = externalID
	r.UserID = userID
	return nil
}

// GetUserID ExternalUserRemappable interface
func (r *Reaction) GetUserID() int64 { return r.UserID }

// GetExternalName ExternalUserRemappable interface
func (r *Reaction) GetExternalName() string { return r.OriginalAuthor }

// GetExternalID ExternalUserRemappable interface
func (r *Reaction) GetExternalID() int64 { return r.OriginalAuthorID }
