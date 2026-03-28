// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cron

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/migrations"
	task_service "code.gitea.io/gitea/services/task"

	"github.com/google/go-github/v74/github"
	"golang.org/x/oauth2"
)

type GitHubRepoAutoImportConfig struct {
	BaseConfig
	GitHubUsername string
	GiteaOwner     string
	Token          string
	TokenFile      string
	Mirror         bool
	MirrorInterval string
	ImportPrivate  bool
	ImportArchived bool
}

func registerGitHubRepoAutoImportTask() {
	RegisterTaskFatal("github_repo_auto_import", &GitHubRepoAutoImportConfig{
		BaseConfig: BaseConfig{
			Enabled:    false,
			RunAtStart: false,
			Schedule:   "@every 30m",
		},
		Mirror:         true,
		MirrorInterval: "8h0m0s",
		ImportPrivate:  true,
		ImportArchived: true,
	}, func(ctx context.Context, _ *user_model.User, cfg Config) error {
		return runGitHubRepoAutoImport(ctx, cfg.(*GitHubRepoAutoImportConfig))
	})
}

func runGitHubRepoAutoImport(ctx context.Context, cfg *GitHubRepoAutoImportConfig) error {
	if strings.TrimSpace(cfg.GiteaOwner) == "" {
		return fmt.Errorf("cron.github_repo_auto_import.GITEA_OWNER must be set")
	}

	token, err := resolveGitHubRepoAutoImportToken(cfg.Token, cfg.TokenFile)
	if err != nil {
		return err
	}

	owner, err := user_model.GetUserByName(ctx, cfg.GiteaOwner)
	if err != nil {
		return fmt.Errorf("load Gitea owner %q: %w", cfg.GiteaOwner, err)
	}

	client := newGitHubRepoAutoImportClient(token)
	authUser, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("load authenticated GitHub user: %w", err)
	}

	githubLogin := authUser.GetLogin()
	if cfg.GitHubUsername != "" && !strings.EqualFold(cfg.GitHubUsername, githubLogin) {
		return fmt.Errorf("configured GitHub username %q does not match token owner %q", cfg.GitHubUsername, githubLogin)
	}

	opt := &github.RepositoryListByAuthenticatedUserOptions{
		Visibility:  "all",
		Affiliation: "owner",
		Sort:        "created",
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	imported := 0
	skipped := 0
	for {
		repos, resp, err := client.Repositories.ListByAuthenticatedUser(ctx, opt)
		if err != nil {
			return fmt.Errorf("list GitHub repositories: %w", err)
		}

		for _, ghRepo := range repos {
			if !shouldAutoImportGitHubRepo(cfg, ghRepo) {
				skipped++
				continue
			}

			repoName := ghRepo.GetName()
			if _, err := repo_model.GetRepositoryByOwnerAndName(ctx, owner.Name, repoName); err == nil {
				skipped++
				continue
			} else if !repo_model.IsErrRepoNotExist(err) {
				return fmt.Errorf("check existing repository %q: %w", repoName, err)
			}

			opts := base.MigrateOptions{
				CloneAddr:      ghRepo.GetCloneURL(),
				RepoName:       repoName,
				Mirror:         cfg.Mirror,
				MirrorInterval: cfg.MirrorInterval,
				Private:        ghRepo.GetPrivate(),
				Description:    ghRepo.GetDescription(),
				OriginalURL:    ghRepo.GetCloneURL(),
				GitServiceType: structs.GithubService,
				AuthUsername:   githubLogin,
				AuthToken:      token,
				Wiki:           true,
			}

			if err := task_service.MigrateRepository(ctx, owner, owner, opts); err != nil {
				if repo_model.IsErrRepoAlreadyExist(err) {
					skipped++
					continue
				}
				return fmt.Errorf("queue migration for %q: %w", repoName, err)
			}
			imported++
		}

		if resp == nil || resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	log.Info("GitHub repo auto-import completed for %s into %s: imported=%d skipped=%d", githubLogin, owner.Name, imported, skipped)
	return nil
}

func newGitHubRepoAutoImportClient(token string) *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := &http.Client{
		Transport: &oauth2.Transport{
			Base:   migrations.NewMigrationHTTPTransport(),
			Source: oauth2.ReuseTokenSource(nil, ts),
		},
	}
	return github.NewClient(httpClient)
}

func resolveGitHubRepoAutoImportToken(inlineToken, tokenFile string) (string, error) {
	if token := strings.TrimSpace(inlineToken); token != "" {
		return token, nil
	}
	if path := strings.TrimSpace(tokenFile); path != "" {
		content, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read GitHub token file %q: %w", path, err)
		}
		token := strings.TrimSpace(string(content))
		if token == "" {
			return "", fmt.Errorf("GitHub token file %q is empty", path)
		}
		return token, nil
	}
	return "", fmt.Errorf("cron.github_repo_auto_import requires TOKEN or TOKEN_FILE")
}

func shouldAutoImportGitHubRepo(cfg *GitHubRepoAutoImportConfig, repo *github.Repository) bool {
	if repo == nil || repo.GetName() == "" || repo.GetCloneURL() == "" {
		return false
	}
	if repo.GetPrivate() && !cfg.ImportPrivate {
		return false
	}
	if repo.GetArchived() && !cfg.ImportArchived {
		return false
	}
	return true
}
