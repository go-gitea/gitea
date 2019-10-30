// Copyright 2019 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/mcuadros/go-version"
)

// Merge merges pull request to base repository.
// FIXME: add repoWorkingPull make sure two merges does not happen at same time.
func Merge(pr *models.PullRequest, doer *models.User, baseGitRepo *git.Repository, mergeStyle models.MergeStyle, message string) (err error) {
	binVersion, err := git.BinVersion()
	if err != nil {
		log.Error("git.BinVersion: %v", err)
		return fmt.Errorf("Unable to get git version: %v", err)
	}

	if err = pr.GetHeadRepo(); err != nil {
		log.Error("GetHeadRepo: %v", err)
		return fmt.Errorf("GetHeadRepo: %v", err)
	} else if err = pr.GetBaseRepo(); err != nil {
		log.Error("GetBaseRepo: %v", err)
		return fmt.Errorf("GetBaseRepo: %v", err)
	}

	prUnit, err := pr.BaseRepo.GetUnit(models.UnitTypePullRequests)
	if err != nil {
		log.Error("pr.BaseRepo.GetUnit: %v", err)
		return err
	}
	prConfig := prUnit.PullRequestsConfig()

	if err := pr.CheckUserAllowedToMerge(doer); err != nil {
		log.Error("CheckUserAllowedToMerge(%v): %v", doer, err)
		return fmt.Errorf("CheckUserAllowedToMerge: %v", err)
	}

	// Check if merge style is correct and allowed
	if !prConfig.IsMergeStyleAllowed(mergeStyle) {
		return models.ErrInvalidMergeStyle{ID: pr.BaseRepo.ID, Style: mergeStyle}
	}

	defer func() {
		go AddTestPullRequestTask(doer, pr.BaseRepo.ID, pr.BaseBranch, false)
	}()

	// Clone base repo.
	tmpBasePath, err := models.CreateTemporaryPath("merge")
	if err != nil {
		log.Error("CreateTemporaryPath: %v", err)
		return err
	}

	defer func() {
		if err := models.RemoveTemporaryPath(tmpBasePath); err != nil {
			log.Error("Merge: RemoveTemporaryPath: %s", err)
		}
	}()

	headRepoPath := pr.HeadRepo.RepoPath()

	if err := git.InitRepository(tmpBasePath, false); err != nil {
		log.Error("git init: %v", err)
		return fmt.Errorf("git init: %v", err)
	}

	remoteRepoName := "head_repo"
	baseBranch := "base"

	// Add head repo remote.
	addCacheRepo := func(staging, cache string) error {
		p := filepath.Join(staging, ".git", "objects", "info", "alternates")
		f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.Error("Creating alternates file: %v", err)
			return err
		}
		defer f.Close()
		data := filepath.Join(cache, "objects")
		if _, err := fmt.Fprintln(f, data); err != nil {
			log.Error("Unable to print to alternates file in temporary repository: %v", err)
			return err
		}
		return nil
	}

	if err := addCacheRepo(tmpBasePath, baseGitRepo.Path); err != nil {
		log.Error("addCacheRepo to temporary repo [%s -> %s]: %v", headRepoPath, tmpBasePath, err)
		return fmt.Errorf("addCacheRepo [%s -> %s]: %v", headRepoPath, tmpBasePath, err)
	}

	var outbuf, errbuf strings.Builder
	if err := git.NewCommand("remote", "add", "-t", pr.BaseBranch, "-m", pr.BaseBranch, "origin", baseGitRepo.Path).RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
		log.Error("git remote add [%s -> %s]: %v\n%s\n%s", baseGitRepo.Path, tmpBasePath, err, outbuf.String(), errbuf.String())
		return fmt.Errorf("git remote add [%s -> %s]: %s", baseGitRepo.Path, tmpBasePath, errbuf.String())
	}
	outbuf.Reset()
	errbuf.Reset()

	if err := git.NewCommand("fetch", "origin", "--no-tags", pr.BaseBranch+":"+baseBranch, pr.BaseBranch+":original_"+baseBranch).RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
		log.Error("git fetch [%s -> %s]: %v:\n%s\n%s", headRepoPath, tmpBasePath, err, outbuf.String(), errbuf.String())
		return fmt.Errorf("git fetch [%s -> %s]: %s", headRepoPath, tmpBasePath, errbuf.String())
	}
	outbuf.Reset()
	errbuf.Reset()

	if err := git.NewCommand("symbolic-ref", "HEAD", git.BranchPrefix+baseBranch).RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
		log.Error("git symbolic-ref HEAD base [%s]: %v\n%s\n%s", tmpBasePath, err, outbuf.String(), errbuf.String())
		return fmt.Errorf("git symbolic-ref HEAD base [%s]: %s", tmpBasePath, errbuf.String())
	}
	outbuf.Reset()
	errbuf.Reset()

	if err := addCacheRepo(tmpBasePath, headRepoPath); err != nil {
		log.Error("addCacheRepo [%s -> %s]: %v", headRepoPath, tmpBasePath, err)
		return fmt.Errorf("addCacheRepo [%s -> %s]: %v", headRepoPath, tmpBasePath, err)
	}

	if err := git.NewCommand("remote", "add", remoteRepoName, headRepoPath).RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
		log.Error("git remote add [%s -> %s]: %v\n%s\n%s", headRepoPath, tmpBasePath, err, outbuf.String(), errbuf.String())
		return fmt.Errorf("git remote add [%s -> %s]: %s", headRepoPath, tmpBasePath, errbuf.String())
	}
	outbuf.Reset()
	errbuf.Reset()

	trackingBranch := "tracking"
	// Fetch head branch
	if err := git.NewCommand("fetch", "--no-tags", remoteRepoName, pr.HeadBranch+":"+trackingBranch).RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
		log.Error("git fetch [%s -> %s]: %v\n%s\n%s", headRepoPath, tmpBasePath, err, outbuf.String(), errbuf.String())
		return fmt.Errorf("git fetch [%s -> %s]: %s", headRepoPath, tmpBasePath, errbuf.String())
	}
	outbuf.Reset()
	errbuf.Reset()

	stagingBranch := "staging"

	// Enable sparse-checkout
	sparseCheckoutList, err := getDiffTree(tmpBasePath, baseBranch, trackingBranch)
	if err != nil {
		log.Error("getDiffTree(%s, %s, %s): %v", tmpBasePath, baseBranch, trackingBranch, err)
		return fmt.Errorf("getDiffTree: %v", err)
	}

	infoPath := filepath.Join(tmpBasePath, ".git", "info")
	if err := os.MkdirAll(infoPath, 0700); err != nil {
		log.Error("creating directory failed [%s]: %v", infoPath, err)
		return fmt.Errorf("creating directory failed [%s]: %v", infoPath, err)
	}
	sparseCheckoutListPath := filepath.Join(infoPath, "sparse-checkout")
	if err := ioutil.WriteFile(sparseCheckoutListPath, []byte(sparseCheckoutList), 0600); err != nil {
		log.Error("Writing sparse-checkout file to %s: %v", sparseCheckoutListPath, err)
		return fmt.Errorf("Writing sparse-checkout file to %s: %v", sparseCheckoutListPath, err)
	}

	gitConfigCommand := func() func() *git.Command {
		binVersion, err := git.BinVersion()
		if err != nil {
			log.Error("Error retrieving git version: %v", err)
		}

		if version.Compare(binVersion, "1.8.0", ">=") {
			return func() *git.Command {
				return git.NewCommand("config", "--local")
			}
		}
		return func() *git.Command {
			return git.NewCommand("config")
		}
	}()

	// Switch off LFS process (set required, clean and smudge here also)
	if err := gitConfigCommand().AddArguments("filter.lfs.process", "").RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
		log.Error("git config [filter.lfs.process -> <> ]: %v\n%s\n%s", err, outbuf.String(), errbuf.String())
		return fmt.Errorf("git config [filter.lfs.process -> <> ]: %v", errbuf.String())
	}
	outbuf.Reset()
	errbuf.Reset()

	if err := gitConfigCommand().AddArguments("filter.lfs.required", "false").RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
		log.Error("git config [filter.lfs.required -> <false> ]: %v\n%s\n%s", err, outbuf.String(), errbuf.String())
		return fmt.Errorf("git config [filter.lfs.required -> <false> ]: %v", errbuf.String())
	}
	outbuf.Reset()
	errbuf.Reset()

	if err := gitConfigCommand().AddArguments("filter.lfs.clean", "").RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
		log.Error("git config [filter.lfs.clean -> <> ]: %v\n%s\n%s", err, outbuf.String(), errbuf.String())
		return fmt.Errorf("git config [filter.lfs.clean -> <> ]: %v", errbuf.String())
	}
	outbuf.Reset()
	errbuf.Reset()

	if err := gitConfigCommand().AddArguments("filter.lfs.smudge", "").RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
		log.Error("git config [filter.lfs.smudge -> <> ]: %v\n%s\n%s", err, outbuf.String(), errbuf.String())
		return fmt.Errorf("git config [filter.lfs.smudge -> <> ]: %v", errbuf.String())
	}
	outbuf.Reset()
	errbuf.Reset()

	if err := gitConfigCommand().AddArguments("core.sparseCheckout", "true").RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
		log.Error("git config [core.sparseCheckout -> true ]: %v\n%s\n%s", err, outbuf.String(), errbuf.String())
		return fmt.Errorf("git config [core.sparsecheckout -> true]: %v", errbuf.String())
	}
	outbuf.Reset()
	errbuf.Reset()

	// Read base branch index
	if err := git.NewCommand("read-tree", "HEAD").RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
		log.Error("git read-tree HEAD: %v\n%s\n%s", err, outbuf.String(), errbuf.String())
		return fmt.Errorf("git read-tree HEAD: %s", errbuf.String())
	}
	outbuf.Reset()
	errbuf.Reset()

	// Determine if we should sign
	signArg := ""
	if version.Compare(binVersion, "1.7.9", ">=") {
		sign, keyID := pr.BaseRepo.SignMerge(doer, tmpBasePath, "HEAD", trackingBranch)
		if sign {
			signArg = "-S" + keyID
		} else if version.Compare(binVersion, "2.0.0", ">=") {
			signArg = "--no-gpg-sign"
		}
	}

	sig := doer.NewGitSig()
	commitTimeStr := time.Now().Format(time.RFC3339)

	// Because this may call hooks we should pass in the environment
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME="+sig.Name,
		"GIT_AUTHOR_EMAIL="+sig.Email,
		"GIT_AUTHOR_DATE="+commitTimeStr,
		"GIT_COMMITTER_NAME="+sig.Name,
		"GIT_COMMITTER_EMAIL="+sig.Email,
		"GIT_COMMITTER_DATE="+commitTimeStr,
	)

	// Merge commits.
	switch mergeStyle {
	case models.MergeStyleMerge:
		fallthrough
	case models.MergeStyleMergeUnrelated:
		cmd := git.NewCommand("merge", "--no-ff", "--no-commit")
		if mergeStyle == models.MergeStyleMergeUnrelated && version.Compare(binVersion, "2.9.0", ">=") {
			cmd.AddArguments("--allow-unrelated-histories")
		}
		cmd.AddArguments(trackingBranch)
		if err := cmd.RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
			// Merge will leave a MERGE_HEAD file in the .git folder if there is a conflict
			if _, statErr := os.Stat(filepath.Join(tmpBasePath, ".git", "MERGE_HEAD")); statErr == nil {
				// We have a merge conflict error
				log.Debug("MergeConflict [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
				return models.ErrMergeConflicts{
					Style:  mergeStyle,
					StdOut: outbuf.String(),
					StdErr: errbuf.String(),
					Err:    err,
				}
			} else if strings.Contains(err.Error(), "refusing to merge unrelated histories") {
				log.Debug("MergeUnrelatedHistories [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
				return models.ErrMergeUnrelatedHistories{
					Style:  mergeStyle,
					StdOut: outbuf.String(),
					StdErr: errbuf.String(),
					Err:    err,
				}
			}
			log.Error("git merge --no-ff --no-commit [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
			return fmt.Errorf("git merge --no-ff --no-commit [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
		}
		outbuf.Reset()
		errbuf.Reset()

		if signArg == "" {
			if err := git.NewCommand("commit", "-m", message).RunInDirTimeoutEnvPipeline(env, -1, tmpBasePath, &outbuf, &errbuf); err != nil {
				log.Error("git commit [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
				return fmt.Errorf("git commit [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
			}
		} else {
			if err := git.NewCommand("commit", signArg, "-m", message).RunInDirTimeoutEnvPipeline(env, -1, tmpBasePath, &outbuf, &errbuf); err != nil {
				log.Error("git commit [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
				return fmt.Errorf("git commit [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
			}
		}
		outbuf.Reset()
		errbuf.Reset()
	case models.MergeStyleRebase:
		// Checkout head branch
		if err := git.NewCommand("checkout", "-b", stagingBranch, trackingBranch).RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
			log.Error("git checkout -b staging tracking [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
			return fmt.Errorf("git checkout -b staging tracking [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
		}
		outbuf.Reset()
		errbuf.Reset()

		// Rebase before merging
		if err := git.NewCommand("rebase", baseBranch).RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
			// Rebase will leave a REBASE_HEAD file in .git if there is a conflict
			if _, statErr := os.Stat(filepath.Join(tmpBasePath, ".git", "REBASE_HEAD")); statErr == nil {
				// The original commit SHA1 that is failing will be in .git/rebase-apply/original-commit
				commitShaBytes, readErr := ioutil.ReadFile(filepath.Join(tmpBasePath, ".git", "rebase-apply", "original-commit"))
				if readErr != nil {
					// Abandon this attempt to handle the error
					log.Error("git rebase staging on to base [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
					return fmt.Errorf("git rebase staging on to base [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
				}
				log.Debug("RebaseConflict at %s [%s:%s -> %s:%s]: %v\n%s\n%s", strings.TrimSpace(string(commitShaBytes)), pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
				return models.ErrRebaseConflicts{
					Style:     mergeStyle,
					CommitSHA: strings.TrimSpace(string(commitShaBytes)),
					StdOut:    outbuf.String(),
					StdErr:    errbuf.String(),
					Err:       err,
				}
			}
			log.Error("git rebase staging on to base [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
			return fmt.Errorf("git rebase staging on to base [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
		}
		outbuf.Reset()
		errbuf.Reset()

		// Checkout base branch again
		if err := git.NewCommand("checkout", baseBranch).RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
			log.Error("git checkout base prior to merge post staging rebase [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
			return fmt.Errorf("git checkout base prior to merge post staging rebase  [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
		}
		outbuf.Reset()
		errbuf.Reset()

		// Merge fast forward
		if err := git.NewCommand("merge", "--ff-only", stagingBranch).RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
			// Merge will leave a MERGE_HEAD file in the .git folder if there is a conflict
			if _, statErr := os.Stat(filepath.Join(tmpBasePath, ".git", "MERGE_HEAD")); statErr == nil {
				// We have a merge conflict error
				log.Debug("MergeConflict [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
				return models.ErrMergeConflicts{
					Style:  mergeStyle,
					StdOut: outbuf.String(),
					StdErr: errbuf.String(),
					Err:    err,
				}
			} else if strings.Contains(err.Error(), "refusing to merge unrelated histories") {
				// This shouldn't happen but may aswell check.
				log.Debug("MergeUnrelatedHistories [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
				return models.ErrMergeUnrelatedHistories{
					Style:  mergeStyle,
					StdOut: outbuf.String(),
					StdErr: errbuf.String(),
					Err:    err,
				}
			}
			log.Error("git merge --no-ff [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
			return fmt.Errorf("git merge --no-ff [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
		}
		outbuf.Reset()
		errbuf.Reset()
	case models.MergeStyleRebaseMerge:
		// Checkout head branch
		if err := git.NewCommand("checkout", "-b", stagingBranch, trackingBranch).RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
			log.Error("git checkout base prior to merge post staging rebase [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
			return fmt.Errorf("git checkout base prior to merge post staging rebase  [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
		}
		outbuf.Reset()
		errbuf.Reset()

		// Rebase before merging
		if err := git.NewCommand("rebase", baseBranch).RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
			// Rebase will leave a REBASE_HEAD file in .git if there is a conflict
			if _, statErr := os.Stat(filepath.Join(tmpBasePath, ".git", "REBASE_HEAD")); statErr == nil {
				// The original commit SHA1 that is failing will be in .git/rebase-apply/original-commit
				commitShaBytes, readErr := ioutil.ReadFile(filepath.Join(tmpBasePath, ".git", "rebase-apply", "original-commit"))
				if readErr != nil {
					// Abandon this attempt to handle the error
					log.Error("git rebase staging on to base [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
					return fmt.Errorf("git rebase staging on to base [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
				}
				log.Debug("RebaseConflict at %s [%s:%s -> %s:%s]: %v\n%s\n%s", strings.TrimSpace(string(commitShaBytes)), pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
				return models.ErrRebaseConflicts{
					Style:     mergeStyle,
					CommitSHA: strings.TrimSpace(string(commitShaBytes)),
					StdOut:    outbuf.String(),
					StdErr:    errbuf.String(),
					Err:       err,
				}
			}
			log.Error("git rebase staging on to base [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
			return fmt.Errorf("git rebase staging on to base [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
		}
		outbuf.Reset()
		errbuf.Reset()

		// Checkout base branch again
		if err := git.NewCommand("checkout", baseBranch).RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
			log.Error("git checkout base prior to merge post staging rebase [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
			return fmt.Errorf("git checkout base prior to merge post staging rebase  [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
		}
		outbuf.Reset()
		errbuf.Reset()

		// Prepare merge with commit
		if err := git.NewCommand("merge", "--no-ff", "--no-commit", stagingBranch).RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
			// Merge will leave a MERGE_HEAD file in the .git folder if there is a conflict
			if _, statErr := os.Stat(filepath.Join(tmpBasePath, ".git", "MERGE_HEAD")); statErr == nil {
				// We have a merge conflict error
				log.Debug("MergeConflict [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
				return models.ErrMergeConflicts{
					Style:  mergeStyle,
					StdOut: outbuf.String(),
					StdErr: errbuf.String(),
					Err:    err,
				}
			} else if strings.Contains(err.Error(), "refusing to merge unrelated histories") {
				// This shouldn't happen but may aswell check.
				log.Debug("MergeUnrelatedHistories [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
				return models.ErrMergeUnrelatedHistories{
					Style:  mergeStyle,
					StdOut: outbuf.String(),
					StdErr: errbuf.String(),
					Err:    err,
				}
			}
			log.Error("git merge --no-ff --no-commit [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
			return fmt.Errorf("git merge --no-ff --no-commit [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
		}
		outbuf.Reset()
		errbuf.Reset()

		// Set custom message and author and create merge commit
		if signArg == "" {
			if err := git.NewCommand("commit", "-m", message).RunInDirTimeoutEnvPipeline(env, -1, tmpBasePath, &outbuf, &errbuf); err != nil {
				log.Error("git commit [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
				return fmt.Errorf("git commit [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
			}
		} else {
			if err := git.NewCommand("commit", signArg, "-m", message).RunInDirTimeoutEnvPipeline(env, -1, tmpBasePath, &outbuf, &errbuf); err != nil {
				log.Error("git commit [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
				return fmt.Errorf("git commit [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
			}
		}
		outbuf.Reset()
		errbuf.Reset()
	case models.MergeStyleSquash:
		// Merge with squash
		if err := git.NewCommand("merge", "--squash", trackingBranch).RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
			// Merge will leave a MERGE_MSG file in the .git folder if there is a conflict
			if _, statErr := os.Stat(filepath.Join(tmpBasePath, ".git", "MERGE_MSG")); statErr == nil {
				// We have a merge conflict error
				log.Debug("MergeConflict [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
				return models.ErrMergeConflicts{
					Style:  mergeStyle,
					StdOut: outbuf.String(),
					StdErr: errbuf.String(),
					Err:    err,
				}
			} else if strings.Contains(err.Error(), "refusing to merge unrelated histories") {
				log.Debug("MergeUnrelatedHistories [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
				return models.ErrMergeUnrelatedHistories{
					Style:  mergeStyle,
					StdOut: outbuf.String(),
					StdErr: errbuf.String(),
					Err:    err,
				}
			}
			log.Error("git merge --squash [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
			return fmt.Errorf("git merge --squash [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
		}
		outbuf.Reset()
		errbuf.Reset()

		sig := pr.Issue.Poster.NewGitSig()
		if signArg == "" {
			if err := git.NewCommand("commit", fmt.Sprintf("--author='%s <%s>'", sig.Name, sig.Email), "-m", message).RunInDirTimeoutEnvPipeline(env, -1, tmpBasePath, &outbuf, &errbuf); err != nil {
				log.Error("git commit [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
				return fmt.Errorf("git commit [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
			}
		} else {
			if err := git.NewCommand("commit", signArg, fmt.Sprintf("--author='%s <%s>'", sig.Name, sig.Email), "-m", message).RunInDirTimeoutEnvPipeline(env, -1, tmpBasePath, &outbuf, &errbuf); err != nil {
				log.Error("git commit [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
				return fmt.Errorf("git commit [%s:%s -> %s:%s]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
			}
		}
		outbuf.Reset()
		errbuf.Reset()
	default:
		return models.ErrInvalidMergeStyle{ID: pr.BaseRepo.ID, Style: mergeStyle}
	}

	// OK we should cache our current head and origin/headbranch
	mergeHeadSHA, err := git.GetFullCommitID(tmpBasePath, "HEAD")
	if err != nil {
		return fmt.Errorf("Failed to get full commit id for HEAD: %v", err)
	}
	mergeBaseSHA, err := git.GetFullCommitID(tmpBasePath, "original_"+baseBranch)
	if err != nil {
		return fmt.Errorf("Failed to get full commit id for origin/%s: %v", pr.BaseBranch, err)
	}

	// Now it's questionable about where this should go - either after or before the push
	// I think in the interests of data safety - failures to push to the lfs should prevent
	// the merge as you can always remerge.
	if setting.LFS.StartServer {
		if err := LFSPush(tmpBasePath, mergeHeadSHA, mergeBaseSHA, pr); err != nil {
			return err
		}
	}

	var headUser *models.User
	err = pr.HeadRepo.GetOwner()
	if err != nil {
		if !models.IsErrUserNotExist(err) {
			log.Error("Can't find user: %d for head repository - %v", pr.HeadRepo.OwnerID, err)
			return err
		}
		log.Error("Can't find user: %d for head repository - defaulting to doer: %s - %v", pr.HeadRepo.OwnerID, doer.Name, err)
		headUser = doer
	} else {
		headUser = pr.HeadRepo.Owner
	}

	env = models.FullPushingEnvironment(
		headUser,
		doer,
		pr.BaseRepo,
		pr.BaseRepo.Name,
		pr.ID,
	)

	// Push back to upstream.
	if err := git.NewCommand("push", "origin", baseBranch+":"+pr.BaseBranch).RunInDirTimeoutEnvPipeline(env, -1, tmpBasePath, &outbuf, &errbuf); err != nil {
		if strings.Contains(err.Error(), "non-fast-forward") {
			return models.ErrMergePushOutOfDate{
				Style:  mergeStyle,
				StdOut: outbuf.String(),
				StdErr: errbuf.String(),
				Err:    err,
			}
		}
		return fmt.Errorf("git push: %s", errbuf.String())
	}
	outbuf.Reset()
	errbuf.Reset()

	pr.MergedCommitID, err = baseGitRepo.GetBranchCommitID(pr.BaseBranch)
	if err != nil {
		return fmt.Errorf("GetBranchCommit: %v", err)
	}

	pr.MergedUnix = timeutil.TimeStampNow()
	pr.Merger = doer
	pr.MergerID = doer.ID

	if err = pr.SetMerged(); err != nil {
		log.Error("setMerged [%d]: %v", pr.ID, err)
	}

	if err = models.MergePullRequestAction(doer, pr.Issue.Repo, pr.Issue); err != nil {
		log.Error("MergePullRequestAction [%d]: %v", pr.ID, err)
	}

	// Reset cached commit count
	cache.Remove(pr.Issue.Repo.GetCommitsCountCacheKey(pr.BaseBranch, true))

	// Reload pull request information.
	if err = pr.LoadAttributes(); err != nil {
		log.Error("LoadAttributes: %v", err)
		return nil
	}

	mode, _ := models.AccessLevel(doer, pr.Issue.Repo)
	if err = models.PrepareWebhooks(pr.Issue.Repo, models.HookEventPullRequest, &api.PullRequestPayload{
		Action:      api.HookIssueClosed,
		Index:       pr.Index,
		PullRequest: pr.APIFormat(),
		Repository:  pr.Issue.Repo.APIFormat(mode),
		Sender:      doer.APIFormat(),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	} else {
		go models.HookQueue.Add(pr.Issue.Repo.ID)
	}

	return nil
}

func getDiffTree(repoPath, baseBranch, headBranch string) (string, error) {
	getDiffTreeFromBranch := func(repoPath, baseBranch, headBranch string) (string, error) {
		var outbuf, errbuf strings.Builder
		// Compute the diff-tree for sparse-checkout
		if err := git.NewCommand("diff-tree", "--no-commit-id", "--name-only", "-r", "--root", baseBranch, headBranch, "--").RunInDirPipeline(repoPath, &outbuf, &errbuf); err != nil {
			return "", fmt.Errorf("git diff-tree [%s base:%s head:%s]: %s", repoPath, baseBranch, headBranch, errbuf.String())
		}
		return outbuf.String(), nil
	}

	list, err := getDiffTreeFromBranch(repoPath, baseBranch, headBranch)
	if err != nil {
		return "", err
	}

	// Prefixing '/' for each entry, otherwise all files with the same name in subdirectories would be matched.
	out := bytes.Buffer{}
	scanner := bufio.NewScanner(strings.NewReader(list))
	for scanner.Scan() {
		fmt.Fprintf(&out, "/%s\n", scanner.Text())
	}
	return out.String(), nil
}
