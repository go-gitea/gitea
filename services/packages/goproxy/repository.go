// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package goproxy

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	auth_model "gitea.dev/models/auth"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

var (
	ErrNotFound       = util.NewNotExistErrorf("module or version not found")
	ErrInvalidVersion = util.NewInvalidArgumentErrorf("invalid module version")
	ErrGoModNotFound  = util.NewNotExistErrorf("go.mod not found")
	ErrGoModMismatch  = util.NewInvalidArgumentErrorf("go.mod module path does not match the requested module")
	ErrGoModTooLarge  = util.NewInvalidArgumentErrorf("go.mod is too large")
)

const maxGoModFileSize = 16 * 1024 * 1024

// Repository is a Gitea repository that can be served as a Go module.
type Repository struct {
	Repo       *repo_model.Repository
	Subdir     string
	ModulePath string
}

// Version is a Go module version resolved from a repository tag.
type Version struct {
	Repository
	Version  string
	TagName  string
	CommitID string
	Time     time.Time
}

// ResolveRepository resolves a module path to a repository hosted on this Gitea instance.
// The second return value reports whether the path looked like a local Gitea module path.
func ResolveRepository(ctx context.Context, modulePath string) (*Repository, bool, error) {
	ownerName, repoName, subdir, ok := splitLocalModulePath(modulePath)
	if !ok {
		return nil, false, nil
	}

	repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, ownerName, repoName)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			return nil, true, nil
		}
		return nil, true, err
	}

	return &Repository{
		Repo:       repo,
		Subdir:     subdir,
		ModulePath: modulePath,
	}, true, nil
}

func splitLocalModulePath(modulePath string) (ownerName, repoName, subdir string, ok bool) {
	appURL, err := url.Parse(setting.AppURL)
	if err != nil {
		return "", "", "", false
	}

	parts := strings.Split(modulePath, "/")
	if len(parts) < 3 || !strings.EqualFold(parts[0], appURL.Hostname()) {
		return "", "", "", false
	}
	parts = parts[1:]

	if setting.AppSubURL != "" {
		prefix := strings.Split(strings.Trim(setting.AppSubURL, "/"), "/")
		if len(parts) < len(prefix) {
			return "", "", "", false
		}
		for i, p := range prefix {
			if parts[i] != p {
				return "", "", "", false
			}
		}
		parts = parts[len(prefix):]
	}

	if len(parts) < 2 {
		return "", "", "", false
	}

	return parts[0], parts[1], strings.Join(parts[2:], "/"), true
}

// CheckAccess verifies that doer may read the repository code.
func (r *Repository) CheckAccess(ctx context.Context, doer *user_model.User, scope auth_model.AccessTokenScope) error {
	if err := r.Repo.LoadOwner(ctx); err != nil {
		return err
	}

	if scope != "" {
		publicOnly, err := scope.PublicOnly()
		if err != nil {
			return err
		}
		if publicOnly && (r.Repo.IsPrivate || !r.Repo.Owner.Visibility.IsPublic()) {
			return ErrNotFound
		}
		hasScope, err := scope.HasScope(auth_model.AccessTokenScopeReadRepository)
		if err != nil {
			return err
		}
		if !hasScope {
			return ErrNotFound
		}
	}

	if !user_model.IsUserVisibleToViewer(ctx, r.Repo.Owner, doer) {
		return ErrNotFound
	}

	permission, err := access_model.GetDoerRepoPermission(ctx, r.Repo, doer)
	if err != nil {
		return err
	}
	if !permission.CanRead(unit.TypeCode) {
		return ErrNotFound
	}

	return nil
}

// ListVersions returns canonical module versions that exist for the repository module.
func (r *Repository) ListVersions(ctx context.Context) ([]string, error) {
	gitRepo, err := git.OpenRepository(ctx, r.Repo)
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	tags, _, err := gitRepo.GetTagInfos(ctx, 0, 0)
	if err != nil {
		return nil, err
	}

	versions := make(map[string]bool, len(tags))
	for _, tag := range tags {
		version, ok := canonicalVersion(tag.Name)
		if !ok {
			continue
		}
		if err := module.Check(r.ModulePath, version); err != nil {
			continue
		}
		if r.hasGoMod(ctx, gitRepo, tag.Object.String()) {
			versions[version] = true
		}
	}

	list := make([]string, 0, len(versions))
	for version := range versions {
		list = append(list, version)
	}
	semver.Sort(list)
	return list, nil
}

// ResolveVersion returns the version requested by the Go command, or the latest
// version when version is "latest".
func (r *Repository) ResolveVersion(ctx context.Context, version string) (*Version, error) {
	if version == "latest" {
		versions, err := r.ListVersions(ctx)
		if err != nil {
			return nil, err
		}
		if len(versions) == 0 {
			return nil, ErrNotFound
		}
		version = versions[len(versions)-1]
	}

	if !semver.IsValid(version) || !strings.HasPrefix(version, "v") {
		return nil, ErrInvalidVersion
	}
	if err := module.Check(r.ModulePath, version); err != nil {
		return nil, ErrInvalidVersion
	}

	gitRepo, err := git.OpenRepository(ctx, r.Repo)
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	tags, _, err := gitRepo.GetTagInfos(ctx, 0, 0)
	if err != nil {
		return nil, err
	}

	for _, tag := range tags {
		tagVersion, ok := canonicalVersion(tag.Name)
		if !ok || tagVersion != version {
			continue
		}
		if !r.hasGoMod(ctx, gitRepo, tag.Object.String()) {
			continue
		}

		return &Version{
			Repository: *r,
			Version:    version,
			TagName:    tag.Name,
			CommitID:   tag.Object.String(),
			Time:       tag.CommitDate.UTC(),
		}, nil
	}

	return nil, ErrNotFound
}

func (r *Repository) hasGoMod(ctx context.Context, gitRepo *git.Repository, commitID string) bool {
	commit, err := gitRepo.GetCommit(ctx, commitID)
	if err != nil {
		return false
	}
	entry, err := commit.GetTreeEntryByPath(ctx, gitRepo, path.Join(r.Subdir, "go.mod"))
	return err == nil && entry.IsRegular()
}

// GoMod returns and validates the go.mod file for the version.
func (v *Version) GoMod(ctx context.Context) ([]byte, error) {
	gitRepo, err := git.OpenRepository(ctx, v.Repo)
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	commit, err := gitRepo.GetCommit(ctx, v.CommitID)
	if err != nil {
		return nil, err
	}
	entry, err := commit.GetTreeEntryByPath(ctx, gitRepo, path.Join(v.Subdir, "go.mod"))
	if err != nil {
		return nil, ErrGoModNotFound
	}
	if !entry.IsRegular() {
		return nil, ErrGoModNotFound
	}

	blob := entry.Blob(gitRepo)
	if blob.Size(ctx) > maxGoModFileSize {
		return nil, ErrGoModTooLarge
	}

	data, err := blob.GetBlobContent(ctx, maxGoModFileSize)
	if err != nil {
		return nil, err
	}

	file, err := modfile.Parse("go.mod", []byte(data), nil)
	if err != nil {
		return nil, fmt.Errorf("parse go.mod: %w", err)
	}
	if file.Module == nil || file.Module.Mod.Path != v.ModulePath {
		return nil, ErrGoModMismatch
	}

	return []byte(data), nil
}

func canonicalVersion(tagName string) (string, bool) {
	if strings.HasPrefix(tagName, "v") && semver.IsValid(tagName) {
		return semver.Canonical(tagName), true
	}
	if !strings.HasPrefix(tagName, "v") && semver.IsValid("v"+tagName) {
		return semver.Canonical("v" + tagName), true
	}
	return "", false
}
