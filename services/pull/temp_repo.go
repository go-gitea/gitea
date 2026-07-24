// Copyright 2019 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
	repo_module "gitea.dev/modules/repository"
)

// Temporary repos created here use standard branch names to help simplify
// merging code
const (
	tmpRepoBaseBranch     = "base"     // equivalent to pr.BaseBranch
	tmpRepoTrackingBranch = "tracking" // equivalent to pr.HeadBranch
	tmpRepoStagingBranch  = "staging"  // this is used for a working branch
)

type prTmpRepoContext struct {
	context.Context
	tmpBasePath string
	tmpRepo     git.RepositoryFacade
	pr          *issues_model.PullRequest
	outbuf      *bytes.Buffer // we keep these around to help reduce needless buffer recreation, any use should be preceded by a Reset and preferably after use
}

// PrepareGitCmd prepares a git command with the correct directory, environment, and output buffers
// This function can only be called with gitcmd.Run()
// Do NOT use it with gitcmd.RunStd*() functions, otherwise it will panic
func (ctx *prTmpRepoContext) PrepareGitCmd(cmd *gitcmd.Command) *gitcmd.Command {
	ctx.outbuf.Reset()
	return cmd.WithRepo(ctx.tmpRepo).WithStdoutBuffer(ctx.outbuf)
}

// createTemporaryRepoForPR creates a temporary repo with "base" for pr.BaseBranch and "tracking" for  pr.HeadBranch
// it also create a second base branch called "original_base"
func createTemporaryRepoForPR(ctx context.Context, pr *issues_model.PullRequest) (prCtx *prTmpRepoContext, cancel context.CancelFunc, retErr error) {
	defer func() {
		if retErr != nil && cancel != nil {
			cancel()
		}
	}()
	if err := pr.LoadHeadRepo(ctx); err != nil {
		return nil, nil, fmt.Errorf("LoadHeadRepo[PR:%d]: %w", pr.ID, err)
	} else if pr.HeadRepo == nil {
		return nil, nil, &repo_model.ErrRepoNotExist{ID: pr.HeadRepoID}
	} else if err := pr.LoadBaseRepo(ctx); err != nil {
		return nil, nil, fmt.Errorf("LoadBaseRepo[PR:%d]: %w", pr.ID, err)
	} else if pr.BaseRepo == nil {
		return nil, nil, &repo_model.ErrRepoNotExist{ID: pr.BaseRepoID}
	} else if err := pr.HeadRepo.LoadOwner(ctx); err != nil {
		return nil, nil, fmt.Errorf("HeadRepo.LoadOwner[PR:%d]: %w", pr.ID, err)
	} else if err := pr.BaseRepo.LoadOwner(ctx); err != nil {
		return nil, nil, fmt.Errorf("BaseRepo.LoadOwner[PR:%d]: %w", pr.ID, err)
	}

	// Clone base repo.
	tmpBasePath, tmpRepo, cleanup, err := repo_module.CreateTemporaryGitRepo("pull")
	if err != nil {
		return nil, nil, fmt.Errorf("CreateTemporaryPath[PR:%d]: %w", pr.ID, err)
	}
	cancel = cleanup

	prCtx = &prTmpRepoContext{
		Context:     ctx,
		tmpBasePath: tmpBasePath,
		tmpRepo:     tmpRepo,
		pr:          pr,
		outbuf:      &bytes.Buffer{},
	}

	baseRepoPath := gitrepo.RepoLocalPath(pr.BaseRepo.CodeStorageRepo())
	headRepoPath := gitrepo.RepoLocalPath(pr.HeadRepo.CodeStorageRepo())

	if err := git.InitRepositoryLocal(ctx, tmpBasePath, false, pr.BaseRepo.ObjectFormatName); err != nil {
		return nil, nil, fmt.Errorf("InitRepository[PR:%d]: %w", pr.ID, err)
	}

	remoteRepoName := "head_repo"

	fetchArgs := gitcmd.TrustedCmdArgs{"--no-tags"}
	if git.DefaultFeatures().CheckVersionAtLeast("2.25.0") {
		// Writing the commit graph can be slow and is not needed here
		fetchArgs = append(fetchArgs, "--no-write-commit-graph")
	}

	// addCacheRepo adds git alternatives for the cacheRepoPath in the repoPath
	addCacheRepo := func(repoPath, cacheRepoPath string) error {
		const altFileRelPath = ".git/objects/info/alternates"
		gitInfoAltFile := filepath.Join(repoPath, filepath.FromSlash(altFileRelPath))
		newAltLine := filepath.Join(cacheRepoPath, "objects")

		f, err := os.OpenFile(gitInfoAltFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("unable to open %s file in %s: %w", altFileRelPath, filepath.Base(repoPath), err)
		}
		defer f.Close()
		if _, err := fmt.Fprintln(f, newAltLine); err != nil {
			return fmt.Errorf("unable to write %s file in %s: %w", altFileRelPath, filepath.Base(repoPath), err)
		}
		return nil
	}

	// Add head repo remote.
	if err := addCacheRepo(tmpBasePath, baseRepoPath); err != nil {
		return nil, nil, fmt.Errorf("unable to add base repository to temporary repo [%s -> tmpBasePath]: %w", pr.BaseRepo.FullName(), err)
	}

	if err := prCtx.PrepareGitCmd(gitcmd.NewCommand("remote", "add", "-t").AddDynamicArguments(pr.BaseBranch).AddArguments("-m").AddDynamicArguments(pr.BaseBranch).AddDynamicArguments("origin", baseRepoPath)).
		RunWithStderr(ctx); err != nil {
		return nil, nil, fmt.Errorf("unable to add base repository as origin [%s -> tmpBasePath]: %w\n%s\n%s", pr.BaseRepo.FullName(), err, prCtx.outbuf.String(), err.Stderr())
	}

	if err := prCtx.PrepareGitCmd(gitcmd.NewCommand("fetch", "origin").AddArguments(fetchArgs...).
		AddDashesAndList(git.BranchPrefix+pr.BaseBranch+":"+git.BranchPrefix+tmpRepoBaseBranch, git.BranchPrefix+pr.BaseBranch+":"+git.BranchPrefix+"original_"+tmpRepoBaseBranch)).
		RunWithStderr(ctx); err != nil {
		return nil, nil, fmt.Errorf("unable to fetch origin base branch [%s:%s -> base, original_base in tmpBasePath]: %w\n%s\n%s", pr.BaseRepo.FullName(), pr.BaseBranch, err, prCtx.outbuf.String(), err.Stderr())
	}

	if err := prCtx.PrepareGitCmd(gitcmd.NewCommand("symbolic-ref").AddDynamicArguments("HEAD", git.BranchPrefix+tmpRepoBaseBranch)).
		RunWithStderr(ctx); err != nil {
		return nil, nil, fmt.Errorf("unable to set HEAD as base branch in tmpBasePath: %w\n%s\n%s", err, prCtx.outbuf.String(), err.Stderr())
	}

	if err := addCacheRepo(tmpBasePath, headRepoPath); err != nil {
		return nil, nil, fmt.Errorf("unable to add head base repository to temporary repo [%s -> tmpBasePath]: %w", pr.HeadRepo.FullName(), err)
	}

	if err := prCtx.PrepareGitCmd(gitcmd.NewCommand("remote", "add").AddDynamicArguments(remoteRepoName, headRepoPath)).
		RunWithStderr(ctx); err != nil {
		return nil, nil, fmt.Errorf("unable to add head repository as head_repo [%s -> tmpBasePath]: %w\n%s\n%s", pr.HeadRepo.FullName(), err, prCtx.outbuf.String(), err.Stderr())
	}

	objectFormat := git.ObjectFormatFromName(pr.BaseRepo.ObjectFormatName)
	// Fetch head branch
	var headBranch string
	if pr.Flow == issues_model.PullRequestFlowGithub {
		headBranch = git.BranchPrefix + pr.HeadBranch
	} else if len(pr.HeadCommitID) == objectFormat.FullLength() { // for not created pull request
		headBranch = pr.HeadCommitID
	} else {
		headBranch = pr.GetGitHeadRefName()
	}
	if err := prCtx.PrepareGitCmd(gitcmd.NewCommand("fetch").AddArguments(fetchArgs...).AddDynamicArguments(remoteRepoName, headBranch+":"+tmpRepoTrackingBranch)).
		RunWithStderr(ctx); err != nil {
		if exist, _ := git_model.IsBranchExist(ctx, pr.HeadRepo.ID, pr.HeadBranch); !exist {
			return nil, nil, git_model.ErrBranchNotExist{BranchName: pr.HeadBranch}
		}
		return nil, nil, fmt.Errorf("unable to fetch head_repo head branch [%s:%s -> tracking in tmpBasePath]: %w\n%s\n%s", pr.HeadRepo.FullName(), headBranch, err, prCtx.outbuf.String(), err.Stderr())
	}
	prCtx.outbuf.Reset()
	return prCtx, cancel, nil
}
