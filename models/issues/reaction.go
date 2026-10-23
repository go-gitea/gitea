// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

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

// Reaction represents a reaction on an issue, comment or release.
type Reaction struct {
	ID int64 `xorm:"pk autoincr"`

	UserID           int64  `xorm:"UNIQUE(s) NOT NULL"` // first unique column implies INDEX
	IssueID          int64  `xorm:"UNIQUE(s) INDEX(issue_comment) NOT NULL"`
	CommentID        int64  `xorm:"UNIQUE(s) INDEX(issue_comment) INDEX NOT NULL"`
	ReleaseID        int64  `xorm:"UNIQUE(s) INDEX NOT NULL DEFAULT(0)"`
	OriginalAuthorID int64  `xorm:"UNIQUE(s) NOT NULL DEFAULT(0)"`
	OriginalAuthor   string `xorm:"UNIQUE(s) NOT NULL DEFAULT('')"`
	Type             string `xorm:"UNIQUE(s) NOT NULL"`

	User        *user_model.User   `xorm:"-"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
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
	IssueIDOnly         int64
	IssueIDWithComments int64
	CommentID           int64
	ReleaseID           int64
	UserID              int64
}

func (opts *FindReactionsOptions) toConds() builder.Cond {
	// If Issue ID is set add to Query
	cond := builder.NewCond()
	if opts.IssueIDOnly > 0 {
		cond = cond.And(builder.Eq{"reaction.issue_id": opts.IssueIDOnly}).And(builder.Expr("reaction.comment_id = 0"))
	}
	if opts.IssueIDWithComments > 0 {
		cond = cond.And(builder.Eq{"reaction.issue_id": opts.IssueIDWithComments})
	}
	if opts.CommentID > 0 {
		cond = cond.And(builder.Eq{"reaction.comment_id": opts.CommentID})
	}
	if opts.ReleaseID > 0 {
		cond = cond.And(builder.Eq{"reaction.release_id": opts.ReleaseID})
	}
	if opts.UserID > 0 {
		cond = cond.And(builder.Eq{"reaction.user_id": opts.UserID, "reaction.original_author_id": 0})
	}
	if !cond.IsValid() {
		setting.PanicInDevOrTesting("FindReactionsOptions.toConds: no conditions specified")
		cond = cond.And(builder.Expr("1=0"))
	}
	return cond
}

// FindCommentReactions returns a ReactionList of all reactions from a comment
func FindCommentReactions(ctx context.Context, commentID int64) (ReactionList, int64, error) {
	return FindReactions(ctx, FindReactionsOptions{CommentID: commentID})
}

// FindIssueReactions returns a ReactionList of all reactions from an issue
func FindIssueReactions(ctx context.Context, issueID int64, listOptions db.ListOptions) (ReactionList, int64, error) {
	return FindReactions(ctx, FindReactionsOptions{
		ListOptions: listOptions,
		IssueIDOnly: issueID,
	})
}

// FindReactions returns a ReactionList of all reactions from an issue or a comment
func FindReactions(ctx context.Context, opts FindReactionsOptions) (ReactionList, int64, error) {
	sess := db.GetEngine(ctx).
		Where(opts.toConds()).
		In("reaction.`type`", setting.UI.Reactions).
		Asc("reaction.issue_id", "reaction.comment_id", "reaction.created_unix", "reaction.id")
	if opts.Page > 0 {
		db.SetSessionPagination(sess, &opts)

		reactions := make([]*Reaction, 0, opts.PageSize)
		count, err := sess.FindAndCount(&reactions)
		return reactions, count, err
	}

	reactions := make([]*Reaction, 0, 10)
	count, err := sess.FindAndCount(&reactions)
	return reactions, count, err
}

func FindReleaseReactions(ctx context.Context, releaseIDs ...int64) (ReactionList, error) {
	reactions := make(ReactionList, 0, 10)
	err := db.GetEngine(ctx).
		In("release_id", releaseIDs).
		In("`type`", setting.UI.ReleaseReactions).
		Asc("created_unix", "id").
		Find(&reactions)
	return reactions, err
}

// ReactionOptions defines options for creating or deleting reactions
type ReactionOptions struct {
	Type      string
	DoerID    int64
	IssueID   int64
	CommentID int64
	ReleaseID int64
}

// CreateReaction creates reaction for issue, comment or release.
func CreateReaction(ctx context.Context, allowed container.Set[string], opts *ReactionOptions) (*Reaction, error) {
	if !allowed.Contains(opts.Type) {
		return nil, ErrForbiddenIssueReaction{opts.Type}
	}

	reaction := &Reaction{
		Type:      opts.Type,
		UserID:    opts.DoerID,
		IssueID:   opts.IssueID,
		CommentID: opts.CommentID,
		ReleaseID: opts.ReleaseID,
	}

	err := db.Insert(ctx, reaction)
	if err != nil {
		if has, _ := db.GetEngine(ctx).Get(reaction); has {
			return reaction, ErrReactionAlreadyExist{Reaction: opts.Type}
		}
		return nil, err
	}
	return reaction, nil
}

func DeleteIssueReaction(ctx context.Context, doerID, issueID int64, content string) error {
	// here and below: the migration reactions have unrelated user_id (might be the doer?)
	// so skip the deletion if the reaction is from an external user (original_author_id != 0)
	sess := db.GetEngine(ctx).Where("issue_id = ? AND comment_id = 0 AND user_id = ? AND original_author_id = 0 AND `type` = ?", issueID, doerID, content)
	_, err := sess.Delete(&Reaction{})
	return err
}

func DeleteCommentReaction(ctx context.Context, doerID, issueID, commentID int64, content string) error {
	sess := db.GetEngine(ctx).Where("issue_id = ? AND comment_id = ? AND user_id = ? AND original_author_id = 0 AND `type` = ?", issueID, commentID, doerID, content)
	_, err := sess.Delete(&Reaction{})
	return err
}

func DeleteCommentAllReactions(ctx context.Context, commentID int64) error {
	sess := db.GetEngine(ctx).Where("comment_id = ?", commentID)
	_, err := sess.Delete(&Reaction{})
	return err
}

func DeleteUserReactions(ctx context.Context, userID int64) error {
	sess := db.GetEngine(ctx).Where("user_id = ?", userID)
	_, err := sess.Delete(&Reaction{})
	return err
}

func DeleteReleaseReaction(ctx context.Context, doerID, releaseID int64, content string) error {
	sess := db.GetEngine(ctx).Where("release_id = ? AND user_id = ? AND `type` = ?", releaseID, doerID, content)
	_, err := sess.Delete(&Reaction{})
	return err
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

// newMigrationOriginalUser creates and returns a fake user for external user
func newMigrationOriginalUser(name string) *user_model.User {
	return &user_model.User{ID: 0, Name: name, LowerName: strings.ToLower(name)}
}

// LoadUsers loads reactions' all users
func (list ReactionList) LoadUsers(ctx context.Context, repo *repo_model.Repository) (map[int64]*user_model.User, error) {
	if len(list) == 0 {
		return map[int64]*user_model.User{}, nil
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
	return userMaps, nil
}

// GetFirstUsers returns first reacted user display names separated by comma
func (list ReactionList) GetFirstUsers() string {
	var buffer bytes.Buffer
	rem := setting.UI.ReactionMaxUserNum
	for _, reaction := range list {
		if buffer.Len() > 0 {
			buffer.WriteString(", ")
		}
		buffer.WriteString(reaction.User.DisplayNameWithFullName())
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
