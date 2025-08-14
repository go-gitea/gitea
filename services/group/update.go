// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"context"
	"strings"

	"code.gitea.io/gitea/models/db"
	group_model "code.gitea.io/gitea/models/group"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/structs"
)

type UpdateOptions struct {
	Name        optional.Option[string]
	Description optional.Option[string]
	Visibility  optional.Option[structs.VisibleType]
}

func UpdateGroup(ctx context.Context, g *group_model.Group, opts *UpdateOptions) error {
	if opts.Name.Has() {
		g.Name = opts.Name.Value()
		g.LowerName = strings.ToLower(g.Name)
	}
	if opts.Description.Has() {
		g.Description = opts.Description.Value()
	}
	if opts.Visibility.Has() {
		g.Visibility = opts.Visibility.Value()
	}
	_, err := db.GetEngine(ctx).ID(g.ID).Update(g)
	return err
}
