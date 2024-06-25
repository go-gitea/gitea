// Copyright 2019 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
)

// Temporary repos created here use standard branch names to help simplify
// merging code
const (
	baseBranch     = "base"     // equivalent to pr.BaseBranch
	trackingBranch = "tracking" // equivalent to pr.HeadBranch
	stagingBranch  = "staging"  // this is used for a working branch
)

type prContext struct {
	context.Context
	tmpBasePath string
	pr          *issues_model.PullRequest
	outbuf      *strings.Builder // we keep these around to help reduce needless buffer recreation,
	errbuf      *strings.Builder // any use should be preceded by a Reset and preferably after use
}

func (ctx *prContext) RunOpts() *git.RunOpts {
	ctx.outbuf.Reset()
	ctx.errbuf.Reset()
	return &git.RunOpts{
		Dir:    ctx.tmpBasePath,
		Stdout: ctx.outbuf,
		Stderr: ctx.errbuf,
	}
}

// createTemporaryRepoForPR creates a temporary repo with "base" for pr.BaseBranch and "tracking" for  pr.HeadBranch
// it also create a second base branch called "original_base"
func createTemporaryRepoForPR(ctx context.Context, pr *issues_model.PullRequest) (prCtx *prContext, cancel context.CancelFunc, err error) {
	if err := pr.LoadHeadRepo(ctx); err != nil {
		log.Error("%-v LoadHeadRepo: %v", pr, err)
		return nil, nil, fmt.Errorf("%v LoadHeadRepo: %w", pr, err)
	} else if pr.HeadRepo == nil {
		log.Error("%-v HeadRepo %d does not exist", pr, pr.HeadRepoID)
		return nil, nil, &repo_model.ErrRepoNotExist{
			ID: pr.HeadRepoID,
		}
	} else if err := pr.LoadBaseRepo(ctx); err != nil {
		log.Error("%-v LoadBaseRepo: %v", pr, err)
		return nil, nil, fmt.Errorf("%v LoadBaseRepo: %w", pr, err)
	} else if pr.BaseRepo == nil {
		log.Error("%-v BaseRepo %d does not exist", pr, pr.BaseRepoID)
		return nil, nil, &repo_model.ErrRepoNotExist{
			ID: pr.BaseRepoID,
		}
	} else if err := pr.HeadRepo.LoadOwner(ctx); err != nil {
		log.Error("%-v HeadRepo.LoadOwner: %v", pr, err)
		return nil, nil, fmt.Errorf("%v HeadRepo.LoadOwner: %w", pr, err)
	} else if err := pr.BaseRepo.LoadOwner(ctx); err != nil {
		log.Error("%-v BaseRepo.LoadOwner: %v", pr, err)
		return nil, nil, fmt.Errorf("%v BaseRepo.LoadOwner: %w", pr, err)
	}

	// Clone base repo.
	tmpBasePath, err := repo_module.CreateTemporaryPath("pull")
	if err != nil {
		log.Error("CreateTemporaryPath[%-v]: %v", pr, err)
		return nil, nil, err
	}
	prCtx = &prContext{
		Context:     ctx,
		tmpBasePath: tmpBasePath,
		pr:          pr,
		outbuf:      &strings.Builder{},
		errbuf:      &strings.Builder{},
	}
	cancel = func() {
		if err := repo_module.RemoveTemporaryPath(tmpBasePath); err != nil {
			log.Error("Error whilst removing removing temporary repo for %-v: %v", pr, err)
		}
	}

	baseRepoPath := pr.BaseRepo.RepoPath()
	headRepoPath := pr.HeadRepo.RepoPath()

	if err := git.InitRepository(ctx, tmpBasePath, false, pr.BaseRepo.ObjectFormatName); err != nil {
		log.Error("Unable to init tmpBasePath for %-v: %v", pr, err)
		cancel()
		return nil, nil, err
	}

	remoteRepoName := "head_repo"
	baseBranch := "base"

	fetchArgs := git.TrustedCmdArgs{"--no-tags"}
	if git.DefaultFeatures().CheckVersionAtLeast("2.25.0") {
		// Writing the commit graph can be slow and is not needed here
		fetchArgs = append(fetchArgs, "--no-write-commit-graph")
	}

	// addCacheRepo adds git alternatives for the cacheRepoPath in the repoPath
	addCacheRepo := func(repoPath, cacheRepoPath string) error {
		p := filepath.Join(repoPath, ".git", "objects", "info", "alternates")
		f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			log.Error("Could not create .git/objects/info/alternates file in %s: %v", repoPath, err)
			return err
		}
		defer f.Close()
		data := filepath.Join(cacheRepoPath, "objects")
		if _, err := fmt.Fprintln(f, data); err != nil {
			log.Error("Could not write to .git/objects/info/alternates file in %s: %v", repoPath, err)
			return err
		}
		return nil
	}

	// Add head repo remote.
	if err := addCacheRepo(tmpBasePath, baseRepoPath); err != nil {
		log.Error("%-v Unable to add base repository to temporary repo [%s -> %s]: %v", pr, pr.BaseRepo.FullName(), tmpBasePath, err)
		cancel()
		return nil, nil, fmt.Errorf("Unable to add base repository to temporary repo [%s -> tmpBasePath]: %w", pr.BaseRepo.FullName(), err)
	}

	if err := git.NewCommand(ctx, "remote", "add", "-t").AddDynamicArguments(pr.BaseBranch).AddArguments("-m").AddDynamicArguments(pr.BaseBranch).AddDynamicArguments("origin", baseRepoPath).
		Run(prCtx.RunOpts()); err != nil {
		log.Error("%-v Unable to add base repository as origin [%s -> %s]: %v\n%s\n%s", pr, pr.BaseRepo.FullName(), tmpBasePath, err, prCtx.outbuf.String(), prCtx.errbuf.String())
		cancel()
		return nil, nil, fmt.Errorf("Unable to add base repository as origin [%s -> tmpBasePath]: %w\n%s\n%s", pr.BaseRepo.FullName(), err, prCtx.outbuf.String(), prCtx.errbuf.String())
	}

	if err := git.NewCommand(ctx, "fetch", "origin").AddArguments(fetchArgs...).AddDashesAndList(pr.BaseBranch+":"+baseBranch, pr.BaseBranch+":original_"+baseBranch).
		Run(prCtx.RunOpts()); err != nil {
		log.Error("%-v Unable to fetch origin base branch [%s:%s -> base, original_base in %s]: %v:\n%s\n%s", pr, pr.BaseRepo.FullName(), pr.BaseBranch, tmpBasePath, err, prCtx.outbuf.String(), prCtx.errbuf.String())
		cancel()
		return nil, nil, fmt.Errorf("Unable to fetch origin base branch [%s:%s -> base, original_base in tmpBasePath]: %w\n%s\n%s", pr.BaseRepo.FullName(), pr.BaseBranch, err, prCtx.outbuf.String(), prCtx.errbuf.String())
	}

	if err := git.NewCommand(ctx, "symbolic-ref").AddDynamicArguments("HEAD", git.BranchPrefix+baseBranch).
		Run(prCtx.RunOpts()); err != nil {
		log.Error("%-v Unable to set HEAD as base branch in [%s]: %v\n%s\n%s", pr, tmpBasePath, err, prCtx.outbuf.String(), prCtx.errbuf.String())
		cancel()
		return nil, nil, fmt.Errorf("Unable to set HEAD as base branch in tmpBasePath: %w\n%s\n%s", err, prCtx.outbuf.String(), prCtx.errbuf.String())
	}

	if err := addCacheRepo(tmpBasePath, headRepoPath); err != nil {
		log.Error("%-v Unable to add head repository to temporary repo [%s -> %s]: %v", pr, pr.HeadRepo.FullName(), tmpBasePath, err)
		cancel()
		return nil, nil, fmt.Errorf("Unable to add head base repository to temporary repo [%s -> tmpBasePath]: %w", pr.HeadRepo.FullName(), err)
	}

	if err := git.NewCommand(ctx, "remote", "add").AddDynamicArguments(remoteRepoName, headRepoPath).
		Run(prCtx.RunOpts()); err != nil {
		log.Error("%-v Unable to add head repository as head_repo [%s -> %s]: %v\n%s\n%s", pr, pr.HeadRepo.FullName(), tmpBasePath, err, prCtx.outbuf.String(), prCtx.errbuf.String())
		cancel()
		return nil, nil, fmt.Errorf("Unable to add head repository as head_repo [%s -> tmpBasePath]: %w\n%s\n%s", pr.HeadRepo.FullName(), err, prCtx.outbuf.String(), prCtx.errbuf.String())
	}

	trackingBranch := "tracking"
	objectFormat := git.ObjectFormatFromName(pr.BaseRepo.ObjectFormatName)
	// Fetch head branch
	var headBranch string
	if pr.Flow == issues_model.PullRequestFlowGithub {
		headBranch = git.BranchPrefix + pr.HeadBranch
	} else if len(pr.HeadCommitID) == objectFormat.FullLength() { // for not created pull request
		headBranch = pr.HeadCommitID
	} else {
		headBranch = pr.GetGitRefName()
	}
	if err := git.NewCommand(ctx, "fetch").AddArguments(fetchArgs...).AddDynamicArguments(remoteRepoName, headBranch+":"+trackingBranch).
		Run(prCtx.RunOpts()); err != nil {
		cancel()
		if !git.IsBranchExist(ctx, pr.HeadRepo.RepoPath(), pr.HeadBranch) {
			return nil, nil, git_model.ErrBranchNotExist{
				BranchName: pr.HeadBranch,
			}
		}
		log.Error("%-v Unable to fetch head_repo head branch [%s:%s -> tracking in %s]: %v:\n%s\n%s", pr, pr.HeadRepo.FullName(), pr.HeadBranch, tmpBasePath, err, prCtx.outbuf.String(), prCtx.errbuf.String())
		return nil, nil, fmt.Errorf("Unable to fetch head_repo head branch [%s:%s -> tracking in tmpBasePath]: %w\n%s\n%s", pr.HeadRepo.FullName(), headBranch, err, prCtx.outbuf.String(), prCtx.errbuf.String())
	}
	prCtx.outbuf.Reset()
	prCtx.errbuf.Reset()

	return prCtx, cancel, nil
}
