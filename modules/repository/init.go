// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/mcuadros/go-version"
	"github.com/unknwon/com"
)

func prepareRepoCommit(ctx models.DBContext, repo *models.Repository, tmpDir, repoPath string, opts models.CreateRepoOptions) error {
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
	if stdout, err := git.NewCommand("clone", repoPath, tmpDir).
		SetDescription(fmt.Sprintf("prepareRepoCommit (git clone): %s to %s", repoPath, tmpDir)).
		RunInDirWithEnv("", env); err != nil {
		log.Error("Failed to clone from %v into %s: stdout: %s\nError: %v", repo, tmpDir, stdout, err)
		return fmt.Errorf("git clone: %v", err)
	}

	// README
	data, err := models.GetRepoInitFile("readme", opts.Readme)
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
	if err = ioutil.WriteFile(filepath.Join(tmpDir, "README.md"),
		[]byte(com.Expand(string(data), match)), 0644); err != nil {
		return fmt.Errorf("write README.md: %v", err)
	}

	// .gitignore
	if len(opts.Gitignores) > 0 {
		var buf bytes.Buffer
		names := strings.Split(opts.Gitignores, ",")
		for _, name := range names {
			data, err = models.GetRepoInitFile("gitignore", name)
			if err != nil {
				return fmt.Errorf("GetRepoInitFile[%s]: %v", name, err)
			}
			buf.WriteString("# ---> " + name + "\n")
			buf.Write(data)
			buf.WriteString("\n")
		}

		if buf.Len() > 0 {
			if err = ioutil.WriteFile(filepath.Join(tmpDir, ".gitignore"), buf.Bytes(), 0644); err != nil {
				return fmt.Errorf("write .gitignore: %v", err)
			}
		}
	}

	// LICENSE
	if len(opts.License) > 0 {
		data, err = models.GetRepoInitFile("license", opts.License)
		if err != nil {
			return fmt.Errorf("GetRepoInitFile[%s]: %v", opts.License, err)
		}

		if err = ioutil.WriteFile(filepath.Join(tmpDir, "LICENSE"), data, 0644); err != nil {
			return fmt.Errorf("write LICENSE: %v", err)
		}
	}

	return nil
}

// initRepoCommit temporarily changes with work directory.
func initRepoCommit(tmpPath string, repo *models.Repository, u *models.User, defaultBranch string) (err error) {
	commitTimeStr := time.Now().Format(time.RFC3339)

	sig := u.NewGitSig()
	// Because this may call hooks we should pass in the environment
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME="+sig.Name,
		"GIT_AUTHOR_EMAIL="+sig.Email,
		"GIT_AUTHOR_DATE="+commitTimeStr,
		"GIT_COMMITTER_NAME="+sig.Name,
		"GIT_COMMITTER_EMAIL="+sig.Email,
		"GIT_COMMITTER_DATE="+commitTimeStr,
	)

	if stdout, err := git.NewCommand("add", "--all").
		SetDescription(fmt.Sprintf("initRepoCommit (git add): %s", tmpPath)).
		RunInDir(tmpPath); err != nil {
		log.Error("git add --all failed: Stdout: %s\nError: %v", stdout, err)
		return fmt.Errorf("git add --all: %v", err)
	}

	binVersion, err := git.BinVersion()
	if err != nil {
		return fmt.Errorf("Unable to get git version: %v", err)
	}

	args := []string{
		"commit", fmt.Sprintf("--author='%s <%s>'", sig.Name, sig.Email),
		"-m", "Initial commit",
	}

	if version.Compare(binVersion, "1.7.9", ">=") {
		sign, keyID, _ := models.SignInitialCommit(tmpPath, u)
		if sign {
			args = append(args, "-S"+keyID)
		} else if version.Compare(binVersion, "2.0.0", ">=") {
			args = append(args, "--no-gpg-sign")
		}
	}

	if stdout, err := git.NewCommand(args...).
		SetDescription(fmt.Sprintf("initRepoCommit (git commit): %s", tmpPath)).
		RunInDirWithEnv(tmpPath, env); err != nil {
		log.Error("Failed to commit: %v: Stdout: %s\nError: %v", args, stdout, err)
		return fmt.Errorf("git commit: %v", err)
	}

	if len(defaultBranch) == 0 {
		defaultBranch = setting.Repository.DefaultBranch
	}

	if stdout, err := git.NewCommand("push", "origin", "master:"+defaultBranch).
		SetDescription(fmt.Sprintf("initRepoCommit (git push): %s", tmpPath)).
		RunInDirWithEnv(tmpPath, models.InternalPushingEnvironment(u, repo)); err != nil {
		log.Error("Failed to push back to master: Stdout: %s\nError: %v", stdout, err)
		return fmt.Errorf("git push: %v", err)
	}

	return nil
}

func checkInitRepository(repoPath string) (err error) {
	// Somehow the directory could exist.
	if com.IsExist(repoPath) {
		return fmt.Errorf("checkInitRepository: path already exists: %s", repoPath)
	}

	// Init git bare new repository.
	if err = git.InitRepository(repoPath, true); err != nil {
		return fmt.Errorf("git.InitRepository: %v", err)
	} else if err = createDelegateHooks(repoPath); err != nil {
		return fmt.Errorf("createDelegateHooks: %v", err)
	}
	return nil
}

// InitRepository initializes README and .gitignore if needed.
func initRepository(ctx models.DBContext, repoPath string, u *models.User, repo *models.Repository, opts models.CreateRepoOptions) (err error) {
	if err = checkInitRepository(repoPath); err != nil {
		return err
	}

	// Initialize repository according to user's choice.
	if opts.AutoInit {
		tmpDir, err := ioutil.TempDir(os.TempDir(), "gitea-"+repo.Name)
		if err != nil {
			return fmt.Errorf("Failed to create temp dir for repository %s: %v", repo.RepoPath(), err)
		}

		defer os.RemoveAll(tmpDir)

		if err = prepareRepoCommit(ctx, repo, tmpDir, repoPath, opts); err != nil {
			return fmt.Errorf("prepareRepoCommit: %v", err)
		}

		// Apply changes and commit.
		if err = initRepoCommit(tmpDir, repo, u, opts.DefaultBranch); err != nil {
			return fmt.Errorf("initRepoCommit: %v", err)
		}
	}

	// Re-fetch the repository from database before updating it (else it would
	// override changes that were done earlier with sql)
	if repo, err = models.GetRepositoryByIDCtx(ctx, repo.ID); err != nil {
		return fmt.Errorf("getRepositoryByID: %v", err)
	}

	if !opts.AutoInit {
		repo.IsEmpty = true
	}

	repo.DefaultBranch = "master"
	if len(opts.DefaultBranch) > 0 {
		repo.DefaultBranch = opts.DefaultBranch
		gitRepo, err := git.OpenRepository(repo.RepoPath())
		if err != nil {
			return fmt.Errorf("openRepository: %v", err)
		}
		if err = gitRepo.SetDefaultBranch(repo.DefaultBranch); err != nil {
			return fmt.Errorf("setDefaultBranch: %v", err)
		}
	}

	if err = models.UpdateRepositoryCtx(ctx, repo, false); err != nil {
		return fmt.Errorf("updateRepository: %v", err)
	}

	return nil
}
