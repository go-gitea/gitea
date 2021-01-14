// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"

	"github.com/gobwas/glob"
)

// GenerateRepoOptions contains the template units to generate
type GenerateRepoOptions struct {
	Name             string
	Description      string
	Private          bool
	GitContent       bool
	Topics           bool
	GitHooks         bool
	Webhooks         bool
	Avatar           bool
	IssueLabels      bool
	BranchProtection bool
}

// IsValid checks whether at least one option is chosen for generation
func (gro GenerateRepoOptions) IsValid() bool {
	return gro.GitContent || gro.Topics || gro.GitHooks || gro.Webhooks || gro.Avatar || gro.IssueLabels || gro.BranchProtection // or other items as they are added
}

// GiteaTemplate holds information about a .gitea/template file
type GiteaTemplate struct {
	Path    string
	Content []byte

	globs []glob.Glob
}

// Globs parses the .gitea/template globs or returns them if they were already parsed
func (gt GiteaTemplate) Globs() []glob.Glob {
	if gt.globs != nil {
		return gt.globs
	}

	gt.globs = make([]glob.Glob, 0)
	lines := strings.Split(string(util.NormalizeEOL(gt.Content)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		g, err := glob.Compile(line, '/')
		if err != nil {
			log.Info("Invalid glob expression '%s' (skipped): %v", line, err)
			continue
		}
		gt.globs = append(gt.globs, g)
	}
	return gt.globs
}

// GenerateTopics generates topics from a template repository
func GenerateTopics(ctx DBContext, templateRepo, generateRepo *Repository) error {
	for _, topic := range templateRepo.Topics {
		if _, err := addTopicByNameToRepo(ctx.e, generateRepo.ID, topic); err != nil {
			return err
		}
	}
	return nil
}

// GenerateGitHooks generates git hooks from a template repository
func GenerateGitHooks(ctx DBContext, templateRepo, generateRepo *Repository) error {
	generateGitRepo, err := git.OpenRepository(generateRepo.RepoPath())
	if err != nil {
		return err
	}
	defer generateGitRepo.Close()

	templateGitRepo, err := git.OpenRepository(templateRepo.RepoPath())
	if err != nil {
		return err
	}
	defer templateGitRepo.Close()

	templateHooks, err := templateGitRepo.Hooks()
	if err != nil {
		return err
	}

	for _, templateHook := range templateHooks {
		generateHook, err := generateGitRepo.GetHook(templateHook.Name())
		if err != nil {
			return err
		}

		generateHook.Content = templateHook.Content
		if err := generateHook.Update(); err != nil {
			return err
		}
	}
	return nil
}

// GenerateWebhooks generates webhooks from a template repository
func GenerateWebhooks(ctx DBContext, templateRepo, generateRepo *Repository) error {
	templateWebhooks, err := GetWebhooksByRepoID(templateRepo.ID, ListOptions{})
	if err != nil {
		return err
	}

	for _, templateWebhook := range templateWebhooks {
		generateWebhook := &Webhook{
			RepoID:      generateRepo.ID,
			URL:         templateWebhook.URL,
			HTTPMethod:  templateWebhook.HTTPMethod,
			ContentType: templateWebhook.ContentType,
			Secret:      templateWebhook.Secret,
			HookEvent:   templateWebhook.HookEvent,
			IsActive:    templateWebhook.IsActive,
			Type:        templateWebhook.Type,
			OrgID:       templateWebhook.OrgID,
			Events:      templateWebhook.Events,
			Meta:        templateWebhook.Meta,
		}
		if err := createWebhook(ctx.e, generateWebhook); err != nil {
			return err
		}
	}
	return nil
}

// GenerateAvatar generates the avatar from a template repository
func GenerateAvatar(ctx DBContext, templateRepo, generateRepo *Repository) error {
	generateRepo.Avatar = strings.Replace(templateRepo.Avatar, strconv.FormatInt(templateRepo.ID, 10), strconv.FormatInt(generateRepo.ID, 10), 1)
	if _, err := storage.Copy(storage.RepoAvatars, generateRepo.CustomAvatarRelativePath(), storage.RepoAvatars, templateRepo.CustomAvatarRelativePath()); err != nil {
		return err
	}

	return updateRepositoryCols(ctx.e, generateRepo, "avatar")
}

// GenerateIssueLabels generates issue labels from a template repository
func GenerateIssueLabels(ctx DBContext, templateRepo, generateRepo *Repository) error {
	templateLabels, err := getLabelsByRepoID(ctx.e, templateRepo.ID, "", ListOptions{})
	if err != nil {
		return err
	}

	for _, templateLabel := range templateLabels {
		generateLabel := &Label{
			RepoID:      generateRepo.ID,
			Name:        templateLabel.Name,
			Description: templateLabel.Description,
			Color:       templateLabel.Color,
		}
		if err := newLabel(ctx.e, generateLabel); err != nil {
			return err
		}
	}
	return nil
}

// GenerateBranchProtection generates branch protection from a template repository
func GenerateBranchProtection(ctx DBContext, doer *User, templateRepo, generateRepo *Repository) error {
	branches, err := templateRepo.getProtectedBranches(ctx.e)
	if err != nil {
		return err
	}

	for _, branch := range branches {
		// Create the branches (other than default, which exists already)
		if !strings.EqualFold(generateRepo.DefaultBranch, branch.BranchName) {
			if err := git.Push(generateRepo.RepoPath(), git.PushOptions{
				Remote: generateRepo.RepoPath(),
				Branch: fmt.Sprintf("%s:%s%s", generateRepo.DefaultBranch, git.BranchPrefix, branch.BranchName),
				Env:    InternalPushingEnvironment(doer, generateRepo),
			}); err != nil {
				if git.IsErrPushOutOfDate(err) || git.IsErrPushRejected(err) {
					return err
				}
				return fmt.Errorf("push: %v", err)
			}
		}

		// Copy protections
		protectBranch := &ProtectedBranch{
			RepoID:                        generateRepo.ID,
			BranchName:                    branch.BranchName,
			CanPush:                       branch.CanPush,
			EnableStatusCheck:             branch.EnableStatusCheck,
			StatusCheckContexts:           branch.StatusCheckContexts,
			RequiredApprovals:             branch.RequiredApprovals,
			BlockOnRejectedReviews:        branch.BlockOnRejectedReviews,
			BlockOnOfficialReviewRequests: branch.BlockOnOfficialReviewRequests,
			BlockOnOutdatedBranch:         branch.BlockOnOutdatedBranch,
			DismissStaleApprovals:         branch.DismissStaleApprovals,
			RequireSignedCommits:          branch.RequireSignedCommits,
			ProtectedFilePatterns:         branch.ProtectedFilePatterns,
		}

		if err := updateProtectBranch(ctx.e, generateRepo, protectBranch, WhitelistOptions{}); err != nil {
			return err
		}
	}

	return nil
}
