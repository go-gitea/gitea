// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/glob"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/huandu/xstrings"
)

type transformer struct {
	Name      string
	Transform func(string) string
}

type expansion struct {
	Name         string
	Value        string
	Transformers []transformer
}

var globalVars = sync.OnceValue(func() (ret struct {
	defaultTransformers    []transformer
	fileNameSanitizeRegexp *regexp.Regexp
},
) {
	ret.defaultTransformers = []transformer{
		{Name: "SNAKE", Transform: xstrings.ToSnakeCase},
		{Name: "KEBAB", Transform: xstrings.ToKebabCase},
		{Name: "CAMEL", Transform: xstrings.ToCamelCase},
		{Name: "PASCAL", Transform: xstrings.ToPascalCase},
		{Name: "LOWER", Transform: strings.ToLower},
		{Name: "UPPER", Transform: strings.ToUpper},
		{Name: "TITLE", Transform: util.ToTitleCase},
	}

	// invalid filename contents, based on https://github.com/sindresorhus/filename-reserved-regex
	// "COM10" needs to be opened with UNC "\\.\COM10" on Windows, so itself is valid
	ret.fileNameSanitizeRegexp = regexp.MustCompile(`(?i)[<>:"/\\|?*\x{0000}-\x{001F}]|^(con|prn|aux|nul|com\d|lpt\d)$`)
	return ret
})

func generateExpansion(ctx context.Context, src string, templateRepo, generateRepo *repo_model.Repository) string {
	transformers := globalVars().defaultTransformers
	year, month, day := time.Now().Date()
	expansions := []expansion{
		{Name: "YEAR", Value: strconv.Itoa(year), Transformers: nil},
		{Name: "MONTH", Value: fmt.Sprintf("%02d", int(month)), Transformers: nil},
		{Name: "MONTH_ENGLISH", Value: month.String(), Transformers: transformers},
		{Name: "DAY", Value: fmt.Sprintf("%02d", day), Transformers: nil},
		{Name: "REPO_NAME", Value: generateRepo.Name, Transformers: transformers},
		{Name: "TEMPLATE_NAME", Value: templateRepo.Name, Transformers: transformers},
		{Name: "REPO_DESCRIPTION", Value: generateRepo.Description, Transformers: nil},
		{Name: "TEMPLATE_DESCRIPTION", Value: templateRepo.Description, Transformers: nil},
		{Name: "REPO_OWNER", Value: generateRepo.OwnerName, Transformers: transformers},
		{Name: "TEMPLATE_OWNER", Value: templateRepo.OwnerName, Transformers: transformers},
		{Name: "REPO_LINK", Value: generateRepo.Link(), Transformers: nil},
		{Name: "TEMPLATE_LINK", Value: templateRepo.Link(), Transformers: nil},
		{Name: "REPO_HTTPS_URL", Value: generateRepo.CloneLinkGeneral(ctx).HTTPS, Transformers: nil},
		{Name: "TEMPLATE_HTTPS_URL", Value: templateRepo.CloneLinkGeneral(ctx).HTTPS, Transformers: nil},
		{Name: "REPO_SSH_URL", Value: generateRepo.CloneLinkGeneral(ctx).SSH, Transformers: nil},
		{Name: "TEMPLATE_SSH_URL", Value: templateRepo.CloneLinkGeneral(ctx).SSH, Transformers: nil},
	}

	expansionMap := make(map[string]string)
	for _, e := range expansions {
		expansionMap[e.Name] = e.Value
		for _, tr := range e.Transformers {
			expansionMap[fmt.Sprintf("%s_%s", e.Name, tr.Name)] = tr.Transform(e.Value)
		}
	}

	return os.Expand(src, func(key string) string {
		if val, ok := expansionMap[key]; ok {
			return val
		}
		return key
	})
}

// giteaTemplateFileMatcher holds information about a .gitea/template file
type giteaTemplateFileMatcher struct {
	LocalFullPath string
	globs         []glob.Glob
}

func newGiteaTemplateFileMatcher(fullPath string, content []byte) *giteaTemplateFileMatcher {
	gt := &giteaTemplateFileMatcher{LocalFullPath: fullPath}
	gt.globs = make([]glob.Glob, 0)
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		g, err := glob.Compile(line, '/')
		if err != nil {
			log.Debug("Invalid glob expression '%s' (skipped): %v", line, err)
			continue
		}
		gt.globs = append(gt.globs, g)
	}
	return gt
}

func (gt *giteaTemplateFileMatcher) HasRules() bool {
	return len(gt.globs) != 0
}

func (gt *giteaTemplateFileMatcher) Match(s string) bool {
	for _, g := range gt.globs {
		if g.Match(s) {
			return true
		}
	}
	return false
}

func readLocalTmpRepoFileContent(localPath string, limit int) ([]byte, error) {
	ok, err := util.IsRegularFile(localPath)
	if err != nil {
		return nil, err
	} else if !ok {
		return nil, fs.ErrNotExist
	}

	f, err := os.Open(localPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return util.ReadWithLimit(f, limit)
}

func readGiteaTemplateFile(tmpDir string) (*giteaTemplateFileMatcher, error) {
	localPath := filepath.Join(tmpDir, ".gitea", "template")
	content, err := readLocalTmpRepoFileContent(localPath, 1024*1024)
	if err != nil {
		return nil, err
	}
	return newGiteaTemplateFileMatcher(localPath, content), nil
}

func substGiteaTemplateFile(ctx context.Context, tmpDir, tmpDirSubPath string, templateRepo, generateRepo *repo_model.Repository) error {
	tmpFullPath := filepath.Join(tmpDir, tmpDirSubPath)
	content, err := readLocalTmpRepoFileContent(tmpFullPath, 1024*1024)
	if err != nil {
		return util.Iif(errors.Is(err, fs.ErrNotExist), nil, err)
	}
	if err := util.Remove(tmpFullPath); err != nil {
		return err
	}

	generatedContent := generateExpansion(ctx, string(content), templateRepo, generateRepo)
	substSubPath := filePathSanitize(generateExpansion(ctx, tmpDirSubPath, templateRepo, generateRepo))
	newLocalPath := filepath.Join(tmpDir, substSubPath)
	regular, err := util.IsRegularFile(newLocalPath)
	if canWrite := regular || errors.Is(err, fs.ErrNotExist); !canWrite {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(newLocalPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(newLocalPath, []byte(generatedContent), 0o644)
}

func processGiteaTemplateFile(ctx context.Context, tmpDir string, templateRepo, generateRepo *repo_model.Repository, fileMatcher *giteaTemplateFileMatcher) error {
	if err := util.Remove(fileMatcher.LocalFullPath); err != nil {
		return fmt.Errorf("unable to remove .gitea/template: %w", err)
	}
	if !fileMatcher.HasRules() {
		return nil // Avoid walking tree if there are no globs
	}

	return filepath.WalkDir(tmpDir, func(fullPath string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		tmpDirSubPath, err := filepath.Rel(tmpDir, fullPath)
		if err != nil {
			return err
		}
		if fileMatcher.Match(filepath.ToSlash(tmpDirSubPath)) {
			return substGiteaTemplateFile(ctx, tmpDir, tmpDirSubPath, templateRepo, generateRepo)
		}
		return nil
	}) // end: WalkDir
}

func generateRepoCommit(ctx context.Context, repo, templateRepo, generateRepo *repo_model.Repository, tmpDir string) error {
	// Clone to temporary path and do the init commit.
	if err := gitrepo.CloneRepoToLocal(ctx, templateRepo, tmpDir, git.CloneRepoOptions{
		Depth:  1,
		Branch: templateRepo.DefaultBranch,
	}); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}

	// Get active submodules from the template
	submodules, err := git.GetTemplateSubmoduleCommits(ctx, tmpDir)
	if err != nil {
		return fmt.Errorf("GetTemplateSubmoduleCommits: %w", err)
	}

	if err = util.RemoveAll(filepath.Join(tmpDir, ".git")); err != nil {
		return fmt.Errorf("remove git dir: %w", err)
	}

	// Variable expansion
	fileMatcher, err := readGiteaTemplateFile(tmpDir)
	if err == nil {
		err = processGiteaTemplateFile(ctx, tmpDir, templateRepo, generateRepo, fileMatcher)
		if err != nil {
			return fmt.Errorf("processGiteaTemplateFile: %w", err)
		}
	} else if errors.Is(err, fs.ErrNotExist) {
		log.Debug("skip processing repo template files: no available .gitea/template")
	} else {
		return fmt.Errorf("readGiteaTemplateFile: %w", err)
	}

	if err = git.InitRepository(ctx, tmpDir, false, templateRepo.ObjectFormatName); err != nil {
		return err
	}

	if err = git.AddTemplateSubmoduleIndexes(ctx, tmpDir, submodules); err != nil {
		return fmt.Errorf("failed to add submodules: %v", err)
	}

	// set default branch based on whether it's specified in the newly generated repo or not
	defaultBranch := repo.DefaultBranch
	if strings.TrimSpace(defaultBranch) == "" {
		defaultBranch = templateRepo.DefaultBranch
	}

	return initRepoCommit(ctx, tmpDir, repo, repo.Owner, defaultBranch)
}

// GenerateGitContent generates git content from a template repository
func GenerateGitContent(ctx context.Context, templateRepo, generateRepo *repo_model.Repository) (err error) {
	tmpDir, cleanup, err := setting.AppDataTempDir("git-repo-content").MkdirTempRandom("gitea-" + generateRepo.Name)
	if err != nil {
		return fmt.Errorf("failed to create temp dir for repository %s: %w", generateRepo.FullName(), err)
	}
	defer cleanup()

	if err = generateRepoCommit(ctx, generateRepo, templateRepo, generateRepo, tmpDir); err != nil {
		return fmt.Errorf("generateRepoCommit: %w", err)
	}

	// re-fetch repo
	if generateRepo, err = repo_model.GetRepositoryByID(ctx, generateRepo.ID); err != nil {
		return fmt.Errorf("getRepositoryByID: %w", err)
	}

	// if there was no default branch supplied when generating the repo, use the default one from the template
	if strings.TrimSpace(generateRepo.DefaultBranch) == "" {
		generateRepo.DefaultBranch = templateRepo.DefaultBranch
	}

	if err = gitrepo.SetDefaultBranch(ctx, generateRepo, generateRepo.DefaultBranch); err != nil {
		return fmt.Errorf("setDefaultBranch: %w", err)
	}
	if err = repo_model.UpdateRepositoryColsNoAutoTime(ctx, generateRepo, "default_branch"); err != nil {
		return fmt.Errorf("updateRepository: %w", err)
	}

	if err := repo_module.UpdateRepoSize(ctx, generateRepo); err != nil {
		return fmt.Errorf("failed to update size for repository: %w", err)
	}

	if err := git_model.CopyLFS(ctx, generateRepo, templateRepo); err != nil {
		return fmt.Errorf("failed to copy LFS: %w", err)
	}
	return nil
}

// GenerateRepoOptions contains the template units to generate
type GenerateRepoOptions struct {
	Name            string
	DefaultBranch   string
	Description     string
	Private         bool
	GitContent      bool
	Topics          bool
	GitHooks        bool
	Webhooks        bool
	Avatar          bool
	IssueLabels     bool
	ProtectedBranch bool
}

// IsValid checks whether at least one option is chosen for generation
func (gro GenerateRepoOptions) IsValid() bool {
	return gro.GitContent || gro.Topics || gro.GitHooks || gro.Webhooks || gro.Avatar ||
		gro.IssueLabels || gro.ProtectedBranch // or other items as they are added
}

func filePathSanitize(s string) string {
	fields := strings.Split(filepath.ToSlash(s), "/")
	for i, field := range fields {
		field = strings.TrimSpace(strings.TrimSpace(globalVars().fileNameSanitizeRegexp.ReplaceAllString(field, "_")))
		if strings.HasPrefix(field, "..") {
			field = "__" + field[2:]
		}
		if strings.EqualFold(field, ".git") {
			field = "_" + field[1:]
		}
		fields[i] = field
	}
	return filepath.Clean(filepath.FromSlash(strings.Trim(strings.Join(fields, "/"), "/")))
}
