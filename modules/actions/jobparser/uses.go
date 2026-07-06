// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// UsesKind enumerates the supported forms of a reusable workflow "uses:" value.
type UsesKind int

const (
	// UsesKindLocalSameRepo is "./<dir>/foo.yml" - a path inside the calling repository.
	// For example: "./.gitea/workflows/foo.yml"
	UsesKindLocalSameRepo UsesKind = iota + 1
	// UsesKindLocalCrossRepo is "owner/repo/<dir>/foo.yml@ref" - a workflow in another repo on the same instance.
	// For example: "owner/repo/.gitea/workflows/foo.yml@ref"
	UsesKindLocalCrossRepo
)

// UsesRef is the parsed form of a reusable workflow "uses:" value.
type UsesRef struct {
	Kind    UsesKind
	Owner   string // empty for UsesKindLocalSameRepo
	Repo    string // empty for UsesKindLocalSameRepo
	GroupID int64  // empty for UsesKindLocalSameRepo
	Path    string // workflow file path inside the source repo
	Ref     string // git ref; empty for UsesKindLocalSameRepo
}

var (
	reLocalSameRepo  = regexp.MustCompile(`^\./([^@]+\.ya?ml)$`)
	reLocalCrossRepo = regexp.MustCompile(`^([-.\w]+)/(?:group/([-.\w]+)/)?([-.\w]+)/([^@]+\.ya?ml)@(.+)$`)
)

// ParseUses parses the SYNTAX of a reusable workflow "uses:" value into a UsesRef. Two forms are supported:
//   - "./<dir>/foo.yml"               (UsesKindLocalSameRepo, no @ref)
//   - "OWNER/group/[GROUP_ID/]REPO/<dir>/foo.yml@REF"  (UsesKindLocalCrossRepo)
//
// It deliberately does NOT validate that <dir> is an allowed workflow directory: the allowed directories are instance-configurable (WORKFLOW_DIRS / SCOPED_WORKFLOW_DIRS).
// The caller (services/actions.ResolveUses) enforces the directory allowlist. The returned Path is the cleaned, repo-relative file path.
func ParseUses(s string) (*UsesRef, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("empty uses value")
	}

	if strings.HasPrefix(s, "./") {
		m := reLocalSameRepo.FindStringSubmatch(s)
		if m == nil {
			return nil, fmt.Errorf(`invalid local "uses:" %q (expect ./<dir>/<file>.yml)`, s)
		}
		p := m[1]
		if path.Clean(p) != p {
			return nil, fmt.Errorf("invalid workflow path %q", s)
		}
		return &UsesRef{Kind: UsesKindLocalSameRepo, Path: p}, nil
	}

	m := reLocalCrossRepo.FindStringSubmatch(s)
	if m == nil {
		return nil, fmt.Errorf(`invalid cross-repo "uses:" %q (expect owner/repo/<dir>/<file>.yml@ref)`, s)
	}
	p := m[4]
	if path.Clean(p) != p {
		return nil, fmt.Errorf("invalid workflow path %q", s)
	}
	gid, _ := strconv.ParseInt(m[2], 10, 64)
	return &UsesRef{
		Kind:    UsesKindLocalCrossRepo,
		Owner:   m[1],
		Repo:    m[3],
		GroupID: gid,
		Path:    p,
		Ref:     m[5],
	}, nil
}
