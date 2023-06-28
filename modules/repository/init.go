// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/label"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates/vars"
	"code.gitea.io/gitea/modules/util"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
)

type OptionFile struct {
	DisplayName string
	Description string
}

var (
	// Gitignores contains the gitiginore files
	Gitignores []string

	// Licenses contains the license files
	Licenses []string

	// Readmes contains the readme files
	Readmes []string

	// LabelTemplateFiles contains the label template files, each item has its DisplayName and Description
	LabelTemplateFiles   []OptionFile
	labelTemplateFileMap = map[string]string{} // DisplayName => FileName mapping
)

type optionFileList struct {
	all    []string // all files provided by bindata & custom-path. Sorted.
	custom []string // custom files provided by custom-path. Non-sorted, internal use only.
}

// mergeCustomLabelFiles merges the custom label files. Always use the file's main name (DisplayName) as the key to de-duplicate.
func mergeCustomLabelFiles(fl optionFileList) []string {
	exts := map[string]int{"": 0, ".yml": 1, ".yaml": 2} // "yaml" file has the highest priority to be used.

	m := map[string]string{}
	merge := func(list []string) {
		sort.Slice(list, func(i, j int) bool { return exts[filepath.Ext(list[i])] < exts[filepath.Ext(list[j])] })
		for _, f := range list {
			m[strings.TrimSuffix(f, filepath.Ext(f))] = f
		}
	}
	merge(fl.all)
	merge(fl.custom)

	files := make([]string, 0, len(m))
	for _, f := range m {
		files = append(files, f)
	}
	sort.Strings(files)
	return files
}

// LoadRepoConfig loads the repository config
func LoadRepoConfig() error {
	types := []string{"gitignore", "license", "readme", "label"} // option file directories
	typeFiles := make([]optionFileList, len(types))
	for i, t := range types {
		var err error
		if typeFiles[i].all, err = options.AssetFS().ListFiles(t, true); err != nil {
			return fmt.Errorf("failed to list %s files: %w", t, err)
		}
		sort.Strings(typeFiles[i].all)
		customPath := filepath.Join(setting.CustomPath, "options", t)
		if isDir, err := util.IsDir(customPath); err != nil {
			return fmt.Errorf("failed to check custom %s dir: %w", t, err)
		} else if isDir {
			if typeFiles[i].custom, err = util.StatDir(customPath); err != nil {
				return fmt.Errorf("failed to list custom %s files: %w", t, err)
			}
		}
	}

	Gitignores = typeFiles[0].all
	Licenses = typeFiles[1].all
	Readmes = typeFiles[2].all

	// Load label templates
	LabelTemplateFiles = nil
	labelTemplateFileMap = map[string]string{}
	for _, file := range mergeCustomLabelFiles(typeFiles[3]) {
		description, err := label.LoadTemplateDescription(file)
		if err != nil {
			return fmt.Errorf("failed to load labels: %w", err)
		}
		displayName := strings.TrimSuffix(file, filepath.Ext(file))
		labelTemplateFileMap[displayName] = file
		LabelTemplateFiles = append(LabelTemplateFiles, OptionFile{DisplayName: displayName, Description: description})
	}

	// Filter out invalid names and promote preferred licenses.
	sortedLicenses := make([]string, 0, len(Licenses))
	for _, name := range setting.Repository.PreferredLicenses {
		if util.SliceContainsString(Licenses, name, true) {
			sortedLicenses = append(sortedLicenses, name)
		}
	}
	for _, name := range Licenses {
		if !util.SliceContainsString(setting.Repository.PreferredLicenses, name, true) {
			sortedLicenses = append(sortedLicenses, name)
		}
	}
	Licenses = sortedLicenses
	return nil
}

func prepareRepoCommit(ctx context.Context, repo *repo_model.Repository, tmpDir, repoPath string, opts CreateRepoOptions) error {
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
	if stdout, _, err := git.NewCommand(ctx, "clone").AddDynamicArguments(repoPath, tmpDir).
		SetDescription(fmt.Sprintf("prepareRepoCommit (git clone): %s to %s", repoPath, tmpDir)).
		RunStdString(&git.RunOpts{Dir: "", Env: env}); err != nil {
		log.Error("Failed to clone from %v into %s: stdout: %s\nError: %v", repo, tmpDir, stdout, err)
		return fmt.Errorf("git clone: %w", err)
	}

	// README
	data, err := options.Readme(opts.Readme)
	if err != nil {
		return fmt.Errorf("GetRepoInitFile[%s]: %w", opts.Readme, err)
	}

	cloneLink := repo.CloneLink()
	match := map[string]string{
		"Name":           repo.Name,
		"Description":    repo.Description,
		"CloneURL.SSH":   cloneLink.SSH,
		"CloneURL.HTTPS": cloneLink.HTTPS,
		"OwnerName":      repo.OwnerName,
	}
	res, err := vars.Expand(string(data), match)
	if err != nil {
		// here we could just log the error and continue the rendering
		log.Error("unable to expand template vars for repo README: %s, err: %v", opts.Readme, err)
	}
	if err = os.WriteFile(filepath.Join(tmpDir, "README.md"),
		[]byte(res), 0o644); err != nil {
		return fmt.Errorf("write README.md: %w", err)
	}

	// .gitignore
	if len(opts.Gitignores) > 0 {
		var buf bytes.Buffer
		names := strings.Split(opts.Gitignores, ",")
		for _, name := range names {
			data, err = options.Gitignore(name)
			if err != nil {
				return fmt.Errorf("GetRepoInitFile[%s]: %w", name, err)
			}
			buf.WriteString("# ---> " + name + "\n")
			buf.Write(data)
			buf.WriteString("\n")
		}

		if buf.Len() > 0 {
			if err = os.WriteFile(filepath.Join(tmpDir, ".gitignore"), buf.Bytes(), 0o644); err != nil {
				return fmt.Errorf("write .gitignore: %w", err)
			}
		}
	}

	// LICENSE
	if len(opts.License) > 0 {
		data, err = getLicense(opts.License, &licenseValues{
			Owner: repo.OwnerName,
			Email: authorSig.Email,
			Repo:  repo.Name,
			Year:  time.Now().Format("2006"),
		})
		if err != nil {
			return fmt.Errorf("getLicense[%s]: %w", opts.License, err)
		}

		if err = os.WriteFile(filepath.Join(tmpDir, "LICENSE"), data, 0o644); err != nil {
			return fmt.Errorf("write LICENSE: %w", err)
		}
	}

	return nil
}

// initRepoCommit temporarily changes with work directory.
func initRepoCommit(ctx context.Context, tmpPath string, repo *repo_model.Repository, u *user_model.User, defaultBranch string) (err error) {
	commitTimeStr := time.Now().Format(time.RFC3339)

	sig := u.NewGitSig()
	// Because this may call hooks we should pass in the environment
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME="+sig.Name,
		"GIT_AUTHOR_EMAIL="+sig.Email,
		"GIT_AUTHOR_DATE="+commitTimeStr,
		"GIT_COMMITTER_DATE="+commitTimeStr,
	)
	committerName := sig.Name
	committerEmail := sig.Email

	if stdout, _, err := git.NewCommand(ctx, "add", "--all").
		SetDescription(fmt.Sprintf("initRepoCommit (git add): %s", tmpPath)).
		RunStdString(&git.RunOpts{Dir: tmpPath}); err != nil {
		log.Error("git add --all failed: Stdout: %s\nError: %v", stdout, err)
		return fmt.Errorf("git add --all: %w", err)
	}

	cmd := git.NewCommand(ctx, "commit", "--message=Initial commit").
		AddOptionFormat("--author='%s <%s>'", sig.Name, sig.Email)

	sign, keyID, signer, _ := asymkey_service.SignInitialCommit(ctx, tmpPath, u)
	if sign {
		cmd.AddOptionFormat("-S%s", keyID)

		if repo.GetTrustModel() == repo_model.CommitterTrustModel || repo.GetTrustModel() == repo_model.CollaboratorCommitterTrustModel {
			// need to set the committer to the KeyID owner
			committerName = signer.Name
			committerEmail = signer.Email
		}
	} else {
		cmd.AddArguments("--no-gpg-sign")
	}

	env = append(env,
		"GIT_COMMITTER_NAME="+committerName,
		"GIT_COMMITTER_EMAIL="+committerEmail,
	)

	if stdout, _, err := cmd.
		SetDescription(fmt.Sprintf("initRepoCommit (git commit): %s", tmpPath)).
		RunStdString(&git.RunOpts{Dir: tmpPath, Env: env}); err != nil {
		log.Error("Failed to commit: %v: Stdout: %s\nError: %v", cmd.String(), stdout, err)
		return fmt.Errorf("git commit: %w", err)
	}

	if len(defaultBranch) == 0 {
		defaultBranch = setting.Repository.DefaultBranch
	}

	if stdout, _, err := git.NewCommand(ctx, "push", "origin").AddDynamicArguments("HEAD:" + defaultBranch).
		SetDescription(fmt.Sprintf("initRepoCommit (git push): %s", tmpPath)).
		RunStdString(&git.RunOpts{Dir: tmpPath, Env: InternalPushingEnvironment(u, repo)}); err != nil {
		log.Error("Failed to push back to HEAD: Stdout: %s\nError: %v", stdout, err)
		return fmt.Errorf("git push: %w", err)
	}

	return nil
}

func checkInitRepository(ctx context.Context, owner, name string) (err error) {
	// Somehow the directory could exist.
	repoPath := repo_model.RepoPath(owner, name)
	isExist, err := util.IsExist(repoPath)
	if err != nil {
		log.Error("Unable to check if %s exists. Error: %v", repoPath, err)
		return err
	}
	if isExist {
		return repo_model.ErrRepoFilesAlreadyExist{
			Uname: owner,
			Name:  name,
		}
	}

	// Init git bare new repository.
	if err = git.InitRepository(ctx, repoPath, true); err != nil {
		return fmt.Errorf("git.InitRepository: %w", err)
	} else if err = createDelegateHooks(repoPath); err != nil {
		return fmt.Errorf("createDelegateHooks: %w", err)
	}
	return nil
}

// InitRepository initializes README and .gitignore if needed.
func initRepository(ctx context.Context, repoPath string, u *user_model.User, repo *repo_model.Repository, opts CreateRepoOptions) (err error) {
	if err = checkInitRepository(ctx, repo.OwnerName, repo.Name); err != nil {
		return err
	}

	// Initialize repository according to user's choice.
	if opts.AutoInit {
		tmpDir, err := os.MkdirTemp(os.TempDir(), "gitea-"+repo.Name)
		if err != nil {
			return fmt.Errorf("Failed to create temp dir for repository %s: %w", repo.RepoPath(), err)
		}
		defer func() {
			if err := util.RemoveAll(tmpDir); err != nil {
				log.Warn("Unable to remove temporary directory: %s: Error: %v", tmpDir, err)
			}
		}()

		if err = prepareRepoCommit(ctx, repo, tmpDir, repoPath, opts); err != nil {
			return fmt.Errorf("prepareRepoCommit: %w", err)
		}

		// Apply changes and commit.
		if err = initRepoCommit(ctx, tmpDir, repo, u, opts.DefaultBranch); err != nil {
			return fmt.Errorf("initRepoCommit: %w", err)
		}
	}

	// Re-fetch the repository from database before updating it (else it would
	// override changes that were done earlier with sql)
	if repo, err = repo_model.GetRepositoryByID(ctx, repo.ID); err != nil {
		return fmt.Errorf("getRepositoryByID: %w", err)
	}

	if !opts.AutoInit {
		repo.IsEmpty = true
	}

	repo.DefaultBranch = setting.Repository.DefaultBranch

	if len(opts.DefaultBranch) > 0 {
		repo.DefaultBranch = opts.DefaultBranch
		gitRepo, err := git.OpenRepository(ctx, repo.RepoPath())
		if err != nil {
			return fmt.Errorf("openRepository: %w", err)
		}
		defer gitRepo.Close()
		if err = gitRepo.SetDefaultBranch(repo.DefaultBranch); err != nil {
			return fmt.Errorf("setDefaultBranch: %w", err)
		}

		if !repo.IsEmpty {
			if _, err := SyncRepoBranches(ctx, repo.ID, u.ID); err != nil {
				return fmt.Errorf("SyncRepoBranches: %w", err)
			}
		}
	}

	if err = UpdateRepository(ctx, repo, false); err != nil {
		return fmt.Errorf("updateRepository: %w", err)
	}

	return nil
}

// InitializeLabels adds a label set to a repository using a template
func InitializeLabels(ctx context.Context, id int64, labelTemplate string, isOrg bool) error {
	list, err := LoadTemplateLabelsByDisplayName(labelTemplate)
	if err != nil {
		return err
	}

	labels := make([]*issues_model.Label, len(list))
	for i := 0; i < len(list); i++ {
		labels[i] = &issues_model.Label{
			Name:        list[i].Name,
			Exclusive:   list[i].Exclusive,
			Description: list[i].Description,
			Color:       list[i].Color,
		}
		if isOrg {
			labels[i].OrgID = id
		} else {
			labels[i].RepoID = id
		}
	}
	for _, label := range labels {
		if err = issues_model.NewLabel(ctx, label); err != nil {
			return err
		}
	}
	return nil
}

// LoadTemplateLabelsByDisplayName loads a label template by its display name
func LoadTemplateLabelsByDisplayName(displayName string) ([]*label.Label, error) {
	if fileName, ok := labelTemplateFileMap[displayName]; ok {
		return label.LoadTemplateFile(fileName)
	}
	return nil, label.ErrTemplateLoad{TemplateFile: displayName, OriginalError: fmt.Errorf("label template %q not found", displayName)}
}
