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
	"strconv"
	"strings"
	"sync"
	"time"

	git_model "codenest.dev/models/git"
	repo_model "codenest.dev/models/repo"
	"codenest.dev/modules/git"
	"codenest.dev/modules/glob"
	"codenest.dev/modules/log"
	repo_module "codenest.dev/modules/repository"
	"codenest.dev/modules/setting"
	"codenest.dev/modules/templates/vars"
	"codenest.dev/modules/util"

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
	defaultTransformers []transformer
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

	return vars.ExpandShellLike(src, expansionMap)
}

type codenestTemplateFileMatcher struct {
	relPath      string
	globsExpand  []glob.Glob
	globsExclude []glob.Glob
}

func newCodeNestTemplateFileMatcher(relPath string, content []byte) *codenestTemplateFileMatcher {
	gt := &codenestTemplateFileMatcher{relPath: relPath}
	scanner := bufio.NewScanner(bytes.NewReader(content))
	addGlob := func(globs *[]glob.Glob, pattern string) {
		g, err := glob.Compile(pattern, '/')
		if err != nil {
			log.Debug("Invalid codenest template glob expression %q (skipped): %v", pattern, err)
			return
		}
		*globs = append(*globs, g)
	}
	curGlobs := &gt.globsExpand
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			switch line {
			case "[expand]":
				curGlobs = &gt.globsExpand
			case "[exclude]":
				curGlobs = &gt.globsExclude
			default:
				log.Debug("Invalid codenest template glob section %q", line)
			}
			continue
		}
		addGlob(curGlobs, line)
	}
	return gt
}

func (gt *codenestTemplateFileMatcher) hasAnyRules() bool {
	return len(gt.globsExpand) != 0 || len(gt.globsExclude) != 0
}

func (gt *codenestTemplateFileMatcher) matchRules(globs []glob.Glob, s string) bool {
	for _, g := range globs {
		if g.Match(s) {
			return true
		}
	}
	return false
}

func readCodeNestTemplateFile(tmpDir string) (*codenestTemplateFileMatcher, error) {
	templateRelPath := filepath.Join(".gitea", "template")
	content, err := util.ReadRegularPathFile(tmpDir, templateRelPath, 1024*1024)
	if err != nil {
		return nil, util.Iif(errors.Is(err, util.ErrNotRegularPathFile), os.ErrNotExist, err)
	}
	return newCodeNestTemplateFileMatcher(templateRelPath, content), nil
}

func substCodeNestTemplateFile(ctx context.Context, tmpDir, tmpDirSubPath string, templateRepo, generateRepo *repo_model.Repository) error {
	content, err := util.ReadRegularPathFile(tmpDir, tmpDirSubPath, 1024*1024)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := os.Remove(util.FilePathJoinAbs(tmpDir, tmpDirSubPath)); err != nil {
		return err
	}

	generatedContent := generateExpansion(ctx, string(content), templateRepo, generateRepo)
	substSubPath := filePathSanitize(generateExpansion(ctx, tmpDirSubPath, templateRepo, generateRepo))
	return util.WriteRegularPathFile(tmpDir, substSubPath, []byte(generatedContent), 0o755, 0o644)
}

// processCodeNestTemplateFile processes and removes the .gitea/template file,
// does file exclusion and variable expansion for template files, and save the processed files to the filesystem.
// It returns a list of skipped files that are not regular paths.
func processCodeNestTemplateFile(ctx context.Context, tmpDir string, templateRepo, generateRepo *repo_model.Repository, fileMatcher *codenestTemplateFileMatcher) (skippedFiles []string, _ error) {
	// Why not use "os.Root" here: symlink is unsafe even in the same root but "os.Root" can't help, it's more difficult to use "os.Root" to do the WalkDir.
	if err := os.Remove(util.FilePathJoinAbs(tmpDir, fileMatcher.relPath)); err != nil {
		return nil, fmt.Errorf("unable to remove .gitea/template: %w", err)
	}
	if !fileMatcher.hasAnyRules() {
		return skippedFiles, nil // Avoid walking tree if there are no globs
	}

	err := filepath.WalkDir(tmpDir, func(fullPath string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relPath, err := filepath.Rel(tmpDir, fullPath)
		if err != nil {
			return err
		}
		if relPath == "." {
			// WalkDir always visits the root "." first, don't process it (don't "exclude" to remove, or "expand" to subst)
			return nil
		}

		treePath := filepath.ToSlash(relPath)

		// try to "exclude" (remove) first
		if fileMatcher.matchRules(fileMatcher.globsExclude, treePath) {
			isDir := d.IsDir()
			// if the target is a symlink, only the symlink is unlinked, so it is safe
			if err := os.RemoveAll(fullPath); err != nil {
				return err
			}
			return util.Iif(isDir, filepath.SkipDir, nil)
		}

		if d.IsDir() {
			return nil
		}
		// try to expand variables for regular files
		if fileMatcher.matchRules(fileMatcher.globsExpand, treePath) {
			err := substCodeNestTemplateFile(ctx, tmpDir, relPath, templateRepo, generateRepo)
			if errors.Is(err, util.ErrNotRegularPathFile) {
				skippedFiles = append(skippedFiles, relPath)
			} else if err != nil {
				return err
			}
		}
		return nil
	}) // end: WalkDir
	if err != nil {
		return nil, err
	}
	if err = util.RemoveAllWithRetry(util.FilePathJoinAbs(tmpDir, ".git")); err != nil {
		return nil, err
	}
	return skippedFiles, nil
}

func generateRepoCommit(ctx context.Context, repo, templateRepo, generateRepo *repo_model.Repository, tmpDir string) error {
	// set default branch based on whether it's specified in the newly generated repo or not
	repo.DefaultBranch = util.IfZero(repo.DefaultBranch, util.IfZero(templateRepo.DefaultBranch, setting.Repository.DefaultBranch))

	// Clone to temporary path and do the init commit.
	if err := git.CloneRepoToLocal(ctx, templateRepo, tmpDir, git.CloneRepoOptions{
		Depth:  1,
		Branch: templateRepo.DefaultBranch,
	}); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}

	// Get active submodules from the template
	submodules, err := git.GetTemplateSubmoduleCommits(ctx, templateRepo)
	if err != nil {
		return fmt.Errorf("GetTemplateSubmoduleCommits: %w", err)
	}

	if err = util.RemoveAllWithRetry(filepath.Join(tmpDir, ".git")); err != nil {
		return fmt.Errorf("remove git dir: %w", err)
	}

	// Variable expansion
	fileMatcher, err := readCodeNestTemplateFile(tmpDir)
	if err == nil {
		_, err = processCodeNestTemplateFile(ctx, tmpDir, templateRepo, generateRepo, fileMatcher)
		if err != nil {
			return fmt.Errorf("processCodeNestTemplateFile: %w", err)
		}
	} else if errors.Is(err, fs.ErrNotExist) {
		log.Debug("skip processing repo template files: no available .gitea/template")
	} else {
		return fmt.Errorf("readCodeNestTemplateFile: %w", err)
	}

	if err = git.InitRepositoryLocal(ctx, tmpDir, false, templateRepo.ObjectFormatName, ""); err != nil {
		return err
	}

	if err = git.AddTemplateSubmoduleIndexes(ctx, tmpDir, submodules); err != nil {
		return fmt.Errorf("failed to add submodules: %v", err)
	}

	return initRepoCommit(ctx, tmpDir, repo, repo.Owner)
}

// GenerateGitContent generates git content from a template repository
func GenerateGitContent(ctx context.Context, templateRepo, generateRepo *repo_model.Repository) (err error) {
	tmpDir, cleanup, err := setting.AppDataTempDir("git-repo-content").MkdirTempRandom("codenest-" + generateRepo.Name)
	if err != nil {
		return fmt.Errorf("failed to create temp dir for repository %s: %w", generateRepo.FullName(), err)
	}
	defer cleanup()

	if err = generateRepoCommit(ctx, generateRepo, templateRepo, generateRepo, tmpDir); err != nil {
		return fmt.Errorf("generateRepoCommit: %w", err)
	}
	if err = git.SetDefaultBranch(ctx, generateRepo, generateRepo.DefaultBranch); err != nil {
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

	if _, err := repo_module.SyncRepoBranches(ctx, generateRepo.ID, 0); err != nil {
		return fmt.Errorf("SyncRepoBranches: %w", err)
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
		field = util.PathNameValidator().InvalidChars.ReplaceAllString(field, "_")
		field = util.PathNameValidator().InvalidNames.ReplaceAllString(field, "_")
		field = strings.TrimSpace(field)
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
