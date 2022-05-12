// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates/vars"
	"code.gitea.io/gitea/modules/util"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
)

var (
	// Gitignores contains the gitiginore files
	Gitignores []string

	// Licenses contains the license files
	Licenses []string

	// Readmes contains the readme files
	Readmes []string

	// LabelTemplates contains the label template files and the list of labels for each file
	LabelTemplates map[string]string
)

// ErrIssueLabelTemplateLoad represents a "ErrIssueLabelTemplateLoad" kind of error.
type ErrIssueLabelTemplateLoad struct {
	TemplateFile  string
	OriginalError error
}

// IsErrIssueLabelTemplateLoad checks if an error is a ErrIssueLabelTemplateLoad.
func IsErrIssueLabelTemplateLoad(err error) bool {
	_, ok := err.(ErrIssueLabelTemplateLoad)
	return ok
}

func (err ErrIssueLabelTemplateLoad) Error() string {
	return fmt.Sprintf("Failed to load label template file '%s': %v", err.TemplateFile, err.OriginalError)
}

// GetRepoInitFile returns repository init files
func GetRepoInitFile(tp, name string) ([]byte, error) {
	cleanedName := strings.TrimLeft(path.Clean("/"+name), "/")
	relPath := path.Join("options", tp, cleanedName)

	// Use custom file when available.
	customPath := path.Join(setting.CustomPath, relPath)
	isFile, err := util.IsFile(customPath)
	if err != nil {
		log.Error("Unable to check if %s is a file. Error: %v", customPath, err)
	}
	if isFile {
		return os.ReadFile(customPath)
	}

	switch tp {
	case "readme":
		return options.Readme(cleanedName)
	case "gitignore":
		return options.Gitignore(cleanedName)
	case "license":
		return options.License(cleanedName)
	case "label":
		return options.Labels(cleanedName)
	default:
		return []byte{}, fmt.Errorf("Invalid init file type")
	}
}

// GetLabelTemplateFile loads the label template file by given name,
// then parses and returns a list of name-color pairs and optionally description.
func GetLabelTemplateFile(name string) ([][3]string, error) {
	data, err := GetRepoInitFile("label", name)
	if err != nil {
		return nil, ErrIssueLabelTemplateLoad{name, fmt.Errorf("GetRepoInitFile: %v", err)}
	}

	lines := strings.Split(string(data), "\n")
	list := make([][3]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}

		parts := strings.SplitN(line, ";", 2)

		fields := strings.SplitN(parts[0], " ", 2)
		if len(fields) != 2 {
			return nil, ErrIssueLabelTemplateLoad{name, fmt.Errorf("line is malformed: %s", line)}
		}

		color := strings.Trim(fields[0], " ")
		if len(color) == 6 {
			color = "#" + color
		}
		if !models.LabelColorPattern.MatchString(color) {
			return nil, ErrIssueLabelTemplateLoad{name, fmt.Errorf("bad HTML color code in line: %s", line)}
		}

		var description string

		if len(parts) > 1 {
			description = strings.TrimSpace(parts[1])
		}

		fields[1] = strings.TrimSpace(fields[1])
		list = append(list, [3]string{fields[1], color, description})
	}

	return list, nil
}

func loadLabels(labelTemplate string) ([]string, error) {
	list, err := GetLabelTemplateFile(labelTemplate)
	if err != nil {
		return nil, err
	}

	labels := make([]string, len(list))
	for i := 0; i < len(list); i++ {
		labels[i] = list[i][0]
	}
	return labels, nil
}

// LoadLabelsFormatted loads the labels' list of a template file as a string separated by comma
func LoadLabelsFormatted(labelTemplate string) (string, error) {
	labels, err := loadLabels(labelTemplate)
	return strings.Join(labels, ", "), err
}

// LoadRepoConfig loads the repository config
func LoadRepoConfig() {
	// Load .gitignore and license files and readme templates.
	types := []string{"gitignore", "license", "readme", "label"}
	typeFiles := make([][]string, 4)
	for i, t := range types {
		files, err := options.Dir(t)
		if err != nil {
			log.Fatal("Failed to get %s files: %v", t, err)
		}
		customPath := path.Join(setting.CustomPath, "options", t)
		isDir, err := util.IsDir(customPath)
		if err != nil {
			log.Fatal("Failed to get custom %s files: %v", t, err)
		}
		if isDir {
			customFiles, err := util.StatDir(customPath)
			if err != nil {
				log.Fatal("Failed to get custom %s files: %v", t, err)
			}

			for _, f := range customFiles {
				if !util.IsStringInSlice(f, files, true) {
					files = append(files, f)
				}
			}
		}
		typeFiles[i] = files
	}

	Gitignores = typeFiles[0]
	Licenses = typeFiles[1]
	Readmes = typeFiles[2]
	LabelTemplatesFiles := typeFiles[3]
	sort.Strings(Gitignores)
	sort.Strings(Licenses)
	sort.Strings(Readmes)
	sort.Strings(LabelTemplatesFiles)

	// Load label templates
	LabelTemplates = make(map[string]string)
	for _, templateFile := range LabelTemplatesFiles {
		labels, err := LoadLabelsFormatted(templateFile)
		if err != nil {
			log.Error("Failed to load labels: %v", err)
		}
		LabelTemplates[templateFile] = labels
	}

	// Filter out invalid names and promote preferred licenses.
	sortedLicenses := make([]string, 0, len(Licenses))
	for _, name := range setting.Repository.PreferredLicenses {
		if util.IsStringInSlice(name, Licenses, true) {
			sortedLicenses = append(sortedLicenses, name)
		}
	}
	for _, name := range Licenses {
		if !util.IsStringInSlice(name, setting.Repository.PreferredLicenses, true) {
			sortedLicenses = append(sortedLicenses, name)
		}
	}
	Licenses = sortedLicenses
}

func prepareRepoCommit(ctx context.Context, repo *repo_model.Repository, tmpDir, repoPath string, opts models.CreateRepoOptions) error {
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
	if stdout, _, err := git.NewCommand(ctx, "clone", repoPath, tmpDir).
		SetDescription(fmt.Sprintf("prepareRepoCommit (git clone): %s to %s", repoPath, tmpDir)).
		RunStdString(&git.RunOpts{Dir: "", Env: env}); err != nil {
		log.Error("Failed to clone from %v into %s: stdout: %s\nError: %v", repo, tmpDir, stdout, err)
		return fmt.Errorf("git clone: %v", err)
	}

	// README
	data, err := GetRepoInitFile("readme", opts.Readme)
	if err != nil {
		return fmt.Errorf("GetRepoInitFile[%s]: %v", opts.Readme, err)
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
		return fmt.Errorf("write README.md: %v", err)
	}

	// .gitignore
	if len(opts.Gitignores) > 0 {
		var buf bytes.Buffer
		names := strings.Split(opts.Gitignores, ",")
		for _, name := range names {
			data, err = GetRepoInitFile("gitignore", name)
			if err != nil {
				return fmt.Errorf("GetRepoInitFile[%s]: %v", name, err)
			}
			buf.WriteString("# ---> " + name + "\n")
			buf.Write(data)
			buf.WriteString("\n")
		}

		if buf.Len() > 0 {
			if err = os.WriteFile(filepath.Join(tmpDir, ".gitignore"), buf.Bytes(), 0o644); err != nil {
				return fmt.Errorf("write .gitignore: %v", err)
			}
		}
	}

	// LICENSE
	if len(opts.License) > 0 {
		data, err = GetRepoInitFile("license", opts.License)
		if err != nil {
			return fmt.Errorf("GetRepoInitFile[%s]: %v", opts.License, err)
		}

		if err = os.WriteFile(filepath.Join(tmpDir, "LICENSE"), data, 0o644); err != nil {
			return fmt.Errorf("write LICENSE: %v", err)
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
		return fmt.Errorf("git add --all: %v", err)
	}

	err = git.LoadGitVersion()
	if err != nil {
		return fmt.Errorf("Unable to get git version: %v", err)
	}

	args := []string{
		"commit", fmt.Sprintf("--author='%s <%s>'", sig.Name, sig.Email),
		"-m", "Initial commit",
	}

	if git.CheckGitVersionAtLeast("1.7.9") == nil {
		sign, keyID, signer, _ := asymkey_service.SignInitialCommit(ctx, tmpPath, u)
		if sign {
			args = append(args, "-S"+keyID)

			if repo.GetTrustModel() == repo_model.CommitterTrustModel || repo.GetTrustModel() == repo_model.CollaboratorCommitterTrustModel {
				// need to set the committer to the KeyID owner
				committerName = signer.Name
				committerEmail = signer.Email
			}
		} else if git.CheckGitVersionAtLeast("2.0.0") == nil {
			args = append(args, "--no-gpg-sign")
		}
	}

	env = append(env,
		"GIT_COMMITTER_NAME="+committerName,
		"GIT_COMMITTER_EMAIL="+committerEmail,
	)

	if stdout, _, err := git.NewCommand(ctx, args...).
		SetDescription(fmt.Sprintf("initRepoCommit (git commit): %s", tmpPath)).
		RunStdString(&git.RunOpts{Dir: tmpPath, Env: env}); err != nil {
		log.Error("Failed to commit: %v: Stdout: %s\nError: %v", args, stdout, err)
		return fmt.Errorf("git commit: %v", err)
	}

	if len(defaultBranch) == 0 {
		defaultBranch = setting.Repository.DefaultBranch
	}

	if stdout, _, err := git.NewCommand(ctx, "push", "origin", "HEAD:"+defaultBranch).
		SetDescription(fmt.Sprintf("initRepoCommit (git push): %s", tmpPath)).
		RunStdString(&git.RunOpts{Dir: tmpPath, Env: InternalPushingEnvironment(u, repo)}); err != nil {
		log.Error("Failed to push back to HEAD: Stdout: %s\nError: %v", stdout, err)
		return fmt.Errorf("git push: %v", err)
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
		return fmt.Errorf("git.InitRepository: %v", err)
	} else if err = createDelegateHooks(repoPath); err != nil {
		return fmt.Errorf("createDelegateHooks: %v", err)
	}
	return nil
}

// InitRepository initializes README and .gitignore if needed.
func initRepository(ctx context.Context, repoPath string, u *user_model.User, repo *repo_model.Repository, opts models.CreateRepoOptions) (err error) {
	if err = checkInitRepository(ctx, repo.OwnerName, repo.Name); err != nil {
		return err
	}

	// Initialize repository according to user's choice.
	if opts.AutoInit {
		tmpDir, err := os.MkdirTemp(os.TempDir(), "gitea-"+repo.Name)
		if err != nil {
			return fmt.Errorf("Failed to create temp dir for repository %s: %v", repo.RepoPath(), err)
		}
		defer func() {
			if err := util.RemoveAll(tmpDir); err != nil {
				log.Warn("Unable to remove temporary directory: %s: Error: %v", tmpDir, err)
			}
		}()

		if err = prepareRepoCommit(ctx, repo, tmpDir, repoPath, opts); err != nil {
			return fmt.Errorf("prepareRepoCommit: %v", err)
		}

		// Apply changes and commit.
		if err = initRepoCommit(ctx, tmpDir, repo, u, opts.DefaultBranch); err != nil {
			return fmt.Errorf("initRepoCommit: %v", err)
		}
	}

	// Re-fetch the repository from database before updating it (else it would
	// override changes that were done earlier with sql)
	if repo, err = repo_model.GetRepositoryByIDCtx(ctx, repo.ID); err != nil {
		return fmt.Errorf("getRepositoryByID: %v", err)
	}

	if !opts.AutoInit {
		repo.IsEmpty = true
	}

	repo.DefaultBranch = setting.Repository.DefaultBranch

	if len(opts.DefaultBranch) > 0 {
		repo.DefaultBranch = opts.DefaultBranch
		gitRepo, err := git.OpenRepository(ctx, repo.RepoPath())
		if err != nil {
			return fmt.Errorf("openRepository: %v", err)
		}
		defer gitRepo.Close()
		if err = gitRepo.SetDefaultBranch(repo.DefaultBranch); err != nil {
			return fmt.Errorf("setDefaultBranch: %v", err)
		}
	}

	if err = models.UpdateRepositoryCtx(ctx, repo, false); err != nil {
		return fmt.Errorf("updateRepository: %v", err)
	}

	return nil
}

// InitializeLabels adds a label set to a repository using a template
func InitializeLabels(ctx context.Context, id int64, labelTemplate string, isOrg bool) error {
	list, err := GetLabelTemplateFile(labelTemplate)
	if err != nil {
		return err
	}

	labels := make([]*models.Label, len(list))
	for i := 0; i < len(list); i++ {
		labels[i] = &models.Label{
			Name:        list[i][0],
			Description: list[i][2],
			Color:       list[i][1],
		}
		if isOrg {
			labels[i].OrgID = id
		} else {
			labels[i].RepoID = id
		}
	}
	for _, label := range labels {
		if err = models.NewLabel(ctx, label); err != nil {
			return err
		}
	}
	return nil
}
