// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package usergroup

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/validation"

	"xorm.io/builder"
)

// UserGroup represents a site-wide group.
type UserGroup struct {
	ID          int64              `xorm:"pk autoincr"`
	Name        string             `xorm:"NOT NULL"`
	LowerName   string             `xorm:"INDEX NOT NULL"`
	Slug        string             `xorm:"UNIQUE NOT NULL"`
	Description string             `xorm:"TEXT NOT NULL"`
	ParentID    int64              `xorm:"INDEX"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

const (
	maxUserGroupNameRunes = 255
	maxUserGroupSlugRunes = 255
)

var userGroupSlugSeparatorPattern = regexp.MustCompile(`-+`)

// ValidateUserGroupName validates a user group display name.
func ValidateUserGroupName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return util.NewInvalidArgumentErrorf("empty user group name")
	}
	if utf8.RuneCountInString(trimmed) > maxUserGroupNameRunes {
		return util.NewInvalidArgumentErrorf("user group name exceeds %d characters", maxUserGroupNameRunes)
	}
	if strings.ContainsAny(trimmed, `/\\`) {
		return util.NewInvalidArgumentErrorf("user group name cannot contain slash or backslash")
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) {
			return util.NewInvalidArgumentErrorf("user group name cannot contain control characters")
		}
	}
	return nil
}

// NormalizeUserGroupSlug derives a lowercase slug from input.
func NormalizeUserGroupSlug(slug string) string {
	slug = strings.TrimSpace(strings.ToLower(slug))
	if slug == "" {
		return ""
	}
	b := strings.Builder{}
	b.Grow(len(slug))
	for _, r := range slug {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_' || r == '.':
			b.WriteByte('-')
		default:
			b.WriteByte('-')
		}
	}
	normalized := userGroupSlugSeparatorPattern.ReplaceAllString(b.String(), "-")
	return strings.Trim(normalized, "-")
}

// ValidateUserGroupSlug validates a stable user group slug.
func ValidateUserGroupSlug(slug string) error {
	trimmed := strings.TrimSpace(slug)
	if trimmed == "" {
		return util.NewInvalidArgumentErrorf("empty user group slug")
	}
	if trimmed != strings.ToLower(trimmed) {
		return util.NewInvalidArgumentErrorf("user group slug must be lowercase")
	}
	if utf8.RuneCountInString(trimmed) > maxUserGroupSlugRunes {
		return util.NewInvalidArgumentErrorf("user group slug exceeds %d characters", maxUserGroupSlugRunes)
	}
	if !validation.IsValidBadgeSlug(trimmed) {
		return util.NewInvalidArgumentErrorf("invalid user group slug")
	}
	return nil
}

func init() {
	db.RegisterModel(new(UserGroup))
}

// ErrUserGroupAlreadyExist represents a "UserGroupAlreadyExist" kind of error.
type ErrUserGroupAlreadyExist struct {
	Slug string
}

// IsErrUserGroupAlreadyExist checks if an error is a ErrUserGroupAlreadyExist.
func IsErrUserGroupAlreadyExist(err error) bool {
	_, ok := err.(ErrUserGroupAlreadyExist)
	return ok
}

func (err ErrUserGroupAlreadyExist) Error() string {
	return fmt.Sprintf("user group already exists [slug: %s]", err.Slug)
}

func (err ErrUserGroupAlreadyExist) Unwrap() error {
	return util.ErrAlreadyExist
}

// ErrUserGroupNotExist represents a "UserGroupNotExist" error.
type ErrUserGroupNotExist struct {
	GroupID int64
	Name    string
}

// IsErrUserGroupNotExist checks if an error is a ErrUserGroupNotExist.
func IsErrUserGroupNotExist(err error) bool {
	_, ok := err.(ErrUserGroupNotExist)
	return ok
}

func (err ErrUserGroupNotExist) Error() string {
	if err.Name != "" {
		return fmt.Sprintf("user group does not exist [name: %s]", err.Name)
	}
	return fmt.Sprintf("user group does not exist [group_id: %d]", err.GroupID)
}

func (err ErrUserGroupNotExist) Unwrap() error {
	return util.ErrNotExist
}

// ErrUserGroupHasChildren represents a "UserGroupHasChildren" error.
type ErrUserGroupHasChildren struct {
	GroupID int64
}

// IsErrUserGroupHasChildren checks if an error is a ErrUserGroupHasChildren.
func IsErrUserGroupHasChildren(err error) bool {
	_, ok := err.(ErrUserGroupHasChildren)
	return ok
}

func (err ErrUserGroupHasChildren) Error() string {
	return fmt.Sprintf("user group has child groups [group_id: %d]", err.GroupID)
}

func (err ErrUserGroupHasChildren) Unwrap() error {
	return util.ErrInvalidArgument
}

// ErrUserGroupCircularReference represents a "UserGroupCircularReference" error.
type ErrUserGroupCircularReference struct {
	GroupID  int64
	ParentID int64
}

// IsErrUserGroupCircularReference checks if an error is a ErrUserGroupCircularReference.
func IsErrUserGroupCircularReference(err error) bool {
	_, ok := err.(ErrUserGroupCircularReference)
	return ok
}

func (err ErrUserGroupCircularReference) Error() string {
	return fmt.Sprintf("user group parent causes circular reference [group_id: %d, parent_id: %d]", err.GroupID, err.ParentID)
}

func (err ErrUserGroupCircularReference) Unwrap() error {
	return util.ErrInvalidArgument
}

// GetUserGroupByID returns user group by ID.
func GetUserGroupByID(ctx context.Context, groupID int64) (*UserGroup, error) {
	g, exist, err := db.Get[UserGroup](ctx, builder.Eq{"id": groupID})
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, ErrUserGroupNotExist{GroupID: groupID}
	}
	return g, nil
}

// GetUserGroupBySlug returns user group by slug.
func GetUserGroupBySlug(ctx context.Context, slug string) (*UserGroup, error) {
	g, exist, err := db.Get[UserGroup](ctx, builder.Eq{"slug": strings.ToLower(slug)})
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, ErrUserGroupNotExist{Name: slug}
	}
	return g, nil
}

// GetUserGroupsByIDs returns user groups mapped by ID.
func GetUserGroupsByIDs(ctx context.Context, groupIDs []int64) (map[int64]*UserGroup, error) {
	groups := make(map[int64]*UserGroup, len(groupIDs))
	if len(groupIDs) == 0 {
		return groups, nil
	}
	return groups, db.GetEngine(ctx).In("id", groupIDs).Find(&groups)
}

// GetUserGroupFullPaths returns a map of group ID → full display path (e.g. "Engineering / Backend / API")
// for the given group IDs. It loads all groups once to resolve ancestor names.
func GetUserGroupFullPaths(ctx context.Context, groupIDs []int64) (map[int64]string, error) {
	result := make(map[int64]string, len(groupIDs))
	if len(groupIDs) == 0 {
		return result, nil
	}

	// Load every group so we can resolve parent names without extra queries.
	var all []*UserGroup
	if err := db.GetEngine(ctx).Find(&all); err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(all))
	parents := make(map[int64]int64, len(all))
	for _, g := range all {
		names[g.ID] = g.Name
		parents[g.ID] = g.ParentID
	}

	// Build paths with memoisation to avoid redundant work on shared ancestors.
	cache := make(map[int64]string, len(all))
	var buildPath func(id int64) string
	buildPath = func(id int64) string {
		if p, ok := cache[id]; ok {
			return p
		}
		name := names[id]
		if parentID := parents[id]; parentID != 0 {
			cache[id] = buildPath(parentID) + " / " + name
		} else {
			cache[id] = name
		}
		return cache[id]
	}

	for _, id := range groupIDs {
		result[id] = buildPath(id)
	}
	return result, nil
}

// SearchUserGroupOptions holds search options for user groups.
type SearchUserGroupOptions struct {
	db.ListOptions
	Keyword string
}

// SearchUserGroups searches user groups.
func SearchUserGroups(ctx context.Context, opts *SearchUserGroupOptions) ([]*UserGroup, int64, error) {
	opts.SetDefaultValues()
	sess := db.GetEngine(ctx)

	cond := builder.NewCond()
	if opts.Keyword != "" {
		cond = cond.And(builder.Like{"lower_name", strings.ToLower(opts.Keyword)})
	}

	sess = db.SetSessionPagination(sess, &opts.ListOptions)
	groups := make([]*UserGroup, 0, opts.PageSize)
	count, err := sess.Where(cond).OrderBy("lower_name").FindAndCount(&groups)
	if err != nil {
		return nil, 0, err
	}
	return groups, count, nil
}

// CreateUserGroup creates a new user group.
func CreateUserGroup(ctx context.Context, group *UserGroup) error {
	group.Name = strings.TrimSpace(group.Name)
	group.Slug = NormalizeUserGroupSlug(util.Iif(group.Slug != "", group.Slug, group.Name))
	group.Description = strings.TrimSpace(group.Description)
	if err := ValidateUserGroupName(group.Name); err != nil {
		return err
	}
	if err := ValidateUserGroupSlug(group.Slug); err != nil {
		return err
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		group.LowerName = strings.ToLower(group.Name)
		if err := validateUserGroupParent(ctx, 0, group.ParentID); err != nil {
			return err
		}

		has, err := db.Exist[UserGroup](ctx, builder.Eq{"slug": group.Slug})
		if err != nil {
			return err
		} else if has {
			return ErrUserGroupAlreadyExist{Slug: group.Slug}
		}

		now := timeutil.TimeStampNow()
		group.CreatedUnix = now
		group.UpdatedUnix = now
		return db.Insert(ctx, group)
	})
}

// UpdateUserGroup updates an existing user group.
func UpdateUserGroup(ctx context.Context, group *UserGroup) error {
	if group.ID == 0 {
		return util.NewInvalidArgumentErrorf("missing group id")
	}
	group.Name = strings.TrimSpace(group.Name)
	group.Slug = NormalizeUserGroupSlug(util.Iif(group.Slug != "", group.Slug, group.Name))
	group.Description = strings.TrimSpace(group.Description)
	if err := ValidateUserGroupName(group.Name); err != nil {
		return err
	}
	if err := ValidateUserGroupSlug(group.Slug); err != nil {
		return err
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		group.LowerName = strings.ToLower(group.Name)
		if err := validateUserGroupParent(ctx, group.ID, group.ParentID); err != nil {
			return err
		}

		has, err := db.Exist[UserGroup](ctx, builder.Eq{"slug": group.Slug}.And(builder.Neq{"id": group.ID}))
		if err != nil {
			return err
		} else if has {
			return ErrUserGroupAlreadyExist{Slug: group.Slug}
		}

		group.UpdatedUnix = timeutil.TimeStampNow()
		_, err = db.GetEngine(ctx).ID(group.ID).Cols("name", "lower_name", "slug", "description", "parent_id", "updated_unix").Update(group)
		return err
	})
}

func validateUserGroupParent(ctx context.Context, groupID, parentID int64) error {
	if parentID == 0 {
		return nil
	}

	parents, err := loadUserGroupParents(ctx)
	if err != nil {
		return err
	}

	if _, ok := parents[parentID]; !ok {
		return ErrUserGroupNotExist{GroupID: parentID}
	}

	if groupID == 0 {
		return nil
	}

	currentID := parentID
	for currentID != 0 {
		if currentID == groupID {
			return ErrUserGroupCircularReference{GroupID: groupID, ParentID: parentID}
		}
		currentID = parents[currentID]
	}
	return nil
}

type globalGroupParent struct {
	ID       int64 `xorm:"pk"`
	ParentID int64
}

func (globalGroupParent) TableName() string { return "user_group" }

func loadUserGroupParents(ctx context.Context) (map[int64]int64, error) {
	var groups []globalGroupParent
	if err := db.GetEngine(ctx).Cols("id", "parent_id").Find(&groups); err != nil {
		return nil, err
	}

	parents := make(map[int64]int64, len(groups))
	for _, group := range groups {
		parents[group.ID] = group.ParentID
	}
	return parents, nil
}

func buildUserGroupChildren(parents map[int64]int64) map[int64][]int64 {
	children := make(map[int64][]int64)
	for id, parentID := range parents {
		if parentID == 0 {
			continue
		}
		children[parentID] = append(children[parentID], id)
	}
	return children
}

func ExpandUserGroupIDsToDescendants(ctx context.Context, groupIDs []int64) ([]int64, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}

	parents, err := loadUserGroupParents(ctx)
	if err != nil {
		return nil, err
	}

	children := buildUserGroupChildren(parents)
	visited := container.Set[int64]{}

	queue := make([]int64, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		if _, ok := parents[groupID]; !ok {
			return nil, ErrUserGroupNotExist{GroupID: groupID}
		}
		if visited.Add(groupID) {
			queue = append(queue, groupID)
		}
	}

	for i := 0; i < len(queue); i++ {
		current := queue[i]
		for _, childID := range children[current] {
			if visited.Add(childID) {
				queue = append(queue, childID)
			}
		}
	}

	return visited.Values(), nil
}

// ExpandUserGroupIDsToAncestors returns the transitive closure of group IDs including all ancestor groups.
func ExpandUserGroupIDsToAncestors(ctx context.Context, groupIDs []int64) ([]int64, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}

	parents, err := loadUserGroupParents(ctx)
	if err != nil {
		return nil, err
	}

	visited := container.Set[int64]{}
	for _, groupID := range groupIDs {
		if _, ok := parents[groupID]; !ok {
			return nil, ErrUserGroupNotExist{GroupID: groupID}
		}
		currentID := groupID
		for currentID != 0 {
			if !visited.Add(currentID) {
				break
			}
			currentID = parents[currentID]
		}
	}

	return visited.Values(), nil
}
