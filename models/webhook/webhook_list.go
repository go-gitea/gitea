// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/util"
	"xorm.io/builder"
)

// CreateWebhooks creates multiple web hooks
func CreateWebhooks(ctx context.Context, ws []*Webhook) error {
	// xorm returns err "no element on slice when insert" for empty slices.
	if len(ws) == 0 {
		return nil
	}
	for i := 0; i < len(ws); i++ {
		ws[i].Type = strings.TrimSpace(ws[i].Type)
	}
	return db.Insert(ctx, ws)
}

// ListWebhookOptions are options to filter webhooks on ListWebhooksByOpts
type ListWebhookOptions struct {
	db.ListOptions
	RepoID   int64
	OwnerID  int64
	IsActive util.OptionalBool
}

func (opts *ListWebhookOptions) toCond() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID != 0 {
		cond = cond.And(builder.Eq{"webhook.repo_id": opts.RepoID})
	}
	if opts.OwnerID != 0 {
		cond = cond.And(builder.Eq{"webhook.owner_id": opts.OwnerID})
	}
	if !opts.IsActive.IsNone() {
		cond = cond.And(builder.Eq{"webhook.is_active": opts.IsActive.IsTrue()})
	}
	return cond
}

// ListWebhooksByOpts return webhooks based on options
func ListWebhooksByOpts(ctx context.Context, opts *ListWebhookOptions) ([]*Webhook, error) {
	sess := db.GetEngine(ctx).Where(opts.toCond())

	if opts.Page != 0 {
		sess = db.SetSessionPagination(sess, opts)
		webhooks := make([]*Webhook, 0, opts.PageSize)
		err := sess.Find(&webhooks)
		return webhooks, err
	}

	webhooks := make([]*Webhook, 0, 10)
	err := sess.Find(&webhooks)
	return webhooks, err
}

// CountWebhooksByOpts count webhooks based on options and ignore pagination
func CountWebhooksByOpts(ctx context.Context, opts *ListWebhookOptions) (int64, error) {
	return db.GetEngine(ctx).Where(opts.toCond()).Count(&Webhook{})
}
