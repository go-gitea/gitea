// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"

	"xorm.io/builder"
)

var (
	// ErrPermissionAuthorizationNotFound is returned when an authorization does not belong to the user.
	ErrPermissionAuthorizationNotFound = errors.New("codespace permission authorization not found")
	// ErrPermissionReductionInvalid is returned when a change would increase an approved permission.
	ErrPermissionReductionInvalid = errors.New("codespace permission can only be reduced")
)

// PermissionAuthorizationView contains one remembered authorization and its repository rules.
type PermissionAuthorizationView struct {
	ID               int64
	SourceRepository string
	Revoked          bool
	UpdatedUnix      int64
	CodespaceCount   int64
	Repositories     []PermissionRepositoryView
}

// PermissionRepositoryView contains one rule shown in user settings.
type PermissionRepositoryView struct {
	TargetRepoID    int64
	UnitType        unit.Type
	Repository      string
	UnitName        string
	RequestedMode   string
	GrantedMode     string
	CanReduceToRead bool
	CanRevoke       bool
}

// ListPermissionAuthorizations returns the current user's remembered Codespace grants.
func ListPermissionAuthorizations(ctx context.Context, userID int64) ([]*PermissionAuthorizationView, error) {
	var authorizations []*codespace_model.PermissionAuthorization
	if err := db.GetEngine(ctx).Where("user_id = ?", userID).Desc("updated_unix", "id").Find(&authorizations); err != nil {
		return nil, err
	}
	views := make([]*PermissionAuthorizationView, 0, len(authorizations))
	if len(authorizations) == 0 {
		return views, nil
	}
	authorizationQuery := builder.Select("id").From("codespace_permission_authorization").Where(builder.Eq{"user_id": userID})
	var rules []*codespace_model.PermissionRepository
	if err := db.GetEngine(ctx).In("authorization_id", authorizationQuery).Asc("target_repo_id", "unit_type").Find(&rules); err != nil {
		return nil, err
	}
	repoIDs := make([]int64, 0, len(authorizations)+len(rules))
	for _, authorization := range authorizations {
		repoIDs = append(repoIDs, authorization.SourceRepoID)
	}
	rulesByAuthorization := make(map[int64][]*codespace_model.PermissionRepository, len(authorizations))
	for _, rule := range rules {
		repoIDs = append(repoIDs, rule.TargetRepoID)
		rulesByAuthorization[rule.AuthorizationID] = append(rulesByAuthorization[rule.AuthorizationID], rule)
	}
	slices.Sort(repoIDs)
	repoIDs = slices.Compact(repoIDs)
	repositories := make(map[int64]*repo_model.Repository, len(repoIDs))
	for ids := range slices.Chunk(repoIDs, db.DefaultMaxInSize) {
		batch, err := repo_model.GetRepositoriesMapByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		if err := repo_model.RepositoryListOfMap(batch).LoadOwners(ctx); err != nil {
			return nil, err
		}
		maps.Copy(repositories, batch)
	}
	var counts []struct {
		PermissionAuthorizationID int64
		Count                     int64
	}
	if err := db.GetEngine(ctx).Table("codespace").In("permission_authorization_id", authorizationQuery).
		Select("permission_authorization_id, COUNT(*) AS count").GroupBy("permission_authorization_id").Find(&counts); err != nil {
		return nil, err
	}
	countsByAuthorization := make(map[int64]int64, len(counts))
	for _, count := range counts {
		countsByAuthorization[count.PermissionAuthorizationID] = count.Count
	}
	for _, authorization := range authorizations {
		sourceRepo := repositories[authorization.SourceRepoID]
		if sourceRepo == nil {
			continue
		}
		view := &PermissionAuthorizationView{
			ID: authorization.ID, SourceRepository: sourceRepo.FullName(),
			Revoked: authorization.RevokedUnix != 0, UpdatedUnix: authorization.UpdatedUnix,
		}
		if !view.Revoked {
			view.CodespaceCount = countsByAuthorization[authorization.ID]
		}
		for _, rule := range rulesByAuthorization[authorization.ID] {
			targetRepo := repositories[rule.TargetRepoID]
			if targetRepo == nil {
				continue
			}
			unitName := fmt.Sprintf("unit-%d", rule.UnitType)
			for name, unitType := range codespacePermissionUnits {
				if unitType == rule.UnitType {
					unitName = name
					break
				}
			}
			view.Repositories = append(view.Repositories, PermissionRepositoryView{
				TargetRepoID: rule.TargetRepoID, UnitType: rule.UnitType, Repository: targetRepo.FullName(), UnitName: unitName,
				RequestedMode: rule.RequestedMode.ToString(), GrantedMode: rule.GrantedMode.ToString(),
				CanReduceToRead: !view.Revoked && rule.GrantedMode == perm.AccessModeWrite,
				CanRevoke:       !view.Revoked && rule.GrantedMode != perm.AccessModeNone,
			})
		}
		views = append(views, view)
	}
	return views, nil
}

// RevokePermissionAuthorization revokes all grants in one remembered authorization.
func RevokePermissionAuthorization(ctx context.Context, userID, authorizationID int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		authorization, err := getUserPermissionAuthorization(ctx, userID, authorizationID)
		if err != nil {
			return err
		}
		if authorization.RevokedUnix != 0 {
			return nil
		}
		now := time.Now().Unix()
		updated, err := db.GetEngine(ctx).ID(authorization.ID).Where("revoked_unix = 0").Cols("revoked_unix", "updated_unix").Update(&codespace_model.PermissionAuthorization{RevokedUnix: now, UpdatedUnix: now})
		if err != nil {
			return err
		}
		if updated != 1 {
			return ErrPermissionAuthorizationNotFound
		}
		return nil
	})
}

// ReducePermissionRepository lowers one rule to read or none.
func ReducePermissionRepository(ctx context.Context, userID, authorizationID, targetRepoID int64, unitType unit.Type, mode perm.AccessMode) error {
	if mode != perm.AccessModeNone && mode != perm.AccessModeRead {
		return ErrPermissionReductionInvalid
	}
	return db.WithTx(ctx, func(ctx context.Context) error {
		authorization, err := getUserPermissionAuthorization(ctx, userID, authorizationID)
		if err != nil {
			return err
		}
		if authorization.RevokedUnix != 0 {
			return ErrPermissionAuthorizationNotFound
		}
		rule := new(codespace_model.PermissionRepository)
		has, err := db.GetEngine(ctx).
			Where("authorization_id = ? AND target_repo_id = ? AND unit_type = ?", authorization.ID, targetRepoID, unitType).
			Get(rule)
		if err != nil {
			return err
		}
		if !has {
			return ErrPermissionAuthorizationNotFound
		}
		if mode > rule.GrantedMode || mode > rule.RequestedMode {
			return ErrPermissionReductionInvalid
		}
		if mode == rule.GrantedMode {
			return nil
		}
		updated, err := db.GetEngine(ctx).
			Where("authorization_id = ? AND target_repo_id = ? AND unit_type = ? AND granted_mode = ?", authorization.ID, targetRepoID, unitType, rule.GrantedMode).
			Cols("granted_mode").Update(&codespace_model.PermissionRepository{GrantedMode: mode})
		if err != nil {
			return err
		}
		if updated != 1 {
			return ErrPermissionReductionInvalid
		}
		now := time.Now().Unix()
		_, err = db.GetEngine(ctx).ID(authorization.ID).Cols("updated_unix").Update(&codespace_model.PermissionAuthorization{UpdatedUnix: now})
		return err
	})
}

func getUserPermissionAuthorization(ctx context.Context, userID, authorizationID int64) (*codespace_model.PermissionAuthorization, error) {
	authorization := new(codespace_model.PermissionAuthorization)
	has, err := db.GetEngine(ctx).ID(authorizationID).Where("user_id = ?", userID).Get(authorization)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrPermissionAuthorizationNotFound
	}
	return authorization, nil
}
