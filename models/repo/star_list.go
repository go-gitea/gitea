// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

type ErrStarListNotFound struct {
	Name string
	ID   int64
}

func (err ErrStarListNotFound) Error() string {
	if err.Name == "" {
		return fmt.Sprintf("A star list with the ID %d was not found", err.ID)
	}

	return fmt.Sprintf("A star list with the name %s was not found", err.Name)
}

// IsErrStarListNotFound  returns if the error is, that the star is not found
func IsErrStarListNotFound(err error) bool {
	_, ok := err.(ErrStarListNotFound)
	return ok
}

type ErrStarListExists struct {
	Name string
}

func (err ErrStarListExists) Error() string {
	return fmt.Sprintf("A star list with the name %s exists", err.Name)
}

// IsErrIssueMaxPinReached returns if the error is, that the star list exists
func IsErrStarListExists(err error) bool {
	_, ok := err.(ErrStarListExists)
	return ok
}

type StarList struct {
	ID              int64  `xorm:"pk autoincr"`
	UserID          int64  `xorm:"INDEX UNIQUE(name)"`
	Name            string `xorm:"INDEX UNIQUE(name)"`
	Description     string
	IsPrivate       bool
	RepositoryCount int64              `xorm:"-"`
	User            *user_model.User   `xorm:"-"`
	CreatedUnix     timeutil.TimeStamp `xorm:"INDEX created"`
	RepoIDs         *[]int64           `xorm:"-"`
}

type StarListRepos struct {
	ID          int64              `xorm:"pk autoincr"`
	StarListID  int64              `xorm:"INDEX UNIQUE(repo)"`
	RepoID      int64              `xorm:"INDEX UNIQUE(repo)"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
}

type StarListSlice []*StarList

func init() {
	db.RegisterModel(new(StarList))
	db.RegisterModel(new(StarListRepos))
}

// GetStarListByID returne the star list for the given ID.
// If the ID do not exists, it returns a ErrStarListNotFound error.
func GetStarListByID(ctx context.Context, id int64) (*StarList, error) {
	var starList StarList

	found, err := db.GetEngine(ctx).Table("star_list").ID(id).Get(&starList)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, ErrStarListNotFound{ID: id}
	}

	return &starList, nil
}

// GetStarListByID returne the star list of the given user with the given name.
// If the name do not exists, it returns a ErrStarListNotFound error.
func GetStarListByName(ctx context.Context, userID int64, name string) (*StarList, error) {
	var starList StarList

	found, err := db.GetEngine(ctx).Table("star_list").Where("user_id = ?", userID).And("LOWER(name) = ?", strings.ToLower(name)).Get(&starList)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, ErrStarListNotFound{Name: name}
	}

	return &starList, nil
}

// GetStarListsByUserID retruns all star lists for the given user
func GetStarListsByUserID(ctx context.Context, userID int64, includePrivate bool) (StarListSlice, error) {
	cond := builder.NewCond().And(builder.Eq{"user_id": userID})

	if !includePrivate {
		cond = cond.And(builder.Eq{"is_private": false})
	}

	starLists := make(StarListSlice, 0)
	err := db.GetEngine(ctx).Table("star_list").Where(cond).Asc("created_unix").Asc("id").Find(&starLists)
	if err != nil {
		return nil, err
	}

	return starLists, nil
}

// CreateStarLists creates a new star list
// It returns a ErrStarListExists if the user already have a star list with this name
func CreateStarList(ctx context.Context, userID int64, name, description string, isPrivate bool) (*StarList, error) {
	_, err := GetStarListByName(ctx, userID, name)
	if err != nil {
		if !IsErrStarListNotFound(err) {
			return nil, err
		}
	} else {
		return nil, ErrStarListExists{Name: name}
	}

	starList := StarList{
		UserID:      userID,
		Name:        name,
		Description: description,
		IsPrivate:   isPrivate,
	}

	_, err = db.GetEngine(ctx).Insert(starList)
	if err != nil {
		return nil, err
	}

	return &starList, nil
}

// DeleteStarListByID deletes the star list with the given ID
func DeleteStarListByID(ctx context.Context, id int64) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}

	_, err = db.GetEngine(ctx).Exec("DELETE FROM star_list_repos WHERE star_list_id = ?", id)
	if err != nil {
		return err
	}

	_, err = db.GetEngine(ctx).Exec("DELETE FROM star_list WHERE id = ?", id)
	if err != nil {
		return err
	}

	return committer.Commit()
}

// LoadRepositoryCount loads just the RepositoryCount.
// The count checks if how many repos in the list the actor is able to see.
func (starList *StarList) LoadRepositoryCount(ctx context.Context, actor *user_model.User) error {
	count, err := CountRepository(ctx, &SearchRepoOptions{Actor: actor, StarListID: starList.ID})
	if err != nil {
		return err
	}

	starList.RepositoryCount = count

	return nil
}

// LoadUser loads the User field
func (starList *StarList) LoadUser(ctx context.Context) error {
	user, err := user_model.GetUserByID(ctx, starList.UserID)
	if err != nil {
		return err
	}

	starList.User = user
	return nil
}

// LoadRepoIDs loads all repo ids which are in the list
func (starList *StarList) LoadRepoIDs(ctx context.Context) error {
	repoIDs := make([]int64, 0)
	err := db.GetEngine(ctx).Table("star_list_repos").Where("star_list_id = ?", starList.ID).Cols("repo_id").Find(&repoIDs)
	if err != nil {
		return err
	}
	starList.RepoIDs = &repoIDs
	return nil
}

// Retruns if the list contains the given repo id.
// This function needs the repo ids loaded to work.
func (starList *StarList) ContainsRepoID(repoID int64) bool {
	return slices.Contains(*starList.RepoIDs, repoID)
}

// AddRepo adds the given repo to the list
func (starList *StarList) AddRepo(ctx context.Context, repoID int64) error {
	err := starList.LoadRepoIDs(ctx)
	if err != nil {
		return err
	}

	if starList.ContainsRepoID(repoID) {
		return nil
	}

	err = StarRepo(ctx, starList.UserID, repoID, true)
	if err != nil {
		return err
	}

	starListRepo := StarListRepos{
		StarListID: starList.ID,
		RepoID:     repoID,
	}

	_, err = db.GetEngine(ctx).Insert(starListRepo)
	return err
}

// RemoveRepo removes the given repo from the list
func (starList *StarList) RemoveRepo(ctx context.Context, repoID int64) error {
	_, err := db.GetEngine(ctx).Exec("DELETE FROM star_list_repos WHERE star_list_id = ? AND repo_id = ?", starList.ID, repoID)
	return err
}

// EditData edits the star list and save it to the database
// It returns a ErrStarListExists if the user already have a star list with this name
func (starList *StarList) EditData(ctx context.Context, name, description string, isPrivate bool) error {
	if !strings.EqualFold(starList.Name, name) {
		_, err := GetStarListByName(ctx, starList.UserID, name)
		if err != nil {
			if !IsErrStarListNotFound(err) {
				return err
			}
		} else {
			return ErrStarListExists{Name: name}
		}
	}

	oldName := starList.Name
	oldDescription := starList.Description
	oldIsPrivate := starList.IsPrivate

	starList.Name = name
	starList.Description = description
	starList.IsPrivate = isPrivate

	_, err := db.GetEngine(ctx).Table("star_list").ID(starList.ID).Cols("name", "description", "is_private").Update(starList)
	if err != nil {
		starList.Name = oldName
		starList.Description = oldDescription
		starList.IsPrivate = oldIsPrivate

		return err
	}

	return nil
}

// HasAccess retruns if the given user has access to this star list
func (starList *StarList) HasAccess(user *user_model.User) bool {
	if !starList.IsPrivate {
		return true
	}

	if user == nil {
		return false
	}

	return starList.UserID == user.ID
}

// MustHaveAccess returns a ErrStarListNotFound if the given user has no access to the star list
func (starList *StarList) MustHaveAccess(user *user_model.User) error {
	if !starList.HasAccess(user) {
		return ErrStarListNotFound{ID: starList.ID, Name: starList.Name}
	}
	return nil
}

// Returns a Link to the star list.
// This function needs the user loaded to work.
func (starList *StarList) Link() string {
	return fmt.Sprintf("%s/-/starlist/%s", starList.User.HomeLink(), url.PathEscape(starList.Name))
}

// LoadUser calls LoadUser on all elements of the list
func (starLists StarListSlice) LoadUser(ctx context.Context) error {
	for _, list := range starLists {
		err := list.LoadUser(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// LoadRepositoryCount calls LoadRepositoryCount on all elements of the list
func (starLists StarListSlice) LoadRepositoryCount(ctx context.Context, actor *user_model.User) error {
	for _, list := range starLists {
		err := list.LoadRepositoryCount(ctx, actor)
		if err != nil {
			return err
		}
	}
	return nil
}

// LoadRepoIDs calls LoadRepoIDs on all elements of the list
func (starLists StarListSlice) LoadRepoIDs(ctx context.Context) error {
	for _, list := range starLists {
		err := list.LoadRepoIDs(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}
