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

// ErrForbiddenReleaseReaction is used when a forbidden reaction was tried to be created
type ErrForbiddenReleaseReaction struct {
	Reaction string
}

// IsErrForbiddenReleaseReaction checks if an error is a ErrForbiddenReleaseReaction.
func IsErrForbiddenReleaseReaction(err error) bool {
	_, ok := err.(ErrForbiddenReleaseReaction)
	return ok
}

func (err ErrForbiddenReleaseReaction) Error() string {
	return fmt.Sprintf("'%s' is not an allowed reaction", err.Reaction)
}

func (err ErrForbiddenReleaseReaction) Unwrap() error {
	return util.ErrPermissionDenied
}

// ErrReleaseReactionAlreadyExist is used when an existing reaction was tried to be created
type ErrReleaseReactionAlreadyExist struct {
	Reaction string
}

// IsErrReleaseReactionAlreadyExist checks if an error is a ErrReleaseReactionAlreadyExist.
func IsErrReleaseReactionAlreadyExist(err error) bool {
	_, ok := err.(ErrReleaseReactionAlreadyExist)
	return ok
}

func (err ErrReleaseReactionAlreadyExist) Error() string {
	return fmt.Sprintf("reaction '%s' already exists", err.Reaction)
}

func (err ErrReleaseReactionAlreadyExist) Unwrap() error {
	return util.ErrAlreadyExist
}

// ReleaseReaction represents reactions on releases.
type ReleaseReaction struct {
	ID               int64              `xorm:"pk autoincr"`
	Type             string             `xorm:"INDEX UNIQUE(s) NOT NULL"`
	ReleaseID        int64              `xorm:"INDEX UNIQUE(s) NOT NULL"`
	UserID           int64              `xorm:"INDEX UNIQUE(s) NOT NULL"`
	OriginalAuthorID int64              `xorm:"INDEX UNIQUE(s) NOT NULL DEFAULT(0)"`
	OriginalAuthor   string             `xorm:"INDEX UNIQUE(s)"`
	User             *user_model.User   `xorm:"-"`
	CreatedUnix      timeutil.TimeStamp `xorm:"INDEX created"`
}

func init() {
	db.RegisterModel(new(ReleaseReaction))
}

// LoadUser load user of reaction
func (r *ReleaseReaction) LoadUser(ctx context.Context) (*user_model.User, error) {
	if r.User != nil {
		return r.User, nil
	}
	user, err := user_model.GetUserByID(ctx, r.UserID)
	if err != nil {
		return nil, err
	}
	r.User = user
	return r.User, nil
}

// RemapExternalUser ExternalUserRemappable interface
func (r *ReleaseReaction) RemapExternalUser(externalName string, externalID, userID int64) error {
	r.OriginalAuthor = externalName
	r.OriginalAuthorID = externalID
	r.UserID = userID
	return nil
}

// GetUserID ExternalUserRemappable interface
func (r *ReleaseReaction) GetUserID() int64 { return r.UserID }

// GetExternalName ExternalUserRemappable interface
func (r *ReleaseReaction) GetExternalName() string { return r.OriginalAuthor }

// GetExternalID ExternalUserRemappable interface
func (r *ReleaseReaction) GetExternalID() int64 { return r.OriginalAuthorID }

// FindReleaseReactionsOptions describes the conditions to Find release reactions
type FindReleaseReactionsOptions struct {
	db.ListOptions
	ReleaseID int64
	UserID    int64
	Reaction  string
}

func (opts *FindReleaseReactionsOptions) toConds() builder.Cond {
	cond := builder.NewCond()
	if opts.ReleaseID > 0 {
		cond = cond.And(builder.Eq{"release_reaction.release_id": opts.ReleaseID})
	}
	if opts.UserID > 0 {
		cond = cond.And(builder.Eq{
			"release_reaction.user_id":            opts.UserID,
			"release_reaction.original_author_id": 0,
		})
	}
	if opts.Reaction != "" {
		cond = cond.And(builder.Eq{"release_reaction.type": opts.Reaction})
	}
	return cond
}

// FindReleaseReactions returns a ReleaseReactionList of all reactions for a release
func FindReleaseReactions(ctx context.Context, releaseID int64, listOptions db.ListOptions) (ReleaseReactionList, int64, error) {
	return FindReleaseReactionsWithOpts(ctx, FindReleaseReactionsOptions{
		ListOptions: listOptions,
		ReleaseID:   releaseID,
	})
}

// FindReleaseReactionsWithOpts returns a ReleaseReactionList of all reactions matching options
func FindReleaseReactionsWithOpts(ctx context.Context, opts FindReleaseReactionsOptions) (ReleaseReactionList, int64, error) {
	sess := db.GetEngine(ctx).
		Where(opts.toConds()).
		In("release_reaction.`type`", setting.UI.Reactions).
		Asc("release_reaction.release_id", "release_reaction.created_unix", "release_reaction.id")
	if opts.Page > 0 {
		db.SetSessionPagination(sess, &opts)

		reactions := make([]*ReleaseReaction, 0, opts.PageSize)
		count, err := sess.FindAndCount(&reactions)
		return reactions, count, err
	}

	reactions := make([]*ReleaseReaction, 0, 10)
	count, err := sess.FindAndCount(&reactions)
	return reactions, count, err
}

// FindReactionsForReleases finds reactions for a slice of releases
func FindReactionsForReleases(ctx context.Context, releases []*Release) (map[int64]ReleaseReactionList, error) {
	if len(releases) == 0 {
		return make(map[int64]ReleaseReactionList), nil
	}
	releaseIDs := make([]int64, len(releases))
	for i, r := range releases {
		releaseIDs[i] = r.ID
	}
	reactions := make([]*ReleaseReaction, 0, 10)
	err := db.GetEngine(ctx).
		In("release_id", releaseIDs).
		In("`type`", setting.UI.Reactions).
		Asc("release_id", "created_unix", "id").
		Find(&reactions)
	if err != nil {
		return nil, err
	}
	reactionsMap := make(map[int64]ReleaseReactionList)
	for _, reaction := range reactions {
		reactionsMap[reaction.ReleaseID] = append(reactionsMap[reaction.ReleaseID], reaction)
	}
	return reactionsMap, nil
}

func createReleaseReaction(ctx context.Context, opts *ReleaseReactionOptions) (*ReleaseReaction, error) {
	reaction := &ReleaseReaction{
		Type:      opts.Type,
		UserID:    opts.DoerID,
		ReleaseID: opts.ReleaseID,
	}
	findOpts := FindReleaseReactionsOptions{
		ReleaseID: opts.ReleaseID,
		Reaction:  opts.Type,
		UserID:    opts.DoerID,
	}

	existingR, _, err := FindReleaseReactionsWithOpts(ctx, findOpts)
	if err != nil {
		return nil, err
	}
	if len(existingR) > 0 {
		return existingR[0], ErrReleaseReactionAlreadyExist{Reaction: opts.Type}
	}

	if err := db.Insert(ctx, reaction); err != nil {
		return nil, err
	}

	return reaction, nil
}

// ReleaseReactionOptions defines options for creating or deleting release reactions
type ReleaseReactionOptions struct {
	Type      string
	DoerID    int64
	ReleaseID int64
}

// CreateReleaseReaction creates reaction for release.
func CreateReleaseReaction(ctx context.Context, opts *ReleaseReactionOptions) (*ReleaseReaction, error) {
	if !setting.UI.ReactionsLookup.Contains(opts.Type) {
		return nil, ErrForbiddenReleaseReaction{opts.Type}
	}

	return db.WithTx2(ctx, func(ctx context.Context) (*ReleaseReaction, error) {
		return createReleaseReaction(ctx, opts)
	})
}

// DeleteReleaseReaction deletes reaction for release.
func DeleteReleaseReaction(ctx context.Context, opts *ReleaseReactionOptions) error {
	reaction := &ReleaseReaction{
		Type:      opts.Type,
		UserID:    opts.DoerID,
		ReleaseID: opts.ReleaseID,
	}

	_, err := db.GetEngine(ctx).Where("original_author_id = 0").Delete(reaction)
	return err
}

// ReleaseReactionList represents list of release reactions
type ReleaseReactionList []*ReleaseReaction

// HasUser check if user has reacted
func (list ReleaseReactionList) HasUser(userID int64) bool {
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
func (list ReleaseReactionList) GroupByType() map[string]ReleaseReactionList {
	reactions := make(map[string]ReleaseReactionList)
	for _, reaction := range list {
		reactions[reaction.Type] = append(reactions[reaction.Type], reaction)
	}
	return reactions
}

func (list ReleaseReactionList) getUserIDs() []int64 {
	return container.FilterSlice(list, func(reaction *ReleaseReaction) (int64, bool) {
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
func (list ReleaseReactionList) LoadUsers(ctx context.Context, repo *Repository) ([]*user_model.User, error) {
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
func (list ReleaseReactionList) GetFirstUsers() string {
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
func (list ReleaseReactionList) GetMoreUserCount() int {
	if len(list) <= setting.UI.ReactionMaxUserNum {
		return 0
	}
	return len(list) - setting.UI.ReactionMaxUserNum
}
