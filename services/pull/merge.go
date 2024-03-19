// Copyright 2019 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	pull_model "code.gitea.io/gitea/models/pull"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/references"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	issue_service "code.gitea.io/gitea/services/issue"
)

// getMergeMessage composes the message used when merging a pull request.
func getMergeMessage(ctx context.Context, baseGitRepo *git.Repository, pr *issues_model.PullRequest, mergeStyle repo_model.MergeStyle, extraVars map[string]string) (message, body string, err error) {
	if err := pr.LoadBaseRepo(ctx); err != nil {
		return "", "", err
	}
	if err := pr.LoadHeadRepo(ctx); err != nil {
		return "", "", err
	}
	if err := pr.LoadIssue(ctx); err != nil {
		return "", "", err
	}
	if err := pr.Issue.LoadPoster(ctx); err != nil {
		return "", "", err
	}

	isExternalTracker := pr.BaseRepo.UnitEnabled(ctx, unit.TypeExternalTracker)
	issueReference := "#"
	if isExternalTracker {
		issueReference = "!"
	}

	if mergeStyle != "" {
		templateFilepath := fmt.Sprintf(".gitea/default_merge_message/%s_TEMPLATE.md", strings.ToUpper(string(mergeStyle)))
		commit, err := baseGitRepo.GetBranchCommit(pr.BaseRepo.DefaultBranch)
		if err != nil {
			return "", "", err
		}
		templateContent, err := commit.GetFileContent(templateFilepath, setting.Repository.PullRequest.DefaultMergeMessageSize)
		if err != nil {
			if !git.IsErrNotExist(err) {
				return "", "", err
			}
		} else {
			vars := map[string]string{
				"BaseRepoOwnerName":      pr.BaseRepo.OwnerName,
				"BaseRepoName":           pr.BaseRepo.Name,
				"BaseBranch":             pr.BaseBranch,
				"HeadRepoOwnerName":      "",
				"HeadRepoName":           "",
				"HeadBranch":             pr.HeadBranch,
				"PullRequestTitle":       pr.Issue.Title,
				"PullRequestDescription": pr.Issue.Content,
				"PullRequestPosterName":  pr.Issue.Poster.Name,
				"PullRequestIndex":       strconv.FormatInt(pr.Index, 10),
				"PullRequestReference":   fmt.Sprintf("%s%d", issueReference, pr.Index),
			}
			if pr.HeadRepo != nil {
				vars["HeadRepoOwnerName"] = pr.HeadRepo.OwnerName
				vars["HeadRepoName"] = pr.HeadRepo.Name
			}
			for extraKey, extraValue := range extraVars {
				vars[extraKey] = extraValue
			}
			refs, err := pr.ResolveCrossReferences(ctx)
			if err == nil {
				closeIssueIndexes := make([]string, 0, len(refs))
				closeWord := "close"
				if len(setting.Repository.PullRequest.CloseKeywords) > 0 {
					closeWord = setting.Repository.PullRequest.CloseKeywords[0]
				}
				for _, ref := range refs {
					if ref.RefAction == references.XRefActionCloses {
						if err := ref.LoadIssue(ctx); err != nil {
							return "", "", err
						}
						closeIssueIndexes = append(closeIssueIndexes, fmt.Sprintf("%s %s%d", closeWord, issueReference, ref.Issue.Index))
					}
				}
				if len(closeIssueIndexes) > 0 {
					vars["ClosingIssues"] = strings.Join(closeIssueIndexes, ", ")
				} else {
					vars["ClosingIssues"] = ""
				}
			}
			message, body = expandDefaultMergeMessage(templateContent, vars)
			return message, body, nil
		}
	}

	if mergeStyle == repo_model.MergeStyleRebase {
		// for fast-forward rebase, do not amend the last commit if there is no template
		return "", "", nil
	}

	// Squash merge has a different from other styles.
	if mergeStyle == repo_model.MergeStyleSquash {
		return fmt.Sprintf("%s (%s%d)", pr.Issue.Title, issueReference, pr.Issue.Index), "", nil
	}

	if pr.BaseRepoID == pr.HeadRepoID {
		return fmt.Sprintf("Merge pull request '%s' (%s%d) from %s into %s", pr.Issue.Title, issueReference, pr.Issue.Index, pr.HeadBranch, pr.BaseBranch), "", nil
	}

	if pr.HeadRepo == nil {
		return fmt.Sprintf("Merge pull request '%s' (%s%d) from <deleted>:%s into %s", pr.Issue.Title, issueReference, pr.Issue.Index, pr.HeadBranch, pr.BaseBranch), "", nil
	}

	return fmt.Sprintf("Merge pull request '%s' (%s%d) from %s:%s into %s", pr.Issue.Title, issueReference, pr.Issue.Index, pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseBranch), "", nil
}

func expandDefaultMergeMessage(template string, vars map[string]string) (message, body string) {
	message = strings.TrimSpace(template)
	if splits := strings.SplitN(message, "\n", 2); len(splits) == 2 {
		message = splits[0]
		body = strings.TrimSpace(splits[1])
	}
	mapping := func(s string) string { return vars[s] }
	return os.Expand(message, mapping), os.Expand(body, mapping)
}

// GetDefaultMergeMessage returns default message used when merging pull request
func GetDefaultMergeMessage(ctx context.Context, baseGitRepo *git.Repository, pr *issues_model.PullRequest, mergeStyle repo_model.MergeStyle) (message, body string, err error) {
	return getMergeMessage(ctx, baseGitRepo, pr, mergeStyle, nil)
}

// Caller should check PR is ready to be merged (review and status checks)
func IsStrategyValid(strategy string) bool {
	return strategy == "theirs" ||
		strategy == "ours" ||
		strategy == "add" ||
		strategy == "delete"
}

func Merge(ctx context.Context, pr *issues_model.PullRequest, doer *user_model.User, baseGitRepo *git.Repository, mergeStyle repo_model.MergeStyle, expectedHeadCommitID, message string, wasAutoMerged bool, strategy []structs.MergeStrategy) error {
	// Merge merges pull request to base repository.
	// Caller should check PR is ready to be merged (review and status checks)
	if err := pr.LoadBaseRepo(ctx); err != nil {
		log.Error("Unable to load base repo: %v", err)
		return fmt.Errorf("unable to load base repo: %w", err)
	} else if err := pr.LoadHeadRepo(ctx); err != nil {
		log.Error("Unable to load head repo: %v", err)
		return fmt.Errorf("unable to load head repo: %w", err)
	}

	pullWorkingPool.CheckIn(fmt.Sprint(pr.ID))
	defer pullWorkingPool.CheckOut(fmt.Sprint(pr.ID))

	// Removing an auto merge pull and ignore if not exist
	// FIXME: is this the correct point to do this? Shouldn't this be after IsMergeStyleAllowed?
	if err := pull_model.DeleteScheduledAutoMerge(ctx, pr.ID); err != nil && !db.IsErrNotExist(err) {
		return err
	}

	prUnit, err := pr.BaseRepo.GetUnit(ctx, unit.TypePullRequests)
	if err != nil {
		log.Error("pr.BaseRepo.GetUnit(unit.TypePullRequests): %v", err)
		return err
	}
	prConfig := prUnit.PullRequestsConfig()

	// Check if merge style is correct and allowed
	if !prConfig.IsMergeStyleAllowed(mergeStyle) {
		return models.ErrInvalidMergeStyle{ID: pr.BaseRepo.ID, Style: mergeStyle}
	}

	defer func() {
		go AddTestPullRequestTask(doer, pr.BaseRepo.ID, pr.BaseBranch, false, "", "")
	}()

	// Run the merge in the hammer context to prevent cancellation
	hammerCtx := graceful.GetManager().HammerContext()

	pr.MergedCommitID, err = doMergeAndPush(hammerCtx, pr, doer, mergeStyle, expectedHeadCommitID, message, strategy)
	if err != nil {
		return err
	}

	pr.MergedUnix = timeutil.TimeStampNow()
	pr.Merger = doer
	pr.MergerID = doer.ID

	if _, err := pr.SetMerged(hammerCtx); err != nil {
		log.Error("SetMerged %-v: %v", pr, err)
	}

	if err := pr.LoadIssue(hammerCtx); err != nil {
		log.Error("LoadIssue %-v: %v", pr, err)
	}

	if err := pr.Issue.LoadRepo(hammerCtx); err != nil {
		log.Error("pr.Issue.LoadRepo %-v: %v", pr, err)
	}
	if err := pr.Issue.Repo.LoadOwner(hammerCtx); err != nil {
		log.Error("LoadOwner for %-v: %v", pr, err)
	}

	if wasAutoMerged {
		notification.NotifyAutoMergePullRequest(hammerCtx, doer, pr)
	} else {
		notification.NotifyMergePullRequest(hammerCtx, doer, pr)
	}

	// Reset cached commit count
	cache.Remove(pr.Issue.Repo.GetCommitsCountCacheKey(pr.BaseBranch, true))

	// Resolve cross references
	refs, err := pr.ResolveCrossReferences(hammerCtx)
	if err != nil {
		log.Error("ResolveCrossReferences: %v", err)
		return nil
	}

	for _, ref := range refs {
		if err = ref.LoadIssue(hammerCtx); err != nil {
			return err
		}
		if err = ref.Issue.LoadRepo(hammerCtx); err != nil {
			return err
		}
		close := ref.RefAction == references.XRefActionCloses
		if close != ref.Issue.IsClosed {
			if err = issue_service.ChangeStatus(ref.Issue, doer, pr.MergedCommitID, close); err != nil {
				// Allow ErrDependenciesLeft
				if !issues_model.IsErrDependenciesLeft(err) {
					return err
				}
			}
		}
	}
	return nil
}

// doMergeAndPush performs the merge operation without changing any pull information in database and pushes it up to the base repository
func doMergeAndPush(ctx context.Context, pr *issues_model.PullRequest, doer *user_model.User, mergeStyle repo_model.MergeStyle, expectedHeadCommitID, message string, strategy []structs.MergeStrategy) (string, error) {
	// Clone base repo.
	mergeCtx, cancel, err := createTemporaryRepoForMerge(ctx, pr, doer, expectedHeadCommitID)
	if err != nil {
		return "", err
	}
	defer cancel()

	// Merge commits.
	switch mergeStyle {
	case repo_model.MergeStyleMerge:
		// Do the merge
		log.Info("Starting a merge")
		cmd := git.NewCommand(ctx, "merge", "--no-ff", "--no-commit").AddDynamicArguments(trackingBranch)
		if err := runMergeCommand(mergeCtx, mergeStyle, cmd); err != nil {
			log.Info("Error while merging: %v", err)

			// If the error was from a merge conflict, that's okay. The strategy might fix it
			if !models.IsErrMergeConflicts(err) || len(strategy) == 0 {
				log.Error("Unable to merge tracking into base: %v", err)
				return "", err
			}
		}

		// Apply the strategy
		log.Info("Beginning to apply strategies: %v", strategy)
		for _, strat := range strategy {
			log.Info("Processing strategy %v", strat)

			if strat.Strategy == "ours" || strat.Strategy == "theirs" {
				strategyFlag := fmt.Sprintf("--%s", strat.Strategy)
				strategyArg := git.ToTrustedCmdArgs([]string{strategyFlag})[0]
				log.Info("Running %s on file %s", strategyFlag, strat.Path)

				// First checkout one or the other
				if err := git.NewCommand(ctx, "checkout").AddOptionValues(strategyArg, strat.Path).Run(&git.RunOpts{
					Dir:    mergeCtx.tmpBasePath,
					Stdout: mergeCtx.outbuf,
					Stderr: mergeCtx.errbuf,
				}); err != nil {
					log.Error("Could not checkout file: %v", err)
					return "", err
				}

				// Then add the changes
				if err := git.NewCommand(ctx, "add").AddDynamicArguments(strat.Path).Run(&git.RunOpts{
					Dir:    mergeCtx.tmpBasePath,
					Stdout: mergeCtx.outbuf,
					Stderr: mergeCtx.errbuf,
				}); err != nil {
					log.Error("Could not add file: %v", err)
					return "", err
				}
			} else if strat.Strategy == "add" {
				log.Info("Running ADD on file %s", strat.Path)
				if err := git.NewCommand(ctx, "add").AddDynamicArguments(strat.Path).Run(&git.RunOpts{
					Dir:    mergeCtx.tmpBasePath,
					Stdout: mergeCtx.outbuf,
					Stderr: mergeCtx.errbuf,
				}); err != nil {
					log.Error("Could not add file: %v", err)
					return "", err
				}
			} else if strat.Strategy == "delete" {
				log.Info("Running DELETE on file %s", strat.Path)
				if err := git.NewCommand(ctx, "rm").AddDynamicArguments(strat.Path).Run(&git.RunOpts{
					Dir:    mergeCtx.tmpBasePath,
					Stdout: mergeCtx.outbuf,
					Stderr: mergeCtx.errbuf,
				}); err != nil {
					log.Error("Could not add file: %v", err)
					return "", err
				}
			} else {
				log.Error("Invalid strategy %s on file %s", strat.Strategy, strat.Path)
				return "", errors.New("invalid strategy")
			}

			log.Info("Done with file %s", strat.Path)
		}

		// Commit the result
		if err := commitAndSignNoAuthor(mergeCtx, message); err != nil {
			log.Error("Unable to make final commit: %v", err)
			return "", err
		}
	case repo_model.MergeStyleRebase, repo_model.MergeStyleRebaseMerge:
		if err := doMergeStyleRebase(mergeCtx, mergeStyle, message); err != nil {
			return "", err
		}
	case repo_model.MergeStyleSquash:
		if err := doMergeStyleSquash(mergeCtx, message); err != nil {
			return "", err
		}
	default:
		return "", models.ErrInvalidMergeStyle{ID: pr.BaseRepo.ID, Style: mergeStyle}
	}

	// OK we should cache our current head and origin/headbranch
	mergeHeadSHA, err := git.GetFullCommitID(ctx, mergeCtx.tmpBasePath, "HEAD")
	if err != nil {
		return "", fmt.Errorf("Failed to get full commit id for HEAD: %w", err)
	}
	mergeBaseSHA, err := git.GetFullCommitID(ctx, mergeCtx.tmpBasePath, "original_"+baseBranch)
	if err != nil {
		return "", fmt.Errorf("Failed to get full commit id for origin/%s: %w", pr.BaseBranch, err)
	}
	mergeCommitID, err := git.GetFullCommitID(ctx, mergeCtx.tmpBasePath, baseBranch)
	if err != nil {
		return "", fmt.Errorf("Failed to get full commit id for the new merge: %w", err)
	}

	// Now it's questionable about where this should go - either after or before the push
	// I think in the interests of data safety - failures to push to the lfs should prevent
	// the merge as you can always remerge.
	if setting.LFS.StartServer {
		if err := LFSPush(ctx, mergeCtx.tmpBasePath, mergeHeadSHA, mergeBaseSHA, pr); err != nil {
			return "", err
		}
	}

	var headUser *user_model.User
	err = pr.HeadRepo.LoadOwner(ctx)
	if err != nil {
		if !user_model.IsErrUserNotExist(err) {
			log.Error("Can't find user: %d for head repository in %-v: %v", pr.HeadRepo.OwnerID, pr, err)
			return "", err
		}
		log.Warn("Can't find user: %d for head repository in %-v - defaulting to doer: %s - %v", pr.HeadRepo.OwnerID, pr, doer.Name, err)
		headUser = doer
	} else {
		headUser = pr.HeadRepo.Owner
	}

	mergeCtx.env = repo_module.FullPushingEnvironment(
		headUser,
		doer,
		pr.BaseRepo,
		pr.BaseRepo.Name,
		pr.ID,
	)
	pushCmd := git.NewCommand(ctx, "push", "origin").AddDynamicArguments(baseBranch + ":" + git.BranchPrefix + pr.BaseBranch)

	// Push back to upstream.
	// TODO: this cause an api call to "/api/internal/hook/post-receive/...",
	//       that prevents us from doint the whole merge in one db transaction
	if err := pushCmd.Run(mergeCtx.RunOpts()); err != nil {
		if strings.Contains(mergeCtx.errbuf.String(), "non-fast-forward") {
			return "", &git.ErrPushOutOfDate{
				StdOut: mergeCtx.outbuf.String(),
				StdErr: mergeCtx.errbuf.String(),
				Err:    err,
			}
		} else if strings.Contains(mergeCtx.errbuf.String(), "! [remote rejected]") {
			err := &git.ErrPushRejected{
				StdOut: mergeCtx.outbuf.String(),
				StdErr: mergeCtx.errbuf.String(),
				Err:    err,
			}
			err.GenerateMessage()
			return "", err
		}
		return "", fmt.Errorf("git push: %s", mergeCtx.errbuf.String())
	}
	mergeCtx.outbuf.Reset()
	mergeCtx.errbuf.Reset()

	return mergeCommitID, nil
}

func commitAndSignNoAuthor(ctx *mergeContext, message string) error {
	cmdCommit := git.NewCommand(ctx, "commit").AddOptionFormat("--message=%s", message)
	if ctx.signKeyID == "" {
		cmdCommit.AddArguments("--no-gpg-sign")
	} else {
		cmdCommit.AddOptionFormat("-S%s", ctx.signKeyID)
	}
	if err := cmdCommit.Run(ctx.RunOpts()); err != nil {
		log.Error("git commit %-v: %v\n%s\n%s", ctx.pr, err, ctx.outbuf.String(), ctx.errbuf.String())
		return fmt.Errorf("git commit %v: %w\n%s\n%s", ctx.pr, err, ctx.outbuf.String(), ctx.errbuf.String())
	}
	return nil
}

func runMergeCommand(ctx *mergeContext, mergeStyle repo_model.MergeStyle, cmd *git.Command) error {
	if err := cmd.Run(ctx.RunOpts()); err != nil {
		// Merge will leave a MERGE_HEAD file in the .git folder if there is a conflict
		if _, statErr := os.Stat(filepath.Join(ctx.tmpBasePath, ".git", "MERGE_HEAD")); statErr == nil {
			// We have a merge conflict error
			log.Debug("MergeConflict %-v: %v\n%s\n%s", ctx.pr, err, ctx.outbuf.String(), ctx.errbuf.String())
			return models.ErrMergeConflicts{
				Style:  mergeStyle,
				StdOut: ctx.outbuf.String(),
				StdErr: ctx.errbuf.String(),
				Err:    err,
			}
		} else if strings.Contains(ctx.errbuf.String(), "refusing to merge unrelated histories") {
			log.Debug("MergeUnrelatedHistories %-v: %v\n%s\n%s", ctx.pr, err, ctx.outbuf.String(), ctx.errbuf.String())
			return models.ErrMergeUnrelatedHistories{
				Style:  mergeStyle,
				StdOut: ctx.outbuf.String(),
				StdErr: ctx.errbuf.String(),
				Err:    err,
			}
		}
		log.Error("git merge %-v: %v\n%s\n%s", ctx.pr, err, ctx.outbuf.String(), ctx.errbuf.String())
		return fmt.Errorf("git merge %v: %w\n%s\n%s", ctx.pr, err, ctx.outbuf.String(), ctx.errbuf.String())
	}
	ctx.outbuf.Reset()
	ctx.errbuf.Reset()

	return nil
}

var escapedSymbols = regexp.MustCompile(`([*[?! \\])`)

// IsUserAllowedToMerge check if user is allowed to merge PR with given permissions and branch protections
func IsUserAllowedToMerge(ctx context.Context, pr *issues_model.PullRequest, p access_model.Permission, user *user_model.User) (bool, error) {
	if user == nil {
		return false, nil
	}

	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, pr.BaseRepoID, pr.BaseBranch)
	if err != nil {
		return false, err
	}

	if (p.CanWrite(unit.TypeCode) && pb == nil) || (pb != nil && git_model.IsUserMergeWhitelisted(ctx, pb, user.ID, p)) {
		return true, nil
	}

	return false, nil
}

// CheckPullBranchProtections checks whether the PR is ready to be merged (reviews and status checks)
func CheckPullBranchProtections(ctx context.Context, pr *issues_model.PullRequest, skipProtectedFilesCheck bool) (err error) {
	if err = pr.LoadBaseRepo(ctx); err != nil {
		return fmt.Errorf("LoadBaseRepo: %w", err)
	}

	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, pr.BaseRepoID, pr.BaseBranch)
	if err != nil {
		return fmt.Errorf("LoadProtectedBranch: %v", err)
	}
	if pb == nil {
		return nil
	}

	isPass, err := IsPullCommitStatusPass(ctx, pr)
	if err != nil {
		return err
	}
	if !isPass {
		return models.ErrDisallowedToMerge{
			Reason: "Not all required status checks successful",
		}
	}

	if !issues_model.HasEnoughApprovals(ctx, pb, pr) {
		return models.ErrDisallowedToMerge{
			Reason: "Does not have enough approvals",
		}
	}
	if issues_model.MergeBlockedByRejectedReview(ctx, pb, pr) {
		return models.ErrDisallowedToMerge{
			Reason: "There are requested changes",
		}
	}
	if issues_model.MergeBlockedByOfficialReviewRequests(ctx, pb, pr) {
		return models.ErrDisallowedToMerge{
			Reason: "There are official review requests",
		}
	}

	if issues_model.MergeBlockedByOutdatedBranch(pb, pr) {
		return models.ErrDisallowedToMerge{
			Reason: "The head branch is behind the base branch",
		}
	}

	if skipProtectedFilesCheck {
		return nil
	}

	if pb.MergeBlockedByProtectedFiles(pr.ChangedProtectedFiles) {
		return models.ErrDisallowedToMerge{
			Reason: "Changed protected files",
		}
	}

	return nil
}

// MergedManually mark pr as merged manually
func MergedManually(pr *issues_model.PullRequest, doer *user_model.User, baseGitRepo *git.Repository, commitID string) error {
	pullWorkingPool.CheckIn(fmt.Sprint(pr.ID))
	defer pullWorkingPool.CheckOut(fmt.Sprint(pr.ID))

	if err := db.WithTx(db.DefaultContext, func(ctx context.Context) error {
		if err := pr.LoadBaseRepo(ctx); err != nil {
			return err
		}
		prUnit, err := pr.BaseRepo.GetUnit(ctx, unit.TypePullRequests)
		if err != nil {
			return err
		}
		prConfig := prUnit.PullRequestsConfig()

		// Check if merge style is correct and allowed
		if !prConfig.IsMergeStyleAllowed(repo_model.MergeStyleManuallyMerged) {
			return models.ErrInvalidMergeStyle{ID: pr.BaseRepo.ID, Style: repo_model.MergeStyleManuallyMerged}
		}

		if len(commitID) < git.SHAFullLength {
			return fmt.Errorf("Wrong commit ID")
		}

		commit, err := baseGitRepo.GetCommit(commitID)
		if err != nil {
			if git.IsErrNotExist(err) {
				return fmt.Errorf("Wrong commit ID")
			}
			return err
		}
		commitID = commit.ID.String()

		ok, err := baseGitRepo.IsCommitInBranch(commitID, pr.BaseBranch)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("Wrong commit ID")
		}

		pr.MergedCommitID = commitID
		pr.MergedUnix = timeutil.TimeStamp(commit.Author.When.Unix())
		pr.Status = issues_model.PullRequestStatusManuallyMerged
		pr.Merger = doer
		pr.MergerID = doer.ID

		var merged bool
		if merged, err = pr.SetMerged(ctx); err != nil {
			return err
		} else if !merged {
			return fmt.Errorf("SetMerged failed")
		}
		return nil
	}); err != nil {
		return err
	}

	notification.NotifyMergePullRequest(baseGitRepo.Ctx, doer, pr)
	log.Info("manuallyMerged[%d]: Marked as manually merged into %s/%s by commit id: %s", pr.ID, pr.BaseRepo.Name, pr.BaseBranch, commitID)
	return nil
}
