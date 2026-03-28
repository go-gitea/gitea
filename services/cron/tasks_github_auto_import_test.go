// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cron

import (
	"os"
	"path/filepath"
	"testing"

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

	publicRepo := &github.Repository{
		Name:     github.Ptr("public-repo"),
		CloneURL: github.Ptr("https://github.com/example/public-repo.git"),
	}
	if !shouldAutoImportGitHubRepo(cfg, publicRepo) {
		t.Fatal("expected public repo to be imported")
	}

	privateRepo := &github.Repository{
		Name:     github.Ptr("private-repo"),
		CloneURL: github.Ptr("https://github.com/example/private-repo.git"),
		Private:  github.Ptr(true),
	}
	if shouldAutoImportGitHubRepo(cfg, privateRepo) {
		t.Fatal("expected private repo to be skipped")
	}

	archivedRepo := &github.Repository{
		Name:     github.Ptr("archived-repo"),
		CloneURL: github.Ptr("https://github.com/example/archived-repo.git"),
		Archived: github.Ptr(true),
	}
	if shouldAutoImportGitHubRepo(cfg, archivedRepo) {
		t.Fatal("expected archived repo to be skipped")
	}
}
