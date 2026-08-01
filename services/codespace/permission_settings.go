// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
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
	RequestHash      string
	Revoked          bool
	CreatedUnix      int64
	UpdatedUnix      int64
	CodespaceCount   int64
	Repositories     []PermissionRepositoryView
}

// PermissionRepositoryView contains one rule shown in user settings.
type PermissionRepositoryView struct {
	ID              int64
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
	for _, authorization := range authorizations {
		sourceRepo, err := repo_model.GetRepositoryByID(ctx, authorization.SourceRepoID)
		if err != nil {
			if repo_model.IsErrRepoNotExist(err) {
				continue
			}
			return nil, err
		}
		if err := sourceRepo.LoadOwner(ctx); err != nil {
			return nil, err
		}
		var rules []*codespace_model.PermissionRepository
		if err := db.GetEngine(ctx).Where("authorization_id = ?", authorization.ID).Asc("target_repo_id", "unit_type").Find(&rules); err != nil {
			return nil, err
		}
		view := &PermissionAuthorizationView{
			ID: authorization.ID, SourceRepository: sourceRepo.FullName(), RequestHash: authorization.RequestHash,
			Revoked: authorization.RevokedUnix != 0, CreatedUnix: authorization.CreatedUnix, UpdatedUnix: authorization.UpdatedUnix,
		}
		if !view.Revoked {
			count, err := db.GetEngine(ctx).Where("permission_authorization_id = ?", authorization.ID).Count(new(codespace_model.Codespace))
			if err != nil {
				return nil, err
			}
			view.CodespaceCount = count
		}
		for _, rule := range rules {
			targetRepo, err := repo_model.GetRepositoryByID(ctx, rule.TargetRepoID)
			if err != nil {
				if repo_model.IsErrRepoNotExist(err) {
					continue
				}
				return nil, err
			}
			if err := targetRepo.LoadOwner(ctx); err != nil {
				return nil, err
			}
			unitName := fmt.Sprintf("unit-%d", rule.UnitType)
			for name, unitType := range codespacePermissionUnits {
				if unitType == rule.UnitType {
					unitName = name
					break
				}
			}
			view.Repositories = append(view.Repositories, PermissionRepositoryView{
				ID: rule.ID, Repository: targetRepo.FullName(), UnitName: unitName,
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
func ReducePermissionRepository(ctx context.Context, userID, authorizationID, ruleID int64, mode perm.AccessMode) error {
	if mode != perm.AccessModeNone && mode != perm.AccessModeRead {
		return ErrPermissionReductionInvalid
	}
	return db.WithTx(ctx, func(ctx context.Context) error {
		authorization, err := getUserPermissionAuthorization(ctx, userID, authorizationID)
		if err != nil || authorization.RevokedUnix != 0 {
			return ErrPermissionAuthorizationNotFound
		}
		rule := new(codespace_model.PermissionRepository)
		has, err := db.GetEngine(ctx).ID(ruleID).Where("authorization_id = ?", authorization.ID).Get(rule)
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
		updated, err := db.GetEngine(ctx).ID(rule.ID).Where("granted_mode = ?", rule.GrantedMode).Cols("granted_mode").Update(&codespace_model.PermissionRepository{GrantedMode: mode})
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
