// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"context"
	"fmt"

	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"

	"xorm.io/builder"
)

func GetGroupByIDAndCond(ctx context.Context, id int64, cond builder.Cond) (*Group, error) {
	group := new(Group)

	has, err := db.GetEngine(ctx).
		Where(cond.And(builder.Eq{"`repo_group`.id": id})).Get(group)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrGroupNotExist{ID: id}
	}
	return group, nil
}

func GetGroupByID(ctx context.Context, id int64) (*Group, error) {
	return GetGroupByIDAndCond(ctx, id, builder.Expr("1 = 1"))
}

func GetGroupByRepoID(ctx context.Context, repoID int64) (*Group, error) {
	group := new(Group)
	_, err := db.GetEngine(ctx).
		Join("INNER", "repository", "repository.group_id = repo_group.id").
		Where(builder.Eq{"repository.`id`": repoID}).
		Get(group)
	return group, err
}

type FindGroupsOptions struct {
	db.ListOptions
	OwnerID       int64
	ParentGroupID int64
	ActorID       int64
	Name          string
}

func (opts FindGroupsOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.OwnerID != 0 {
		cond = cond.And(builder.Eq{"owner_id": opts.OwnerID})
	}
	if opts.ParentGroupID >= 0 {
		cond = cond.And(builder.Eq{"parent_group_id": opts.ParentGroupID})
	}
	if opts.Name != "" {
		cond = cond.And(builder.Eq{"lower_name": opts.Name})
	}
	return cond
}

func FindGroups(ctx context.Context, opts *FindGroupsOptions) (RepoGroupList, error) {
	sess := db.GetEngine(ctx)
	if opts.Page > 0 {
		sess = db.SetSessionPagination(sess, opts)
	}
	sess = sess.Where(opts.ToConds())

	groups := make([]*Group, 0, 10)
	return groups, sess.
		Asc("repo_group.sort_order").
		Find(&groups)
}

func findGroupsByCond(ctx context.Context, opts *FindGroupsOptions, cond builder.Cond) db.Engine {
	if opts.Page <= 0 {
		opts.Page = 1
	}

	sess := db.GetEngine(ctx).Where(cond.And(opts.ToConds()))
	if opts.PageSize > 0 {
		sess = sess.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	return sess.Asc("sort_order")
}

func FindGroupsByCond(ctx context.Context, opts *FindGroupsOptions, cond builder.Cond) (RepoGroupList, error) {
	defaultSize := 50
	if opts.PageSize > 0 {
		defaultSize = opts.PageSize
	}
	sess := findGroupsByCond(ctx, opts, cond)
	groups := make([]*Group, 0, defaultSize)
	if err := sess.Find(&groups); err != nil {
		return nil, err
	}
	return groups, nil
}

func CountGroups(ctx context.Context, opts *FindGroupsOptions) (int64, error) {
	return db.GetEngine(ctx).Where(opts.ToConds()).Count(new(Group))
}

func UpdateGroupOwnerName(ctx context.Context, oldUser, newUser string) error {
	if _, err := db.GetEngine(ctx).Exec("UPDATE `repo_group` SET owner_name=? WHERE owner_name=?", newUser, oldUser); err != nil {
		return fmt.Errorf("change group owner name: %w", err)
	}
	return nil
}

func UpdateGroup(ctx context.Context, group *Group) error {
	sess := db.GetEngine(ctx)
	_, err := sess.Table(group.TableName()).ID(group.ID).Update(group)
	return err
}

func GetOwnerByGroupID(ctx context.Context, groupID int64) (*user_model.User, error) {
	e := db.GetEngine(ctx)
	tableName := "repo_group"
	user := new(user_model.User)
	has, err := e.Join("INNER", tableName, fmt.Sprintf("`%s`.owner_id = `user`.`id`", tableName)).
		Where(builder.Eq{fmt.Sprintf("`%s`.id", tableName): groupID}).Get(user)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, user_model.ErrUserNotExist{}
	}
	return user, err
}
