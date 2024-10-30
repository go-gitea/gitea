// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conversations

import (
	"bytes"
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ErrForbiddenConversationReaction is used when a forbidden reaction was try to created
type ErrForbiddenConversationReaction struct {
	Reaction string
}

// IsErrForbiddenConversationReaction checks if an error is a ErrForbiddenConversationReaction.
func IsErrForbiddenConversationReaction(err error) bool {
	_, ok := err.(ErrForbiddenConversationReaction)
	return ok
}

func (err ErrForbiddenConversationReaction) Error() string {
	return fmt.Sprintf("'%s' is not an allowed reaction", err.Reaction)
}

func (err ErrForbiddenConversationReaction) Unwrap() error {
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

// CommentReaction represents a reactions on conversations and comments.
type CommentReaction struct {
	ID               int64              `xorm:"pk autoincr"`
	Type             string             `xorm:"INDEX UNIQUE(s) NOT NULL"`
	ConversationID   int64              `xorm:"INDEX UNIQUE(s) NOT NULL"`
	CommentID        int64              `xorm:"INDEX UNIQUE(s)"`
	UserID           int64              `xorm:"INDEX UNIQUE(s) NOT NULL"`
	OriginalAuthorID int64              `xorm:"INDEX UNIQUE(s) NOT NULL DEFAULT(0)"`
	OriginalAuthor   string             `xorm:"INDEX UNIQUE(s)"`
	User             *user_model.User   `xorm:"-"`
	CreatedUnix      timeutil.TimeStamp `xorm:"INDEX created"`
}

// LoadUser load user of reaction
func (r *CommentReaction) LoadUser(ctx context.Context) (*user_model.User, error) {
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
func (r *CommentReaction) RemapExternalUser(externalName string, externalID, userID int64) error {
	r.OriginalAuthor = externalName
	r.OriginalAuthorID = externalID
	r.UserID = userID
	return nil
}

// GetUserID ExternalUserRemappable interface
func (r *CommentReaction) GetUserID() int64 { return r.UserID }

// GetExternalName ExternalUserRemappable interface
func (r *CommentReaction) GetExternalName() string { return r.OriginalAuthor }

// GetExternalID ExternalUserRemappable interface
func (r *CommentReaction) GetExternalID() int64 { return r.OriginalAuthorID }

func init() {
	db.RegisterModel(new(CommentReaction))
}

// FindReactionsOptions describes the conditions to Find reactions
type FindReactionsOptions struct {
	db.ListOptions
	ConversationID int64
	CommentID      int64
	UserID         int64
	Reaction       string
}

func (opts *FindReactionsOptions) toConds() builder.Cond {
	// If Conversation ID is set add to Query
	cond := builder.NewCond()
	if opts.ConversationID > 0 {
		cond = cond.And(builder.Eq{"comment_reaction.conversation_id": opts.ConversationID})
	}
	// If CommentID is > 0 add to Query
	// If it is 0 Query ignore CommentID to select
	// If it is -1 it explicit search of Conversation Reactions where CommentID = 0
	if opts.CommentID > 0 {
		cond = cond.And(builder.Eq{"comment_reaction.comment_id": opts.CommentID})
	} else if opts.CommentID == -1 {
		cond = cond.And(builder.Eq{"comment_reaction.comment_id": 0})
	}
	if opts.UserID > 0 {
		cond = cond.And(builder.Eq{
			"comment_reaction.user_id":            opts.UserID,
			"comment_reaction.original_author_id": 0,
		})
	}
	if opts.Reaction != "" {
		cond = cond.And(builder.Eq{"comment_reaction.type": opts.Reaction})
	}

	return cond
}

// FindCommentReactions returns a ReactionList of all reactions from an comment
func FindCommentReactions(ctx context.Context, conversationID, commentID int64) (ReactionList, int64, error) {
	return FindReactions(ctx, FindReactionsOptions{
		ConversationID: conversationID,
		CommentID:      commentID,
	})
}

// FindConversationReactions returns a ReactionList of all reactions from an conversation
func FindConversationReactions(ctx context.Context, conversationID int64, listOptions db.ListOptions) (ReactionList, int64, error) {
	return FindReactions(ctx, FindReactionsOptions{
		ListOptions:    listOptions,
		ConversationID: conversationID,
		CommentID:      -1,
	})
}

// FindReactions returns a ReactionList of all reactions from an conversation or a comment
func FindReactions(ctx context.Context, opts FindReactionsOptions) (ReactionList, int64, error) {
	sess := db.GetEngine(ctx).
		Where(opts.toConds()).
		In("comment_reaction.`type`", setting.UI.Reactions).
		Asc("comment_reaction.conversation_id", "comment_reaction.comment_id", "comment_reaction.created_unix", "comment_reaction.id")
	if opts.Page != 0 {
		sess = db.SetSessionPagination(sess, &opts)

		reactions := make([]*CommentReaction, 0, opts.PageSize)
		count, err := sess.FindAndCount(&reactions)
		return reactions, count, err
	}

	reactions := make([]*CommentReaction, 0, 10)
	count, err := sess.FindAndCount(&reactions)
	return reactions, count, err
}

func createReaction(ctx context.Context, opts *ReactionOptions) (*CommentReaction, error) {
	reaction := &CommentReaction{
		Type:           opts.Type,
		UserID:         opts.DoerID,
		ConversationID: opts.ConversationID,
		CommentID:      opts.CommentID,
	}
	findOpts := FindReactionsOptions{
		ConversationID: opts.ConversationID,
		CommentID:      opts.CommentID,
		Reaction:       opts.Type,
		UserID:         opts.DoerID,
	}
	if findOpts.CommentID == 0 {
		// explicit search of Conversation Reactions where CommentID = 0
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
	Type           string
	DoerID         int64
	ConversationID int64
	CommentID      int64
}

// CreateReaction creates reaction for conversation or comment.
func CreateReaction(ctx context.Context, opts *ReactionOptions) (*CommentReaction, error) {
	if !setting.UI.ReactionsLookup.Contains(opts.Type) {
		return nil, ErrForbiddenConversationReaction{opts.Type}
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

// DeleteReaction deletes reaction for conversation or comment.
func DeleteReaction(ctx context.Context, opts *ReactionOptions) error {
	reaction := &CommentReaction{
		Type:           opts.Type,
		UserID:         opts.DoerID,
		ConversationID: opts.ConversationID,
		CommentID:      opts.CommentID,
	}

	sess := db.GetEngine(ctx).Where("original_author_id = 0")
	if opts.CommentID == -1 {
		reaction.CommentID = 0
		sess.MustCols("comment_id")
	}

	_, err := sess.Delete(reaction)
	return err
}

// DeleteConversationReaction deletes a reaction on conversation.
func DeleteConversationReaction(ctx context.Context, doerID, conversationID int64, content string) error {
	return DeleteReaction(ctx, &ReactionOptions{
		Type:           content,
		DoerID:         doerID,
		ConversationID: conversationID,
		CommentID:      -1,
	})
}

// DeleteCommentReaction deletes a reaction on comment.
func DeleteCommentReaction(ctx context.Context, doerID, conversationID, commentID int64, content string) error {
	return DeleteReaction(ctx, &ReactionOptions{
		Type:           content,
		DoerID:         doerID,
		ConversationID: conversationID,
		CommentID:      commentID,
	})
}

// ReactionList represents list of reactions
type ReactionList []*CommentReaction

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
	return container.FilterSlice(list, func(reaction *CommentReaction) (int64, bool) {
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
			reaction.User = user_model.NewReplaceUser(fmt.Sprintf("%s(%s)", reaction.OriginalAuthor, repo.OriginalServiceType.Name()))
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
