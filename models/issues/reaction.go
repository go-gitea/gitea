// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ErrForbiddenIssueReaction is used when a forbidden reaction was try to created
type ErrForbiddenIssueReaction struct {
	Reaction string
}

// IsErrForbiddenIssueReaction checks if an error is a ErrForbiddenIssueReaction.
func IsErrForbiddenIssueReaction(err error) bool {
	_, ok := err.(ErrForbiddenIssueReaction)
	return ok
}

func (err ErrForbiddenIssueReaction) Error() string {
	return fmt.Sprintf("'%s' is not an allowed reaction", err.Reaction)
}

func (err ErrForbiddenIssueReaction) Unwrap() error {
	return util.ErrPermissionDenied
}

// ErrReactionAlreadyExist is used when a existing reaction was try to created
type ErrReactionAlreadyExist struct {
	Reaction string
}

// IsErrReactionAlreadyExist checks if an error is a ErrReactionAlreadyExist.
func IsErrReactionAlreadyExist(err error) bool {
	_, ok := err.(ErrReactionAlreadyExist)
	return ok
}

func (err ErrReactionAlreadyExist) Error() string {
	return fmt.Sprintf("reaction '%s' already exists", err.Reaction)
}

func (err ErrReactionAlreadyExist) Unwrap() error {
	return util.ErrAlreadyExist
}

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

// LoadUser load user of reaction
func (r *Reaction) LoadUser(ctx context.Context) (*user_model.User, error) {
	if r.User != nil {
		return r.User, nil
	}
	user, err := user_model.GetUserByID(ctx, r.UserID)
	if err != nil {
		return nil, err
	}
	r.User = user
	return user, nil
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
func FindCommentReactions(ctx context.Context, issueID, commentID int64) (ReactionList, int64, error) {
	return FindReactions(ctx, FindReactionsOptions{
		IssueID:   issueID,
		CommentID: commentID,
	})
}

// FindIssueReactions returns a ReactionList of all reactions from an issue
func FindIssueReactions(ctx context.Context, issueID int64, listOptions db.ListOptions) (ReactionList, int64, error) {
	return FindReactions(ctx, FindReactionsOptions{
		ListOptions: listOptions,
		IssueID:     issueID,
		CommentID:   -1,
	})
}

// FindReactions returns a ReactionList of all reactions from an issue or a comment
func FindReactions(ctx context.Context, opts FindReactionsOptions) (ReactionList, int64, error) {
	sess := db.GetEngine(ctx).
		Where(opts.toConds()).
		In("reaction.`type`", setting.UI.Reactions).
		Asc("reaction.issue_id", "reaction.comment_id", "reaction.created_unix", "reaction.id")
	if opts.Page > 0 {
		sess = db.SetSessionPagination(sess, &opts)

		reactions := make([]*Reaction, 0, opts.PageSize)
		count, err := sess.FindAndCount(&reactions)
		return reactions, count, err
	}

	reactions := make([]*Reaction, 0, 10)
	count, err := sess.FindAndCount(&reactions)
	return reactions, count, err
}

func createReaction(ctx context.Context, opts *ReactionOptions) (*Reaction, error) {
	reaction := &Reaction{
		Type:      opts.Type,
		UserID:    opts.DoerID,
		IssueID:   opts.IssueID,
		CommentID: opts.CommentID,
	}
	findOpts := FindReactionsOptions{
		IssueID:   opts.IssueID,
		CommentID: opts.CommentID,
		Reaction:  opts.Type,
		UserID:    opts.DoerID,
	}
	if findOpts.CommentID == 0 {
		// explicit search of Issue Reactions where CommentID = 0
		findOpts.CommentID = -1
	}

	existingR, _, err := FindReactions(ctx, findOpts)
	if err != nil {
		return nil, err
	}
	if len(existingR) > 0 {
		return existingR[0], ErrReactionAlreadyExist{Reaction: opts.Type}
	}

	if err := db.Insert(ctx, reaction); err != nil {
		return nil, err
	}

	return reaction, nil
}

// ReactionOptions defines options for creating or deleting reactions
type ReactionOptions struct {
	Type      string
	DoerID    int64
	IssueID   int64
	CommentID int64
}

// CreateReaction creates reaction for issue or comment.
func CreateReaction(ctx context.Context, opts *ReactionOptions) (*Reaction, error) {
	if !setting.UI.ReactionsLookup.Contains(opts.Type) {
		return nil, ErrForbiddenIssueReaction{opts.Type}
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, err
	}
	defer committer.Close()

	reaction, err := createReaction(ctx, opts)
	if err != nil {
		return reaction, err
	}

	if err := committer.Commit(); err != nil {
		return nil, err
	}
	return reaction, nil
}

// DeleteReaction deletes reaction for issue or comment.
func DeleteReaction(ctx context.Context, opts *ReactionOptions) error {
	reaction := &Reaction{
		Type:      opts.Type,
		UserID:    opts.DoerID,
		IssueID:   opts.IssueID,
		CommentID: opts.CommentID,
	}

	sess := db.GetEngine(ctx).Where("original_author_id = 0")
	if opts.CommentID == -1 {
		reaction.CommentID = 0
		sess.MustCols("comment_id")
	}

	_, err := sess.Delete(reaction)
	return err
}

// DeleteIssueReaction deletes a reaction on issue.
func DeleteIssueReaction(ctx context.Context, doerID, issueID int64, content string) error {
	return DeleteReaction(ctx, &ReactionOptions{
		Type:      content,
		DoerID:    doerID,
		IssueID:   issueID,
		CommentID: -1,
	})
}

// DeleteCommentReaction deletes a reaction on comment.
func DeleteCommentReaction(ctx context.Context, doerID, issueID, commentID int64, content string) error {
	return DeleteReaction(ctx, &ReactionOptions{
		Type:      content,
		DoerID:    doerID,
		IssueID:   issueID,
		CommentID: commentID,
	})
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
	return container.FilterSlice(list, func(reaction *Reaction) (int64, bool) {
		if reaction.OriginalAuthor != "" {
			return 0, false
		}
		return reaction.UserID, true
	})
}

func valuesUser(m map[int64]*user_model.User) []*user_model.User {
	values := make([]*user_model.User, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

// newMigrationOriginalUser creates and returns a fake user for external user
func newMigrationOriginalUser(name string) *user_model.User {
	return &user_model.User{ID: 0, Name: name, LowerName: strings.ToLower(name)}
}

// LoadUsers loads reactions' all users
func (list ReactionList) LoadUsers(ctx context.Context, repo *repo_model.Repository) ([]*user_model.User, error) {
	if len(list) == 0 {
		return nil, nil
	}

	userIDs := list.getUserIDs()
	userMaps := make(map[int64]*user_model.User, len(userIDs))
	err := db.GetEngine(ctx).
		In("id", userIDs).
		Find(&userMaps)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}

	for _, reaction := range list {
		if reaction.OriginalAuthor != "" {
			reaction.User = newMigrationOriginalUser(fmt.Sprintf("%s(%s)", reaction.OriginalAuthor, repo.OriginalServiceType.Name()))
		} else if user, ok := userMaps[reaction.UserID]; ok {
			reaction.User = user
		} else {
			reaction.User = user_model.NewGhostUser()
		}
	}
	return valuesUser(userMaps), nil
}

// GetFirstUsers returns first reacted user display names separated by comma
func (list ReactionList) GetFirstUsers() string {
	var buffer bytes.Buffer
	rem := setting.UI.ReactionMaxUserNum
	for _, reaction := range list {
		if buffer.Len() > 0 {
			buffer.WriteString(", ")
		}
		buffer.WriteString(reaction.User.Name)
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
