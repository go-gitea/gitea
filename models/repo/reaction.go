// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// ErrForbiddenReaction is used when a forbidden reaction was try to created
type ErrForbiddenReaction struct {
	Reaction string
}

// IsErrForbiddenReaction checks if an error is a ErrForbiddenReaction.
func IsErrForbiddenReaction(err error) bool {
	_, ok := err.(ErrForbiddenReaction)
	return ok
}

func (err ErrForbiddenReaction) Error() string {
	return fmt.Sprintf("'%s' is not an allowed reaction", err.Reaction)
}

func (err ErrForbiddenReaction) Unwrap() error {
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

// Reaction represents a reactions on issues, comments, and releases.
type Reaction struct {
	ID               int64              `xorm:"pk autoincr"`
	Type             string             `xorm:"INDEX UNIQUE(s) NOT NULL"`
	IssueID          int64              `xorm:"INDEX UNIQUE(s) NOT NULL DEFAULT(0)"`
	CommentID        int64              `xorm:"INDEX UNIQUE(s) NOT NULL DEFAULT(0)"`
	ReleaseID        int64              `xorm:"INDEX UNIQUE(s) NOT NULL DEFAULT(0)"`
	UserID           int64              `xorm:"INDEX UNIQUE(s) NOT NULL"`
	OriginalAuthorID int64              `xorm:"INDEX UNIQUE(s) NOT NULL DEFAULT(0)"`
	OriginalAuthor   string             `xorm:"INDEX UNIQUE(s)"`
	User             *user_model.User   `xorm:"-"`
	CreatedUnix      timeutil.TimeStamp `xorm:"INDEX created"`
}

func init() {
	db.RegisterModel(new(Reaction))
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

func (r *Reaction) GetType() string           { return r.Type }
func (r *Reaction) GetOriginalAuthor() string { return r.OriginalAuthor }
func (r *Reaction) GetUser() *user_model.User { return r.User }

// FindReactionsOptions describes the conditions to Find reactions
type FindReactionsOptions struct {
	db.ListOptions
	IssueID   int64
	CommentID int64
	ReleaseID int64
	UserID    int64
	Reaction  string
}

func (opts *FindReactionsOptions) toConds() builder.Cond {
	cond := builder.NewCond()
	if opts.IssueID > 0 {
		cond = cond.And(builder.Eq{"reaction.issue_id": opts.IssueID})
	}
	if opts.CommentID > 0 {
		cond = cond.And(builder.Eq{"reaction.comment_id": opts.CommentID})
	} else if opts.CommentID == -1 {
		cond = cond.And(builder.Eq{"reaction.comment_id": 0})
	}
	if opts.ReleaseID > 0 {
		cond = cond.And(builder.Eq{"reaction.release_id": opts.ReleaseID})
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

// FindReactions returns a ReactionList of all reactions matching options
func FindReactions(ctx context.Context, opts FindReactionsOptions) (ReactionList, int64, error) {
	sess := db.GetEngine(ctx).
		Where(opts.toConds()).
		In("reaction.`type`", setting.UI.Reactions).
		Asc("reaction.issue_id", "reaction.comment_id", "reaction.release_id", "reaction.created_unix", "reaction.id")
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

// FindReactionsForReleases finds reactions for a slice of releases
func FindReactionsForReleases(ctx context.Context, releases []*Release) (map[int64]ReactionList, error) {
	if len(releases) == 0 {
		return make(map[int64]ReactionList), nil
	}
	releaseIDs := make([]int64, len(releases))
	for i, r := range releases {
		releaseIDs[i] = r.ID
	}
	reactions := make([]*Reaction, 0, 10)
	err := db.GetEngine(ctx).
		In("release_id", releaseIDs).
		In("`type`", setting.UI.Reactions).
		Asc("release_id", "created_unix", "id").
		Find(&reactions)
	if err != nil {
		return nil, err
	}
	reactionsMap := make(map[int64]ReactionList)
	for _, reaction := range reactions {
		reactionsMap[reaction.ReleaseID] = append(reactionsMap[reaction.ReleaseID], reaction)
	}
	return reactionsMap, nil
}

func createReaction(ctx context.Context, opts *ReactionOptions) (*Reaction, error) {
	reaction := &Reaction{
		Type:      opts.Type,
		UserID:    opts.DoerID,
		IssueID:   opts.IssueID,
		CommentID: opts.CommentID,
		ReleaseID: opts.ReleaseID,
	}
	findOpts := FindReactionsOptions{
		IssueID:   opts.IssueID,
		CommentID: opts.CommentID,
		ReleaseID: opts.ReleaseID,
		Reaction:  opts.Type,
		UserID:    opts.DoerID,
	}
	if findOpts.CommentID == 0 && findOpts.ReleaseID == 0 {
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
	ReleaseID int64
}

// CreateReaction creates reaction for issue, comment or release.
func CreateReaction(ctx context.Context, opts *ReactionOptions) (*Reaction, error) {
	if !setting.UI.ReactionsLookup.Contains(opts.Type) {
		return nil, ErrForbiddenReaction{opts.Type}
	}

	return db.WithTx2(ctx, func(ctx context.Context) (*Reaction, error) {
		return createReaction(ctx, opts)
	})
}

// DeleteReaction deletes reaction for issue, comment or release.
func DeleteReaction(ctx context.Context, opts *ReactionOptions) error {
	reaction := &Reaction{
		Type:      opts.Type,
		UserID:    opts.DoerID,
		IssueID:   opts.IssueID,
		CommentID: opts.CommentID,
		ReleaseID: opts.ReleaseID,
	}

	sess := db.GetEngine(ctx).Where("original_author_id = 0")
	if opts.CommentID == -1 {
		reaction.CommentID = 0
		sess.MustCols("comment_id")
	}

	_, err := sess.Delete(reaction)
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

func valuesUser(m map[int64]*user_model.User) []*user_model.User {
	values := make([]*user_model.User, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

func newMigrationOriginalUser(name string) *user_model.User {
	return &user_model.User{ID: 0, Name: name, LowerName: strings.ToLower(name)}
}

// LoadUsers loads reactions' all users
func (list ReactionList) LoadUsers(ctx context.Context, repo *Repository) ([]*user_model.User, error) {
	if len(list) == 0 {
		return nil, nil
	}

	userIDs := list.getUserIDs()
	userMaps := make(map[int64]*user_model.User, len(userIDs))
	if len(userIDs) > 0 {
		err := db.GetEngine(ctx).
			In("id", userIDs).
			Find(&userMaps)
		if err != nil {
			return nil, fmt.Errorf("find user: %w", err)
		}
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

// LoadUsersMap loads reactions' all users for a map of reactions
func LoadUsersMap(ctx context.Context, repo *Repository, reactionsMap map[int64]ReactionList) ([]*user_model.User, error) {
	if len(reactionsMap) == 0 {
		return nil, nil
	}

	userIDsMap := make(map[int64]bool)
	for _, list := range reactionsMap {
		for _, reaction := range list {
			if reaction.UserID > 0 && reaction.OriginalAuthor == "" {
				userIDsMap[reaction.UserID] = true
			}
		}
	}
	if len(userIDsMap) == 0 {
		for _, list := range reactionsMap {
			for _, reaction := range list {
				if reaction.OriginalAuthor != "" {
					reaction.User = newMigrationOriginalUser(fmt.Sprintf("%s(%s)", reaction.OriginalAuthor, repo.OriginalServiceType.Name()))
				} else {
					reaction.User = user_model.NewGhostUser()
				}
			}
		}
		return nil, nil
	}

	userIDs := make([]int64, 0, len(userIDsMap))
	for id := range userIDsMap {
		userIDs = append(userIDs, id)
	}

	userMaps := make(map[int64]*user_model.User, len(userIDs))
	err := db.GetEngine(ctx).
		In("id", userIDs).
		Find(&userMaps)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}

	for _, list := range reactionsMap {
		for _, reaction := range list {
			if reaction.OriginalAuthor != "" {
				reaction.User = newMigrationOriginalUser(fmt.Sprintf("%s(%s)", reaction.OriginalAuthor, repo.OriginalServiceType.Name()))
			} else if user, ok := userMaps[reaction.UserID]; ok {
				reaction.User = user
			} else {
				reaction.User = user_model.NewGhostUser()
			}
		}
	}
	return valuesUser(userMaps), nil
}
