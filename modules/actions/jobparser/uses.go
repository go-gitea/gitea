// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
)

// UsesKind enumerates the supported forms of a reusable workflow "uses:" value.
type UsesKind int

const (
	// UsesKindLocalSameRepo is "./.gitea/workflows/foo.yml" - a path inside the calling repository.
	UsesKindLocalSameRepo UsesKind = iota + 1
	// UsesKindLocalCrossRepo is "owner/repo/.gitea/workflows/foo.yml@ref" - a workflow in another repo on the same instance.
	UsesKindLocalCrossRepo
)

// UsesRef is the parsed form of a reusable workflow "uses:" value.
type UsesRef struct {
	Kind  UsesKind
	Owner string // empty for UsesKindLocalSameRepo
	Repo  string // empty for UsesKindLocalSameRepo
	Path  string // workflow file path inside the source repo
	Ref   string // git ref; empty for UsesKindLocalSameRepo
}

var (
	reLocalSameRepo  = regexp.MustCompile(`^\./\.(gitea|github)/workflows/([^@]+\.ya?ml)$`)
	reLocalCrossRepo = regexp.MustCompile(`^([-.\w]+)/([-.\w]+)/\.(gitea|github)/workflows/([^@]+\.ya?ml)@(.+)$`)
)

// ParseUses parses a reusable workflow "uses:" value.
// Only two forms are supported:
//   - "./.gitea/workflows/foo.yml"              (UsesKindLocalSameRepo, no @ref)
//   - "OWNER/REPO/.gitea/workflows/foo.yml@REF" (UsesKindLocalCrossRepo)
func ParseUses(s string) (*UsesRef, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("empty uses value")
	}

	if strings.HasPrefix(s, "./") {
		m := reLocalSameRepo.FindStringSubmatch(s)
		if m == nil {
			return nil, fmt.Errorf(`invalid local "uses:" %q (expect ./.gitea/workflows/<file>.yml)`, s)
		}
		p := fmt.Sprintf(".%s/workflows/%s", m[1], m[2])
		if path.Clean(p) != p {
			return nil, fmt.Errorf("invalid workflow path %q", s)
		}
		return &UsesRef{Kind: UsesKindLocalSameRepo, Path: p}, nil
	}

	m := reLocalCrossRepo.FindStringSubmatch(s)
	if m == nil {
		return nil, fmt.Errorf(`invalid cross-repo "uses:" %q (expect owner/repo/.gitea/workflows/<file>.yml@ref)`, s)
	}
	p := fmt.Sprintf(".%s/workflows/%s", m[3], m[4])
	if path.Clean(p) != p {
		return nil, fmt.Errorf("invalid workflow path %q", s)
	}
	return &UsesRef{
		Kind:  UsesKindLocalCrossRepo,
		Owner: m[1],
		Repo:  m[2],
		Path:  p,
		Ref:   m[5],
	}, nil
}
