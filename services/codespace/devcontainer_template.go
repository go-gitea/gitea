// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"strings"
	"time"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
)

var ErrDevContainerTemplateNotFound = errors.New("codespace Dev Container template not found")

type DevContainerTemplateUpsertOptions struct {
	UserID  int64
	ID      int64
	Name    string
	Content string
}

type DevContainerTemplateDeleteOptions struct {
	UserID int64
	ID     int64
}

func listVisibleDevContainerTemplates(ctx context.Context, userID int64) ([]*codespace_model.DevContainerTemplate, error) {
	var templates []*codespace_model.DevContainerTemplate
	if err := db.GetEngine(ctx).
		In("user_id", []int64{0, userID}).
		Asc("user_id", "name", "id").
		Find(&templates); err != nil {
		return nil, err
	}
	return templates, nil
}

func ListDevContainerTemplates(ctx context.Context, userID int64) ([]*codespace_model.DevContainerTemplate, error) {
	var templates []*codespace_model.DevContainerTemplate
	if err := db.GetEngine(ctx).Where("user_id = ?", userID).Asc("name", "id").Find(&templates); err != nil {
		return nil, err
	}
	return templates, nil
}

func UpsertDevContainerTemplate(ctx context.Context, opts DevContainerTemplateUpsertOptions) error {
	name := strings.TrimSpace(opts.Name)
	content := strings.TrimSpace(opts.Content)
	if name == "" {
		return errors.New("Dev Container template name is required")
	}
	if len(name) > 255 {
		return errors.New("Dev Container template name is too long")
	}
	template := &codespace_model.DevContainerTemplate{Name: name, Content: content}
	if _, err := loadTemplateDevContainer(template); err != nil {
		return err
	}
	now := time.Now().Unix()
	return db.WithTx(ctx, func(ctx context.Context) error {
		if opts.ID > 0 {
			existing := new(codespace_model.DevContainerTemplate)
			has, err := db.GetEngine(ctx).Where("id = ? AND user_id = ?", opts.ID, opts.UserID).Get(existing)
			if err != nil {
				return err
			}
			if !has {
				return ErrDevContainerTemplateNotFound
			}
			existing.Name = name
			existing.Content = content
			existing.UpdatedUnix = now
			_, err = db.GetEngine(ctx).ID(existing.ID).Cols("name", "content", "updated_unix").Update(existing)
			return err
		}
		template.UserID = opts.UserID
		template.CreatedUnix = now
		template.UpdatedUnix = now
		_, err := db.GetEngine(ctx).Insert(template)
		return err
	})
}

func DeleteDevContainerTemplate(ctx context.Context, opts DevContainerTemplateDeleteOptions) error {
	deleted, err := db.GetEngine(ctx).Where("id = ? AND user_id = ?", opts.ID, opts.UserID).Delete(new(codespace_model.DevContainerTemplate))
	if err != nil {
		return err
	}
	if deleted == 0 {
		return ErrDevContainerTemplateNotFound
	}
	return nil
}
