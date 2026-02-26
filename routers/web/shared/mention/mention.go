// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mention

import (
	"context"

	"code.gitea.io/gitea/models/organization"
	user_model "code.gitea.io/gitea/models/user"
)

// Mention is the JSON structure returned by mention autocomplete endpoints.
type Mention struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Name     string `json:"name"`
	FullName string `json:"fullname"`
	Avatar   string `json:"avatar"`
}

// Collector builds a deduplicated list of Mention entries.
type Collector struct {
	seen   map[string]bool
	Result []Mention
}

// NewCollector creates a new Collector.
func NewCollector() *Collector {
	return &Collector{seen: make(map[string]bool)}
}

// AddUsers adds user mentions, skipping duplicates.
func (c *Collector) AddUsers(ctx context.Context, users []*user_model.User) {
	for _, u := range users {
		if !c.seen[u.Name] {
			c.seen[u.Name] = true
			c.Result = append(c.Result, Mention{
				Key:      u.Name + " " + u.FullName,
				Value:    u.Name,
				Name:     u.Name,
				FullName: u.FullName,
				Avatar:   u.AvatarLink(ctx),
			})
		}
	}
}

// AddMentionableTeams loads and adds team mentions for the given owner (if it's an org).
func (c *Collector) AddMentionableTeams(ctx context.Context, doer, owner *user_model.User) error {
	if doer == nil || !owner.IsOrganization() {
		return nil
	}

	org := organization.OrgFromUser(owner)
	isAdmin := doer.IsAdmin
	if !isAdmin {
		var err error
		isAdmin, err = org.IsOwnedBy(ctx, doer.ID)
		if err != nil {
			return err
		}
	}

	var teams []*organization.Team
	var err error
	if isAdmin {
		teams, err = org.LoadTeams(ctx)
	} else {
		teams, err = org.GetUserTeams(ctx, doer.ID)
	}
	if err != nil {
		return err
	}

	for _, team := range teams {
		key := owner.Name + "/" + team.Name
		if !c.seen[key] {
			c.seen[key] = true
			c.Result = append(c.Result, Mention{
				Key:    key,
				Value:  key,
				Name:   key,
				Avatar: owner.AvatarLink(ctx),
			})
		}
	}
	return nil
}

// ResultOrEmpty returns the collected mentions, or an empty slice if none.
func (c *Collector) ResultOrEmpty() []Mention {
	if c.Result == nil {
		return []Mention{}
	}
	return c.Result
}
