// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	"github.com/gobwas/glob"
	"github.com/unknwon/com"
)

// GenerateRepoOptions contains the template units to generate
type GenerateRepoOptions struct {
	Name        string
	Description string
	Private     bool
	GitContent  bool
	Topics      bool
	GitHooks    bool
	Webhooks    bool
	Avatar      bool
	IssueLabels bool
}

// IsValid checks whether at least one option is chosen for generation
func (gro GenerateRepoOptions) IsValid() bool {
	return gro.GitContent || gro.Topics || gro.GitHooks || gro.Webhooks || gro.Avatar || gro.IssueLabels // or other items as they are added
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

func checkGiteaTemplate(tmpDir string) (*GiteaTemplate, error) {
	gtPath := filepath.Join(tmpDir, ".gitea", "template")
	if _, err := os.Stat(gtPath); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	content, err := ioutil.ReadFile(gtPath)
	if err != nil {
		return nil, err
	}

	gt := &GiteaTemplate{
		Path:    gtPath,
		Content: content,
	}

	return gt, nil
}

func generateRepoCommit(e Engine, repo, templateRepo, generateRepo *Repository, tmpDir string) error {
	commitTimeStr := time.Now().Format(time.RFC3339)
	authorSig := repo.Owner.NewGitSig()

	// Because this may call hooks we should pass in the environment
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME="+authorSig.Name,
		"GIT_AUTHOR_EMAIL="+authorSig.Email,
		"GIT_AUTHOR_DATE="+commitTimeStr,
		"GIT_COMMITTER_NAME="+authorSig.Name,
		"GIT_COMMITTER_EMAIL="+authorSig.Email,
		"GIT_COMMITTER_DATE="+commitTimeStr,
	)

	// Clone to temporary path and do the init commit.
	templateRepoPath := templateRepo.repoPath(e)
	if err := git.Clone(templateRepoPath, tmpDir, git.CloneRepoOptions{
		Depth: 1,
	}); err != nil {
		return fmt.Errorf("git clone: %v", err)
	}

	if err := os.RemoveAll(path.Join(tmpDir, ".git")); err != nil {
		return fmt.Errorf("remove git dir: %v", err)
	}

	// Variable expansion
	gt, err := checkGiteaTemplate(tmpDir)
	if err != nil {
		return fmt.Errorf("checkGiteaTemplate: %v", err)
	}

	if gt != nil {
		if err := os.Remove(gt.Path); err != nil {
			return fmt.Errorf("remove .giteatemplate: %v", err)
		}

		// Avoid walking tree if there are no globs
		if len(gt.Globs()) > 0 {
			tmpDirSlash := strings.TrimSuffix(filepath.ToSlash(tmpDir), "/") + "/"
			if err := filepath.Walk(tmpDirSlash, func(path string, info os.FileInfo, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}

				if info.IsDir() {
					return nil
				}

				base := strings.TrimPrefix(filepath.ToSlash(path), tmpDirSlash)
				for _, g := range gt.Globs() {
					if g.Match(base) {
						content, err := ioutil.ReadFile(path)
						if err != nil {
							return err
						}

						if err := ioutil.WriteFile(path,
							[]byte(generateExpansion(string(content), templateRepo, generateRepo)),
							0644); err != nil {
							return err
						}
						break
					}
				}
				return nil
			}); err != nil {
				return err
			}
		}
	}

	if err := git.InitRepository(tmpDir, false); err != nil {
		return err
	}

	repoPath := repo.repoPath(e)
	if stdout, err := git.NewCommand("remote", "add", "origin", repoPath).
		SetDescription(fmt.Sprintf("generateRepoCommit (git remote add): %s to %s", templateRepoPath, tmpDir)).
		RunInDirWithEnv(tmpDir, env); err != nil {
		log.Error("Unable to add %v as remote origin to temporary repo to %s: stdout %s\nError: %v", repo, tmpDir, stdout, err)
		return fmt.Errorf("git remote add: %v", err)
	}

	return initRepoCommit(tmpDir, repo, repo.Owner)
}

// generateRepository initializes repository from template
func generateRepository(e Engine, repo, templateRepo, generateRepo *Repository) (err error) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "gitea-"+repo.Name)
	if err != nil {
		return fmt.Errorf("Failed to create temp dir for repository %s: %v", repo.repoPath(e), err)
	}

	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Error("RemoveAll: %v", err)
		}
	}()

	if err = generateRepoCommit(e, repo, templateRepo, generateRepo, tmpDir); err != nil {
		return fmt.Errorf("generateRepoCommit: %v", err)
	}

	// re-fetch repo
	if repo, err = getRepositoryByID(e, repo.ID); err != nil {
		return fmt.Errorf("getRepositoryByID: %v", err)
	}

	repo.DefaultBranch = "master"
	if err = updateRepository(e, repo, false); err != nil {
		return fmt.Errorf("updateRepository: %v", err)
	}

	return nil
}

// GenerateRepository generates a repository from a template
func GenerateRepository(ctx DBContext, doer, owner *User, templateRepo *Repository, opts GenerateRepoOptions) (_ *Repository, err error) {
	generateRepo := &Repository{
		OwnerID:       owner.ID,
		Owner:         owner,
		Name:          opts.Name,
		LowerName:     strings.ToLower(opts.Name),
		Description:   opts.Description,
		IsPrivate:     opts.Private,
		IsEmpty:       !opts.GitContent || templateRepo.IsEmpty,
		IsFsckEnabled: templateRepo.IsFsckEnabled,
		TemplateID:    templateRepo.ID,
	}

	if err = createRepository(ctx.e, doer, owner, generateRepo); err != nil {
		return nil, err
	}

	repoPath := RepoPath(owner.Name, generateRepo.Name)
	if err = checkInitRepository(repoPath); err != nil {
		return generateRepo, err
	}

	return generateRepo, nil
}

// GenerateGitContent generates git content from a template repository
func GenerateGitContent(ctx DBContext, templateRepo, generateRepo *Repository) error {
	if err := generateRepository(ctx.e, generateRepo, templateRepo, generateRepo); err != nil {
		return err
	}

	if err := generateRepo.updateSize(ctx.e); err != nil {
		return fmt.Errorf("failed to update size for repository: %v", err)
	}

	if err := copyLFS(ctx.e, generateRepo, templateRepo); err != nil {
		return fmt.Errorf("failed to copy LFS: %v", err)
	}
	return nil
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
	generateGitRepo, err := git.OpenRepository(generateRepo.repoPath(ctx.e))
	if err != nil {
		return err
	}
	defer generateGitRepo.Close()

	templateGitRepo, err := git.OpenRepository(templateRepo.repoPath(ctx.e))
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
	templateWebhooks, err := GetWebhooksByRepoID(templateRepo.ID)
	if err != nil {
		return err
	}

	for _, templateWebhook := range templateWebhooks {
		generateWebhook := &Webhook{
			RepoID:       generateRepo.ID,
			URL:          templateWebhook.URL,
			HTTPMethod:   templateWebhook.HTTPMethod,
			ContentType:  templateWebhook.ContentType,
			Secret:       templateWebhook.Secret,
			HookEvent:    templateWebhook.HookEvent,
			IsActive:     templateWebhook.IsActive,
			HookTaskType: templateWebhook.HookTaskType,
			OrgID:        templateWebhook.OrgID,
			Events:       templateWebhook.Events,
			Meta:         templateWebhook.Meta,
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
	if err := com.Copy(templateRepo.CustomAvatarPath(), generateRepo.CustomAvatarPath()); err != nil {
		return err
	}

	return updateRepositoryCols(ctx.e, generateRepo, "avatar")
}

// GenerateIssueLabels generates issue labels from a template repository
func GenerateIssueLabels(ctx DBContext, templateRepo, generateRepo *Repository) error {
	templateLabels, err := getLabelsByRepoID(ctx.e, templateRepo.ID, "")
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

func generateExpansion(src string, templateRepo, generateRepo *Repository) string {
	return os.Expand(src, func(key string) string {
		switch key {
		case "REPO_NAME":
			return generateRepo.Name
		case "TEMPLATE_NAME":
			return templateRepo.Name
		case "REPO_DESCRIPTION":
			return generateRepo.Description
		case "TEMPLATE_DESCRIPTION":
			return templateRepo.Description
		case "REPO_OWNER":
			return generateRepo.MustOwnerName()
		case "TEMPLATE_OWNER":
			return templateRepo.MustOwnerName()
		case "REPO_LINK":
			return generateRepo.Link()
		case "TEMPLATE_LINK":
			return templateRepo.Link()
		case "REPO_HTTPS_URL":
			return generateRepo.CloneLink().HTTPS
		case "TEMPLATE_HTTPS_URL":
			return templateRepo.CloneLink().HTTPS
		case "REPO_SSH_URL":
			return generateRepo.CloneLink().SSH
		case "TEMPLATE_SSH_URL":
			return templateRepo.CloneLink().SSH
		default:
			return key
		}
	})
}
