// Copyright 2019 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

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
	"code.gitea.io/gitea/modules/globallock"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/references"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	issue_service "code.gitea.io/gitea/services/issue"
	notify_service "code.gitea.io/gitea/services/notify"
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
	if err := pr.Issue.LoadRepo(ctx); err != nil {
		return "", "", err
	}

	isExternalTracker := pr.BaseRepo.UnitEnabled(ctx, unit.TypeExternalTracker)
	issueReference := "#"
	if isExternalTracker {
		issueReference = "!"
	}

	reviewedOn := fmt.Sprintf("Reviewed-on: %s", httplib.MakeAbsoluteURL(ctx, pr.Issue.Link()))
	reviewedBy := pr.GetApprovers(ctx)

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
				"ReviewedOn":             reviewedOn,
				"ReviewedBy":             reviewedBy,
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

	body = fmt.Sprintf("%s\n%s", reviewedOn, reviewedBy)

	// Squash merge has a different from other styles.
	if mergeStyle == repo_model.MergeStyleSquash {
		return fmt.Sprintf("%s (%s%d)", pr.Issue.Title, issueReference, pr.Issue.Index), body, nil
	}

	if pr.BaseRepoID == pr.HeadRepoID {
		return fmt.Sprintf("Merge pull request '%s' (%s%d) from %s into %s", pr.Issue.Title, issueReference, pr.Issue.Index, pr.HeadBranch, pr.BaseBranch), body, nil
	}

	if pr.HeadRepo == nil {
		return fmt.Sprintf("Merge pull request '%s' (%s%d) from <deleted>:%s into %s", pr.Issue.Title, issueReference, pr.Issue.Index, pr.HeadBranch, pr.BaseBranch), body, nil
	}

	return fmt.Sprintf("Merge pull request '%s' (%s%d) from %s:%s into %s", pr.Issue.Title, issueReference, pr.Issue.Index, pr.HeadRepo.FullName(), pr.HeadBranch, pr.BaseBranch), body, nil
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

// ErrInvalidMergeStyle represents an error if merging with disabled merge strategy
type ErrInvalidMergeStyle struct {
	ID    int64
	Style repo_model.MergeStyle
}

// IsErrInvalidMergeStyle checks if an error is a ErrInvalidMergeStyle.
func IsErrInvalidMergeStyle(err error) bool {
	_, ok := err.(ErrInvalidMergeStyle)
	return ok
}

func (err ErrInvalidMergeStyle) Error() string {
	return fmt.Sprintf("merge strategy is not allowed or is invalid [repo_id: %d, strategy: %s]",
		err.ID, err.Style)
}

func (err ErrInvalidMergeStyle) Unwrap() error {
	return util.ErrInvalidArgument
}

// Merge merges pull request to base repository.
// Caller should check PR is ready to be merged (review and status checks)
func Merge(ctx context.Context, pr *issues_model.PullRequest, doer *user_model.User, baseGitRepo *git.Repository, mergeStyle repo_model.MergeStyle, expectedHeadCommitID, message string, wasAutoMerged bool) error {
	if err := pr.LoadBaseRepo(ctx); err != nil {
		log.Error("Unable to load base repo: %v", err)
		return fmt.Errorf("unable to load base repo: %w", err)
	} else if err := pr.LoadHeadRepo(ctx); err != nil {
		log.Error("Unable to load head repo: %v", err)
		return fmt.Errorf("unable to load head repo: %w", err)
	}

	prUnit, err := pr.BaseRepo.GetUnit(ctx, unit.TypePullRequests)
	if err != nil {
		log.Error("pr.BaseRepo.GetUnit(unit.TypePullRequests): %v", err)
		return err
	}
	prConfig := prUnit.PullRequestsConfig()

	// Check if merge style is correct and allowed
	if !prConfig.IsMergeStyleAllowed(mergeStyle) {
		return ErrInvalidMergeStyle{ID: pr.BaseRepo.ID, Style: mergeStyle}
	}

	releaser, err := globallock.Lock(ctx, getPullWorkingLockKey(pr.ID))
	if err != nil {
		log.Error("lock.Lock(): %v", err)
		return fmt.Errorf("lock.Lock: %w", err)
	}
	defer releaser()
	defer func() {
		go AddTestPullRequestTask(doer, pr.BaseRepo.ID, pr.BaseBranch, false, "", "")
	}()

	_, err = doMergeAndPush(ctx, pr, doer, mergeStyle, expectedHeadCommitID, message, repo_module.PushTriggerPRMergeToBase)
	releaser()
	if err != nil {
		return err
	}

	// reload pull request because it has been updated by post receive hook
	pr, err = issues_model.GetPullRequestByID(ctx, pr.ID)
	if err != nil {
		return err
	}

	if err := pr.LoadIssue(ctx); err != nil {
		log.Error("LoadIssue %-v: %v", pr, err)
	}

	if err := pr.Issue.LoadRepo(ctx); err != nil {
		log.Error("pr.Issue.LoadRepo %-v: %v", pr, err)
	}
	if err := pr.Issue.Repo.LoadOwner(ctx); err != nil {
		log.Error("LoadOwner for %-v: %v", pr, err)
	}

	if wasAutoMerged {
		notify_service.AutoMergePullRequest(ctx, doer, pr)
	} else {
		notify_service.MergePullRequest(ctx, doer, pr)
	}

	// Reset cached commit count
	cache.Remove(pr.Issue.Repo.GetCommitsCountCacheKey(pr.BaseBranch, true))

	return handleCloseCrossReferences(ctx, pr, doer)
}

func handleCloseCrossReferences(ctx context.Context, pr *issues_model.PullRequest, doer *user_model.User) error {
	// Resolve cross references
	refs, err := pr.ResolveCrossReferences(ctx)
	if err != nil {
		log.Error("ResolveCrossReferences: %v", err)
		return nil
	}

	for _, ref := range refs {
		if err = ref.LoadIssue(ctx); err != nil {
			return err
		}
		if err = ref.Issue.LoadRepo(ctx); err != nil {
			return err
		}
		if ref.RefAction == references.XRefActionCloses && !ref.Issue.IsClosed {
			if err = issue_service.CloseIssue(ctx, ref.Issue, doer, pr.MergedCommitID); err != nil {
				// Allow ErrDependenciesLeft
				if !issues_model.IsErrDependenciesLeft(err) {
					return err
				}
			}
		} else if ref.RefAction == references.XRefActionReopens && ref.Issue.IsClosed {
			if err = issue_service.ReopenIssue(ctx, ref.Issue, doer, pr.MergedCommitID); err != nil {
				return err
			}
		}
	}
	return nil
}

// doMergeAndPush performs the merge operation without changing any pull information in database and pushes it up to the base repository
func doMergeAndPush(ctx context.Context, pr *issues_model.PullRequest, doer *user_model.User, mergeStyle repo_model.MergeStyle, expectedHeadCommitID, message string, pushTrigger repo_module.PushTrigger) (string, error) { //nolint:unparam
	// Clone base repo.
	mergeCtx, cancel, err := createTemporaryRepoForMerge(ctx, pr, doer, expectedHeadCommitID)
	if err != nil {
		return "", err
	}
	defer cancel()

	// Merge commits.
	switch mergeStyle {
	case repo_model.MergeStyleMerge:
		if err := doMergeStyleMerge(mergeCtx, message); err != nil {
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
	case repo_model.MergeStyleFastForwardOnly:
		if err := doMergeStyleFastForwardOnly(mergeCtx); err != nil {
			return "", err
		}
	default:
		return "", ErrInvalidMergeStyle{ID: pr.BaseRepo.ID, Style: mergeStyle}
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

	mergeCtx.env = append(mergeCtx.env, repo_module.EnvPushTrigger+"="+string(pushTrigger))
	pushCmd := git.NewCommand(ctx, "push", "origin").AddDynamicArguments(baseBranch + ":" + git.BranchPrefix + pr.BaseBranch)

	// Push back to upstream.
	// This cause an api call to "/api/internal/hook/post-receive/...",
	// If it's merge, all db transaction and operations should be there but not here to prevent deadlock.
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

// ErrMergeConflicts represents an error if merging fails with a conflict
type ErrMergeConflicts struct {
	Style  repo_model.MergeStyle
	StdOut string
	StdErr string
	Err    error
}

// IsErrMergeConflicts checks if an error is a ErrMergeConflicts.
func IsErrMergeConflicts(err error) bool {
	_, ok := err.(ErrMergeConflicts)
	return ok
}

func (err ErrMergeConflicts) Error() string {
	return fmt.Sprintf("Merge Conflict Error: %v: %s\n%s", err.Err, err.StdErr, err.StdOut)
}

// ErrMergeUnrelatedHistories represents an error if merging fails due to unrelated histories
type ErrMergeUnrelatedHistories struct {
	Style  repo_model.MergeStyle
	StdOut string
	StdErr string
	Err    error
}

// IsErrMergeUnrelatedHistories checks if an error is a ErrMergeUnrelatedHistories.
func IsErrMergeUnrelatedHistories(err error) bool {
	_, ok := err.(ErrMergeUnrelatedHistories)
	return ok
}

func (err ErrMergeUnrelatedHistories) Error() string {
	return fmt.Sprintf("Merge UnrelatedHistories Error: %v: %s\n%s", err.Err, err.StdErr, err.StdOut)
}

// ErrMergeDivergingFastForwardOnly represents an error if a fast-forward-only merge fails because the branches diverge
type ErrMergeDivergingFastForwardOnly struct {
	StdOut string
	StdErr string
	Err    error
}

// IsErrMergeDivergingFastForwardOnly checks if an error is a ErrMergeDivergingFastForwardOnly.
func IsErrMergeDivergingFastForwardOnly(err error) bool {
	_, ok := err.(ErrMergeDivergingFastForwardOnly)
	return ok
}

func (err ErrMergeDivergingFastForwardOnly) Error() string {
	return fmt.Sprintf("Merge DivergingFastForwardOnly Error: %v: %s\n%s", err.Err, err.StdErr, err.StdOut)
}

func runMergeCommand(ctx *mergeContext, mergeStyle repo_model.MergeStyle, cmd *git.Command) error {
	if err := cmd.Run(ctx.RunOpts()); err != nil {
		// Merge will leave a MERGE_HEAD file in the .git folder if there is a conflict
		if _, statErr := os.Stat(filepath.Join(ctx.tmpBasePath, ".git", "MERGE_HEAD")); statErr == nil {
			// We have a merge conflict error
			log.Debug("MergeConflict %-v: %v\n%s\n%s", ctx.pr, err, ctx.outbuf.String(), ctx.errbuf.String())
			return ErrMergeConflicts{
				Style:  mergeStyle,
				StdOut: ctx.outbuf.String(),
				StdErr: ctx.errbuf.String(),
				Err:    err,
			}
		} else if strings.Contains(ctx.errbuf.String(), "refusing to merge unrelated histories") {
			log.Debug("MergeUnrelatedHistories %-v: %v\n%s\n%s", ctx.pr, err, ctx.outbuf.String(), ctx.errbuf.String())
			return ErrMergeUnrelatedHistories{
				Style:  mergeStyle,
				StdOut: ctx.outbuf.String(),
				StdErr: ctx.errbuf.String(),
				Err:    err,
			}
		} else if mergeStyle == repo_model.MergeStyleFastForwardOnly && strings.Contains(ctx.errbuf.String(), "Not possible to fast-forward, aborting") {
			log.Debug("MergeDivergingFastForwardOnly %-v: %v\n%s\n%s", ctx.pr, err, ctx.outbuf.String(), ctx.errbuf.String())
			return ErrMergeDivergingFastForwardOnly{
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

// ErrDisallowedToMerge represents an error that a branch is protected and the current user is not allowed to modify it.
type ErrDisallowedToMerge struct {
	Reason string
}

// IsErrDisallowedToMerge checks if an error is an ErrDisallowedToMerge.
func IsErrDisallowedToMerge(err error) bool {
	_, ok := err.(ErrDisallowedToMerge)
	return ok
}

func (err ErrDisallowedToMerge) Error() string {
	return fmt.Sprintf("not allowed to merge [reason: %s]", err.Reason)
}

func (err ErrDisallowedToMerge) Unwrap() error {
	return util.ErrPermissionDenied
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
		return ErrDisallowedToMerge{
			Reason: "Not all required status checks successful",
		}
	}

	if !issues_model.HasEnoughApprovals(ctx, pb, pr) {
		return ErrDisallowedToMerge{
			Reason: "Does not have enough approvals",
		}
	}
	if issues_model.MergeBlockedByRejectedReview(ctx, pb, pr) {
		return ErrDisallowedToMerge{
			Reason: "There are requested changes",
		}
	}
	if issues_model.MergeBlockedByOfficialReviewRequests(ctx, pb, pr) {
		return ErrDisallowedToMerge{
			Reason: "There are official review requests",
		}
	}

	if issues_model.MergeBlockedByOutdatedBranch(pb, pr) {
		return ErrDisallowedToMerge{
			Reason: "The head branch is behind the base branch",
		}
	}

	if skipProtectedFilesCheck {
		return nil
	}

	if pb.MergeBlockedByProtectedFiles(pr.ChangedProtectedFiles) {
		return ErrDisallowedToMerge{
			Reason: "Changed protected files",
		}
	}

	return nil
}

// MergedManually mark pr as merged manually
func MergedManually(ctx context.Context, pr *issues_model.PullRequest, doer *user_model.User, baseGitRepo *git.Repository, commitID string) error {
	releaser, err := globallock.Lock(ctx, getPullWorkingLockKey(pr.ID))
	if err != nil {
		log.Error("lock.Lock(): %v", err)
		return fmt.Errorf("lock.Lock: %w", err)
	}
	defer releaser()

	err = db.WithTx(ctx, func(ctx context.Context) error {
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
			return ErrInvalidMergeStyle{ID: pr.BaseRepo.ID, Style: repo_model.MergeStyleManuallyMerged}
		}

		objectFormat := git.ObjectFormatFromName(pr.BaseRepo.ObjectFormatName)
		if len(commitID) != objectFormat.FullLength() {
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

		var merged bool
		if merged, err = SetMerged(ctx, pr, commitID, timeutil.TimeStamp(commit.Author.When.Unix()), doer, issues_model.PullRequestStatusManuallyMerged); err != nil {
			return err
		} else if !merged {
			return fmt.Errorf("SetMerged failed")
		}
		return nil
	})
	releaser()
	if err != nil {
		return err
	}

	notify_service.MergePullRequest(baseGitRepo.Ctx, doer, pr)
	log.Info("manuallyMerged[%d]: Marked as manually merged into %s/%s by commit id: %s", pr.ID, pr.BaseRepo.Name, pr.BaseBranch, commitID)

	return handleCloseCrossReferences(ctx, pr, doer)
}

// SetMerged sets a pull request to merged and closes the corresponding issue
func SetMerged(ctx context.Context, pr *issues_model.PullRequest, mergedCommitID string, mergedTimeStamp timeutil.TimeStamp, merger *user_model.User, mergeStatus issues_model.PullRequestStatus) (bool, error) {
	if pr.HasMerged {
		return false, fmt.Errorf("PullRequest[%d] already merged", pr.Index)
	}

	pr.HasMerged = true
	pr.MergedCommitID = mergedCommitID
	pr.MergedUnix = mergedTimeStamp
	pr.Merger = merger
	pr.MergerID = merger.ID
	pr.Status = mergeStatus
	// reset the conflicted files as there cannot be any if we're merged
	pr.ConflictedFiles = []string{}

	if pr.MergedCommitID == "" || pr.MergedUnix == 0 || pr.Merger == nil {
		return false, fmt.Errorf("unable to merge PullRequest[%d], some required fields are empty", pr.Index)
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return false, err
	}
	defer committer.Close()

	pr.Issue = nil
	if err := pr.LoadIssue(ctx); err != nil {
		return false, err
	}

	if err := pr.Issue.LoadRepo(ctx); err != nil {
		return false, err
	}

	if err := pr.Issue.Repo.LoadOwner(ctx); err != nil {
		return false, err
	}

	// Removing an auto merge pull and ignore if not exist
	if err := pull_model.DeleteScheduledAutoMerge(ctx, pr.ID); err != nil && !db.IsErrNotExist(err) {
		return false, fmt.Errorf("DeleteScheduledAutoMerge[%d]: %v", pr.ID, err)
	}

	// Set issue as closed
	if _, err := issues_model.SetIssueAsClosed(ctx, pr.Issue, pr.Merger, true); err != nil {
		return false, fmt.Errorf("ChangeIssueStatus: %w", err)
	}

	// We need to save all of the data used to compute this merge as it may have already been changed by TestPatch. FIXME: need to set some state to prevent TestPatch from running whilst we are merging.
	if cnt, err := db.GetEngine(ctx).Where("id = ?", pr.ID).
		And("has_merged = ?", false).
		Cols("has_merged, status, merge_base, merged_commit_id, merger_id, merged_unix, conflicted_files").
		Update(pr); err != nil {
		return false, fmt.Errorf("failed to update pr[%d]: %w", pr.ID, err)
	} else if cnt != 1 {
		return false, issues_model.ErrIssueAlreadyChanged
	}

	if err := committer.Commit(); err != nil {
		return false, err
	}

	return true, nil
}
