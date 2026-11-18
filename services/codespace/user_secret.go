// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/globallock"
	secret_module "gitea.dev/modules/secret"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	secret_service "gitea.dev/services/secrets"
)

const (
	userSecretCountLimit     = 100
	userSecretValueSizeLimit = 48 * 1024
)

var (
	ErrUserSecretNotFound     = errors.New("codespace user secret not found")
	ErrUserSecretNameInvalid  = errors.New("codespace user secret name is invalid")
	ErrUserSecretNameConflict = errors.New("codespace user secret name already exists")
	ErrUserSecretValueInvalid = errors.New("codespace user secret value is invalid")
	ErrUserSecretCountLimit   = errors.New("codespace user secret count limit exceeded")
)

// UserSecretView is the value-free representation used by user settings.
type UserSecretView struct {
	ID          int64
	Name        string
	CreatedUnix int64
	UpdatedUnix int64
}

// ListUserSecrets returns a user's secrets without reading their values.
func ListUserSecrets(ctx context.Context, userID int64) ([]UserSecretView, error) {
	var secrets []UserSecretView
	err := db.GetEngine(ctx).Table(new(codespace_model.UserSecret)).Where("user_id = ?", userID).
		Cols("id", "name", "created_unix", "updated_unix").Asc("name").Find(&secrets)
	return secrets, err
}

// CreateUserSecret stores a reusable value owned by one user.
func CreateUserSecret(ctx context.Context, user *user_model.User, name, value string) error {
	if user == nil || user.ID <= 0 {
		return util.NewInvalidArgumentErrorf("user is required")
	}
	name = strings.ToUpper(strings.TrimSpace(name))
	if err := validateUserSecretName(name); err != nil {
		return err
	}
	if err := validateUserSecretValue(value); err != nil {
		return err
	}
	encrypted, err := secret_module.EncryptSecret(setting.SecretKey, value)
	if err != nil {
		return err
	}
	return globallock.LockAndDo(ctx, codespaceUserRelationLockKey(user.ID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			currentUser, err := user_model.GetUserByID(ctx, user.ID)
			if err != nil {
				return err
			}
			if currentUser.Type != user_model.UserTypeIndividual {
				return util.NewInvalidArgumentErrorf("individual user is required")
			}
			has, err := db.GetEngine(ctx).Where("user_id = ? AND name = ?", user.ID, name).Exist(new(codespace_model.UserSecret))
			if err != nil {
				return err
			}
			if has {
				return ErrUserSecretNameConflict
			}
			count, err := db.GetEngine(ctx).Where("user_id = ?", user.ID).Count(new(codespace_model.UserSecret))
			if err != nil {
				return err
			}
			if count >= userSecretCountLimit {
				return ErrUserSecretCountLimit
			}
			now := time.Now().Unix()
			return db.Insert(ctx, &codespace_model.UserSecret{
				UserID: user.ID, Name: name, DataEncrypted: encrypted,
				CreatedUnix: now, UpdatedUnix: now,
			})
		})
	})
}

// UpdateUserSecretValue replaces a user's saved value.
func UpdateUserSecretValue(ctx context.Context, userID, secretID int64, value string) error {
	if err := validateUserSecretValue(value); err != nil {
		return err
	}
	encrypted, err := secret_module.EncryptSecret(setting.SecretKey, value)
	if err != nil {
		return err
	}
	return globallock.LockAndDo(ctx, codespaceUserRelationLockKey(userID), func(ctx context.Context) error {
		affected, err := db.GetEngine(ctx).ID(secretID).Where("user_id = ?", userID).
			Cols("data_encrypted", "updated_unix").Update(&codespace_model.UserSecret{
			DataEncrypted: encrypted, UpdatedUnix: time.Now().Unix(),
		})
		if err == nil && affected == 0 {
			return ErrUserSecretNotFound
		}
		return err
	})
}

// DeleteUserSecret removes a user's saved value.
func DeleteUserSecret(ctx context.Context, userID, secretID int64) error {
	return globallock.LockAndDo(ctx, codespaceUserRelationLockKey(userID), func(ctx context.Context) error {
		affected, err := db.GetEngine(ctx).ID(secretID).Where("user_id = ?", userID).Delete(new(codespace_model.UserSecret))
		if err == nil && affected == 0 {
			return ErrUserSecretNotFound
		}
		return err
	})
}

func listUserSecretNames(ctx context.Context, userID int64) ([]string, error) {
	var names []string
	err := db.GetEngine(ctx).Table(new(codespace_model.UserSecret)).Where("user_id = ?", userID).Cols("name").Asc("name").Find(&names)
	return names, err
}

func validateUserSecretValue(value string) error {
	if value == "" || len(value) > userSecretValueSizeLimit || !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("%w: %w", ErrUserSecretValueInvalid, util.ErrInvalidArgument)
	}
	return nil
}

func validateUserSecretName(name string) error {
	if err := secret_service.ValidateName(name); err != nil {
		return fmt.Errorf("%w: %v", ErrUserSecretNameInvalid, err)
	}
	if strings.HasPrefix(name, "CODESPACE_") || name == "HOME" || name == "USER" || name == "LOGNAME" || name == "SHELL" || name == "PATH" || name == "TERM" || name == "COLORTERM" {
		return fmt.Errorf("%w: reserved name", ErrUserSecretNameInvalid)
	}
	return nil
}
