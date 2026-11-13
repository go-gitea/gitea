// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"net/url"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

// EntityRef is a denormalized reference persisted at record time.
type EntityRef struct {
	Type ScopeType `json:"type"`
	ID   int64     `json:"id,omitempty"`
	Name string    `json:"name,omitempty"`
}

func (r EntityRef) DisplayName() string {
	if r.Name != "" {
		return r.Name
	}
	if r.Type == ScopeSystem {
		return "System"
	}
	return ""
}

func (r EntityRef) HomeLink() string {
	switch r.Type {
	case ScopeUser, ScopeOrganization:
		if r.Name == "" {
			return ""
		}
		return setting.AppSubURL + "/" + url.PathEscape(r.Name)
	case ScopeRepository:
		if r.Name == "" {
			return ""
		}
		return setting.AppSubURL + "/" + util.PathEscapeSegments(r.Name)
	default:
		return ""
	}
}

func (r EntityRef) HasLink() bool {
	return r.HomeLink() != "" && r.ID > 0
}
