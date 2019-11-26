// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/modules/process"
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
	_, stderr, err := process.GetManager().ExecDirEnv(
		10*time.Minute, "",
		fmt.Sprintf("generateRepoCommit(git clone): %s", templateRepoPath),
		env,
		git.GitExecutable, "clone", "--depth", "1", templateRepoPath, tmpDir,
	)
	if err != nil {
		return fmt.Errorf("git clone: %v - %s", err, stderr)
	}

	if err := os.RemoveAll(path.Join(tmpDir, ".git")); err != nil {
		return fmt.Errorf("remove git dir: %v", err)
	}

	// Variable expansion in _template files
	if err := filepath.Walk(tmpDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if info.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		pathNoExt := strings.TrimSuffix(path, ext)

		if !strings.HasSuffix(pathNoExt, "_template") {
			return nil
		}

		newPath := fmt.Sprintf("%s%s", strings.TrimSuffix(pathNoExt, "_template"), ext)

		content, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		if err := ioutil.WriteFile(newPath,
			[]byte(generateExpansion(string(content), templateRepo, generateRepo)),
			0644); err != nil {
			return err
		}

		return os.Remove(path)
	}); err != nil {
		return err
	}

	if err := git.InitRepository(tmpDir, false); err != nil {
		return err
	}

	repoPath := repo.repoPath(e)
	_, stderr, err = process.GetManager().ExecDirEnv(
		-1, tmpDir,
		fmt.Sprintf("generateRepoCommit(git remote add): %s", repoPath),
		env,
		git.GitExecutable, "remote", "add", "origin", repoPath,
	)
	if err != nil {
		return fmt.Errorf("git remote add: %v - %s", err, stderr)
	}

	return initRepoCommit(tmpDir, repo.Owner)
}

// generateRepository initializes repository from template
func generateRepository(e Engine, repo, templateRepo, generateRepo *Repository) (err error) {
	tmpDir := filepath.Join(os.TempDir(), "gitea-"+repo.Name+"-"+com.ToStr(time.Now().Nanosecond()))

	if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create dir %s: %v", tmpDir, err)
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

		generateHook.Content = generateExpansion(templateHook.Content, templateRepo, generateRepo)
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
