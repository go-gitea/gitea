// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"

	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/util"
)

func validateSavedReply(title, content string) error {
	if util.IsEmptyString(title) {
		return user_model.ErrSavedReplyTitleEmpty
	}
	if util.IsEmptyString(content) {
		return user_model.ErrSavedReplyContentEmpty
	}
	return nil
}

func CreateSavedReply(ctx context.Context, user *user_model.User, title, content string) error {
	if err := validateSavedReply(title, content); err != nil {
		return err
	}

	return db.Insert(ctx, &user_model.SavedReply{
		UserID:  user.ID,
		Content: content,
		Title:   title,
	})
}

func UpdateSavedReply(ctx context.Context, user *user_model.User, id int64, title, content string) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := validateSavedReply(title, content); err != nil {
			return err
		}

		savedReply, err := user_model.GetSavedReply(ctx, id)
		if err != nil {
			return err
		}

		if savedReply.UserID != user.ID {
			return user_model.ErrSavedReplyDoesNotBelongToUser
		}

		return user_model.UpdateSavedReply(ctx, savedReply.ID, title, content)
	})
}

func DeleteSavedReply(ctx context.Context, user *user_model.User, id int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		savedReply, err := user_model.GetSavedReply(ctx, id)
		if err != nil {
			return err
		}
		if savedReply.UserID != user.ID {
			return user_model.ErrSavedReplyDoesNotBelongToUser
		}
		_, err = db.DeleteByID[user_model.SavedReply](ctx, savedReply.ID)
		return err
	})
}
