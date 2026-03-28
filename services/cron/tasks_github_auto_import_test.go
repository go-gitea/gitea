// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cron

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	base "code.gitea.io/gitea/modules/migration"
	"github.com/google/go-github/v74/github"
)

func TestResolveGitHubRepoAutoImportToken(t *testing.T) {
	t.Run("inline token wins", func(t *testing.T) {
		token, err := resolveGitHubRepoAutoImportToken(" inline-token ", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token != "inline-token" {
			t.Fatalf("unexpected token %q", token)
		}
	})

	t.Run("token file fallback", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "token.txt")
		if err := os.WriteFile(path, []byte(" file-token \n"), 0o600); err != nil {
			t.Fatalf("write token file: %v", err)
		}

		token, err := resolveGitHubRepoAutoImportToken("", path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token != "file-token" {
			t.Fatalf("unexpected token %q", token)
		}
	})
}

func TestShouldAutoImportGitHubRepo(t *testing.T) {
	cfg := &GitHubRepoAutoImportConfig{
		ImportPrivate:  false,
		ImportArchived: false,
	}

	publicName := "public-repo"
	publicCloneURL := "https://github.com/example/public-repo.git"
	publicRepo := &github.Repository{
		Name:     &publicName,
		CloneURL: &publicCloneURL,
	}
	if !shouldAutoImportGitHubRepo(cfg, publicRepo) {
		t.Fatal("expected public repo to be imported")
	}

	privateName := "private-repo"
	privateCloneURL := "https://github.com/example/private-repo.git"
	privateValue := true
	privateRepo := &github.Repository{
		Name:     &privateName,
		CloneURL: &privateCloneURL,
		Private:  &privateValue,
	}
	if shouldAutoImportGitHubRepo(cfg, privateRepo) {
		t.Fatal("expected private repo to be skipped")
	}

	archivedName := "archived-repo"
	archivedCloneURL := "https://github.com/example/archived-repo.git"
	archivedValue := true
	archivedRepo := &github.Repository{
		Name:     &archivedName,
		CloneURL: &archivedCloneURL,
		Archived: &archivedValue,
	}
	if shouldAutoImportGitHubRepo(cfg, archivedRepo) {
		t.Fatal("expected archived repo to be skipped")
	}
}

func TestQueueGitHubRepoAutoImportMigrationsContinuesAfterQueueFailure(t *testing.T) {
	originalGetRepositoryByOwnerAndName := repoModelGetRepositoryByOwnerAndName
	originalMigrateRepository := gitHubRepoAutoImportMigrateRepository
	t.Cleanup(func() {
		repoModelGetRepositoryByOwnerAndName = originalGetRepositoryByOwnerAndName
		gitHubRepoAutoImportMigrateRepository = originalMigrateRepository
	})

	repoModelGetRepositoryByOwnerAndName = func(ctx context.Context, ownerName, repoName string) (*repo_model.Repository, error) {
		return nil, repo_model.ErrRepoNotExist{OwnerName: ownerName, RepoName: repoName}
	}

	attempted := make([]string, 0, 2)
	gitHubRepoAutoImportMigrateRepository = func(ctx context.Context, doer, owner *user_model.User, opts base.MigrateOptions) error {
		attempted = append(attempted, opts.RepoName)
		if opts.RepoName == "broken-repo" {
			return errors.New("invalid repo name")
		}
		return nil
	}

	cfg := &GitHubRepoAutoImportConfig{
		Mirror:         true,
		MirrorInterval: "8h0m0s",
		ImportPrivate:  true,
		ImportArchived: true,
	}
	owner := &user_model.User{Name: "owner"}

	brokenName := "broken-repo"
	brokenCloneURL := "https://github.com/example/broken-repo.git"
	okName := "ok-repo"
	okCloneURL := "https://github.com/example/ok-repo.git"
	repos := []*github.Repository{
		{Name: &brokenName, CloneURL: &brokenCloneURL},
		{Name: &okName, CloneURL: &okCloneURL},
	}

	imported, skipped, failed, failedRepos, err := queueGitHubRepoAutoImportMigrations(t.Context(), cfg, owner, "example", "token", repos)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if imported != 1 {
		t.Fatalf("unexpected imported count %d", imported)
	}
	if skipped != 0 {
		t.Fatalf("unexpected skipped count %d", skipped)
	}
	if failed != 1 {
		t.Fatalf("unexpected failed count %d", failed)
	}
	if len(failedRepos) != 1 || failedRepos[0] != "broken-repo" {
		t.Fatalf("unexpected failed repos %#v", failedRepos)
	}
	if len(attempted) != 2 || attempted[0] != "broken-repo" || attempted[1] != "ok-repo" {
		t.Fatalf("unexpected attempted repos %#v", attempted)
	}
}
