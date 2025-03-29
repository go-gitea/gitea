// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"
	"fmt"

	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/gitdiff"
)

// ToAPIPullRequest assumes following fields have been assigned with valid values:
// Required - Issue
// Optional - Merger
func ToAPIPullRequest(ctx context.Context, pr *issues_model.PullRequest, doer *user_model.User) *api.PullRequest {
	var (
		baseBranch *git.Branch
		headBranch *git.Branch
		baseCommit *git.Commit
		err        error
	)

	if err = pr.LoadIssue(ctx); err != nil {
		log.Error("pr.LoadIssue[%d]: %v", pr.ID, err)
		return nil
	}

	if err = pr.Issue.LoadRepo(ctx); err != nil {
		log.Error("pr.Issue.LoadRepo[%d]: %v", pr.ID, err)
		return nil
	}

	apiIssue := ToAPIIssue(ctx, doer, pr.Issue)
	if err := pr.LoadBaseRepo(ctx); err != nil {
		log.Error("GetRepositoryById[%d]: %v", pr.ID, err)
		return nil
	}

	if err := pr.LoadHeadRepo(ctx); err != nil {
		log.Error("GetRepositoryById[%d]: %v", pr.ID, err)
		return nil
	}

	var doerID int64
	if doer != nil {
		doerID = doer.ID
	}

	const repoDoerPermCacheKey = "repo_doer_perm_cache"
	p, err := cache.GetWithContextCache(ctx, repoDoerPermCacheKey, fmt.Sprintf("%d_%d", pr.BaseRepoID, doerID),
		func() (access_model.Permission, error) {
			return access_model.GetUserRepoPermission(ctx, pr.BaseRepo, doer)
		})
	if err != nil {
		log.Error("GetUserRepoPermission[%d]: %v", pr.BaseRepoID, err)
		p.AccessMode = perm.AccessModeNone
	}

	apiPullRequest := &api.PullRequest{
		ID:             pr.ID,
		URL:            pr.Issue.HTMLURL(),
		Index:          pr.Index,
		Poster:         apiIssue.Poster,
		Title:          apiIssue.Title,
		Body:           apiIssue.Body,
		Labels:         apiIssue.Labels,
		Milestone:      apiIssue.Milestone,
		Assignee:       apiIssue.Assignee,
		Assignees:      util.SliceNilAsEmpty(apiIssue.Assignees),
		State:          apiIssue.State,
		Draft:          pr.IsWorkInProgress(ctx),
		IsLocked:       apiIssue.IsLocked,
		Comments:       apiIssue.Comments,
		ReviewComments: pr.GetReviewCommentsCount(ctx),
		HTMLURL:        pr.Issue.HTMLURL(),
		DiffURL:        pr.Issue.DiffURL(),
		PatchURL:       pr.Issue.PatchURL(),
		HasMerged:      pr.HasMerged,
		MergeBase:      pr.MergeBase,
		Mergeable:      pr.Mergeable(ctx),
		Deadline:       apiIssue.Deadline,
		Created:        pr.Issue.CreatedUnix.AsTimePtr(),
		Updated:        pr.Issue.UpdatedUnix.AsTimePtr(),
		PinOrder:       util.Iif(apiIssue.PinOrder == -1, 0, apiIssue.PinOrder),

		// output "[]" rather than null to align to github outputs
		RequestedReviewers:      []*api.User{},
		RequestedReviewersTeams: []*api.Team{},

		AllowMaintainerEdit: pr.AllowMaintainerEdit,

		Base: &api.PRBranchInfo{
			Name:       pr.BaseBranch,
			Ref:        pr.BaseBranch,
			RepoID:     pr.BaseRepoID,
			Repository: ToRepo(ctx, pr.BaseRepo, p),
		},
		Head: &api.PRBranchInfo{
			Name:   pr.HeadBranch,
			Ref:    fmt.Sprintf("%s%d/head", git.PullPrefix, pr.Index),
			RepoID: -1,
		},
	}

	if err = pr.LoadRequestedReviewers(ctx); err != nil {
		log.Error("LoadRequestedReviewers[%d]: %v", pr.ID, err)
		return nil
	}
	if err = pr.LoadRequestedReviewersTeams(ctx); err != nil {
		log.Error("LoadRequestedReviewersTeams[%d]: %v", pr.ID, err)
		return nil
	}

	for _, reviewer := range pr.RequestedReviewers {
		apiPullRequest.RequestedReviewers = append(apiPullRequest.RequestedReviewers, ToUser(ctx, reviewer, nil))
	}

	for _, reviewerTeam := range pr.RequestedReviewersTeams {
		convertedTeam, err := ToTeam(ctx, reviewerTeam, true)
		if err != nil {
			log.Error("LoadRequestedReviewersTeams[%d]: %v", pr.ID, err)
			return nil
		}

		apiPullRequest.RequestedReviewersTeams = append(apiPullRequest.RequestedReviewersTeams, convertedTeam)
	}

	if pr.Issue.ClosedUnix != 0 {
		apiPullRequest.Closed = pr.Issue.ClosedUnix.AsTimePtr()
	}

	gitRepo, err := gitrepo.OpenRepository(ctx, pr.BaseRepo)
	if err != nil {
		log.Error("OpenRepository[%s]: %v", pr.BaseRepo.RepoPath(), err)
		return nil
	}
	defer gitRepo.Close()

	baseBranch, err = gitRepo.GetBranch(pr.BaseBranch)
	if err != nil && !git.IsErrBranchNotExist(err) {
		log.Error("GetBranch[%s]: %v", pr.BaseBranch, err)
		return nil
	}

	if err == nil {
		baseCommit, err = baseBranch.GetCommit()
		if err != nil && !git.IsErrNotExist(err) {
			log.Error("GetCommit[%s]: %v", baseBranch.Name, err)
			return nil
		}

		if err == nil {
			apiPullRequest.Base.Sha = baseCommit.ID.String()
		}
	}

	if pr.Flow == issues_model.PullRequestFlowAGit {
		gitRepo, err := gitrepo.OpenRepository(ctx, pr.BaseRepo)
		if err != nil {
			log.Error("OpenRepository[%s]: %v", pr.GetGitRefName(), err)
			return nil
		}
		defer gitRepo.Close()

		apiPullRequest.Head.Sha, err = gitRepo.GetRefCommitID(pr.GetGitRefName())
		if err != nil {
			log.Error("GetRefCommitID[%s]: %v", pr.GetGitRefName(), err)
			return nil
		}
		apiPullRequest.Head.RepoID = pr.BaseRepoID
		apiPullRequest.Head.Repository = apiPullRequest.Base.Repository
		apiPullRequest.Head.Name = ""
	}

	if pr.HeadRepo != nil && pr.Flow == issues_model.PullRequestFlowGithub {
		p, err := access_model.GetUserRepoPermission(ctx, pr.HeadRepo, doer)
		if err != nil {
			log.Error("GetUserRepoPermission[%d]: %v", pr.HeadRepoID, err)
			p.AccessMode = perm.AccessModeNone
		}

		apiPullRequest.Head.RepoID = pr.HeadRepo.ID
		apiPullRequest.Head.Repository = ToRepo(ctx, pr.HeadRepo, p)

		headGitRepo, err := gitrepo.OpenRepository(ctx, pr.HeadRepo)
		if err != nil {
			log.Error("OpenRepository[%s]: %v", pr.HeadRepo.RepoPath(), err)
			return nil
		}
		defer headGitRepo.Close()

		headBranch, err = headGitRepo.GetBranch(pr.HeadBranch)
		if err != nil && !git.IsErrBranchNotExist(err) {
			log.Error("GetBranch[%s]: %v", pr.HeadBranch, err)
			return nil
		}

		// Outer scope variables to be used in diff calculation
		var (
			startCommitID string
			endCommitID   string
		)

		if git.IsErrBranchNotExist(err) {
			headCommitID, err := headGitRepo.GetRefCommitID(apiPullRequest.Head.Ref)
			if err != nil && !git.IsErrNotExist(err) {
				log.Error("GetCommit[%s]: %v", pr.HeadBranch, err)
				return nil
			}
			if err == nil {
				apiPullRequest.Head.Sha = headCommitID
				endCommitID = headCommitID
			}
		} else {
			commit, err := headBranch.GetCommit()
			if err != nil && !git.IsErrNotExist(err) {
				log.Error("GetCommit[%s]: %v", headBranch.Name, err)
				return nil
			}
			if err == nil {
				apiPullRequest.Head.Ref = pr.HeadBranch
				apiPullRequest.Head.Sha = commit.ID.String()
				endCommitID = commit.ID.String()
			}
		}

		// Calculate diff
		startCommitID = pr.MergeBase

		diffShortStats, err := gitdiff.GetDiffShortStat(gitRepo, startCommitID, endCommitID)
		if err != nil {
			log.Error("GetDiffShortStat: %v", err)
		} else {
			apiPullRequest.ChangedFiles = &diffShortStats.NumFiles
			apiPullRequest.Additions = &diffShortStats.TotalAddition
			apiPullRequest.Deletions = &diffShortStats.TotalDeletion
		}
	}

	if len(apiPullRequest.Head.Sha) == 0 && len(apiPullRequest.Head.Ref) != 0 {
		baseGitRepo, err := gitrepo.OpenRepository(ctx, pr.BaseRepo)
		if err != nil {
			log.Error("OpenRepository[%s]: %v", pr.BaseRepo.RepoPath(), err)
			return nil
		}
		defer baseGitRepo.Close()
		refs, err := baseGitRepo.GetRefsFiltered(apiPullRequest.Head.Ref)
		if err != nil {
			log.Error("GetRefsFiltered[%s]: %v", apiPullRequest.Head.Ref, err)
			return nil
		} else if len(refs) == 0 {
			log.Error("unable to resolve PR head ref")
		} else {
			apiPullRequest.Head.Sha = refs[0].Object.String()
		}
	}

	if pr.HasMerged {
		apiPullRequest.Merged = pr.MergedUnix.AsTimePtr()
		apiPullRequest.MergedCommitID = &pr.MergedCommitID
		apiPullRequest.MergedBy = ToUser(ctx, pr.Merger, nil)
	}

	return apiPullRequest
}

func ToAPIPullRequests(ctx context.Context, baseRepo *repo_model.Repository, prs issues_model.PullRequestList, doer *user_model.User) ([]*api.PullRequest, error) {
	for _, pr := range prs {
		pr.BaseRepo = baseRepo
		if pr.BaseRepoID == pr.HeadRepoID {
			pr.HeadRepo = baseRepo
		}
	}

	// NOTE: load head repositories
	if err := prs.LoadRepositories(ctx); err != nil {
		return nil, err
	}
	issueList, err := prs.LoadIssues(ctx)
	if err != nil {
		return nil, err
	}

	if err := issueList.LoadLabels(ctx); err != nil {
		return nil, err
	}
	if err := issueList.LoadPosters(ctx); err != nil {
		return nil, err
	}
	if err := issueList.LoadAttachments(ctx); err != nil {
		return nil, err
	}
	if err := issueList.LoadMilestones(ctx); err != nil {
		return nil, err
	}
	if err := issueList.LoadAssignees(ctx); err != nil {
		return nil, err
	}
	if err = issueList.LoadPinOrder(ctx); err != nil {
		return nil, err
	}

	reviews, err := prs.LoadReviews(ctx)
	if err != nil {
		return nil, err
	}
	if err = reviews.LoadReviewers(ctx); err != nil {
		return nil, err
	}

	reviewersMap := make(map[int64][]*user_model.User)
	for _, review := range reviews {
		if review.Reviewer != nil {
			reviewersMap[review.IssueID] = append(reviewersMap[review.IssueID], review.Reviewer)
		}
	}

	reviewCounts, err := prs.LoadReviewCommentsCounts(ctx)
	if err != nil {
		return nil, err
	}

	gitRepo, err := gitrepo.OpenRepository(ctx, baseRepo)
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	baseRepoPerm, err := access_model.GetUserRepoPermission(ctx, baseRepo, doer)
	if err != nil {
		log.Error("GetUserRepoPermission[%d]: %v", baseRepo.ID, err)
		baseRepoPerm.AccessMode = perm.AccessModeNone
	}

	apiRepo := ToRepo(ctx, baseRepo, baseRepoPerm)
	baseBranchCache := make(map[string]*git_model.Branch)
	apiPullRequests := make([]*api.PullRequest, 0, len(prs))
	for _, pr := range prs {
		apiIssue := ToAPIIssue(ctx, doer, pr.Issue)

		apiPullRequest := &api.PullRequest{
			ID:             pr.ID,
			URL:            pr.Issue.HTMLURL(),
			Index:          pr.Index,
			Poster:         apiIssue.Poster,
			Title:          apiIssue.Title,
			Body:           apiIssue.Body,
			Labels:         apiIssue.Labels,
			Milestone:      apiIssue.Milestone,
			Assignee:       apiIssue.Assignee,
			Assignees:      apiIssue.Assignees,
			State:          apiIssue.State,
			Draft:          pr.IsWorkInProgress(ctx),
			IsLocked:       apiIssue.IsLocked,
			Comments:       apiIssue.Comments,
			ReviewComments: reviewCounts[pr.IssueID],
			HTMLURL:        pr.Issue.HTMLURL(),
			DiffURL:        pr.Issue.DiffURL(),
			PatchURL:       pr.Issue.PatchURL(),
			HasMerged:      pr.HasMerged,
			MergeBase:      pr.MergeBase,
			Mergeable:      pr.Mergeable(ctx),
			Deadline:       apiIssue.Deadline,
			Created:        pr.Issue.CreatedUnix.AsTimePtr(),
			Updated:        pr.Issue.UpdatedUnix.AsTimePtr(),
			PinOrder:       util.Iif(apiIssue.PinOrder == -1, 0, apiIssue.PinOrder),

			AllowMaintainerEdit: pr.AllowMaintainerEdit,

			Base: &api.PRBranchInfo{
				Name:       pr.BaseBranch,
				Ref:        pr.BaseBranch,
				RepoID:     pr.BaseRepoID,
				Repository: apiRepo,
			},
			Head: &api.PRBranchInfo{
				Name:   pr.HeadBranch,
				Ref:    fmt.Sprintf("%s%d/head", git.PullPrefix, pr.Index),
				RepoID: -1,
			},
		}

		pr.RequestedReviewers = reviewersMap[pr.IssueID]
		for _, reviewer := range pr.RequestedReviewers {
			apiPullRequest.RequestedReviewers = append(apiPullRequest.RequestedReviewers, ToUser(ctx, reviewer, nil))
		}

		for _, reviewerTeam := range pr.RequestedReviewersTeams {
			convertedTeam, err := ToTeam(ctx, reviewerTeam, true)
			if err != nil {
				log.Error("LoadRequestedReviewersTeams[%d]: %v", pr.ID, err)
				return nil, err
			}

			apiPullRequest.RequestedReviewersTeams = append(apiPullRequest.RequestedReviewersTeams, convertedTeam)
		}

		if pr.Issue.ClosedUnix != 0 {
			apiPullRequest.Closed = pr.Issue.ClosedUnix.AsTimePtr()
		}

		baseBranch, ok := baseBranchCache[pr.BaseBranch]
		if !ok {
			baseBranch, err = git_model.GetBranch(ctx, baseRepo.ID, pr.BaseBranch)
			if err == nil {
				baseBranchCache[pr.BaseBranch] = baseBranch
			} else if !git_model.IsErrBranchNotExist(err) {
				return nil, err
			}
		}

		if baseBranch != nil {
			apiPullRequest.Base.Sha = baseBranch.CommitID
		}

		if pr.Flow == issues_model.PullRequestFlowAGit {
			apiPullRequest.Head.Sha, err = gitRepo.GetRefCommitID(pr.GetGitRefName())
			if err != nil {
				log.Error("GetRefCommitID[%s]: %v", pr.GetGitRefName(), err)
				return nil, err
			}
			apiPullRequest.Head.RepoID = pr.BaseRepoID
			apiPullRequest.Head.Repository = apiPullRequest.Base.Repository
			apiPullRequest.Head.Name = ""
		}

		var headGitRepo *git.Repository
		if pr.HeadRepo != nil && pr.Flow == issues_model.PullRequestFlowGithub {
			if pr.HeadRepoID == pr.BaseRepoID {
				apiPullRequest.Head.RepoID = pr.HeadRepo.ID
				apiPullRequest.Head.Repository = apiRepo
				headGitRepo = gitRepo
			} else {
				p, err := access_model.GetUserRepoPermission(ctx, pr.HeadRepo, doer)
				if err != nil {
					log.Error("GetUserRepoPermission[%d]: %v", pr.HeadRepoID, err)
					p.AccessMode = perm.AccessModeNone
				}

				apiPullRequest.Head.RepoID = pr.HeadRepo.ID
				apiPullRequest.Head.Repository = ToRepo(ctx, pr.HeadRepo, p)

				headGitRepo, err = gitrepo.OpenRepository(ctx, pr.HeadRepo)
				if err != nil {
					log.Error("OpenRepository[%s]: %v", pr.HeadRepo.RepoPath(), err)
					return nil, err
				}
				defer headGitRepo.Close()
			}

			headBranch, err := headGitRepo.GetBranch(pr.HeadBranch)
			if err != nil && !git.IsErrBranchNotExist(err) {
				log.Error("GetBranch[%s]: %v", pr.HeadBranch, err)
				return nil, err
			}

			if git.IsErrBranchNotExist(err) {
				headCommitID, err := headGitRepo.GetRefCommitID(apiPullRequest.Head.Ref)
				if err != nil && !git.IsErrNotExist(err) {
					log.Error("GetCommit[%s]: %v", pr.HeadBranch, err)
					return nil, err
				}
				if err == nil {
					apiPullRequest.Head.Sha = headCommitID
				}
			} else {
				commit, err := headBranch.GetCommit()
				if err != nil && !git.IsErrNotExist(err) {
					log.Error("GetCommit[%s]: %v", headBranch.Name, err)
					return nil, err
				}
				if err == nil {
					apiPullRequest.Head.Ref = pr.HeadBranch
					apiPullRequest.Head.Sha = commit.ID.String()
				}
			}
		}

		if len(apiPullRequest.Head.Sha) == 0 && len(apiPullRequest.Head.Ref) != 0 {
			refs, err := gitRepo.GetRefsFiltered(apiPullRequest.Head.Ref)
			if err != nil {
				log.Error("GetRefsFiltered[%s]: %v", apiPullRequest.Head.Ref, err)
				return nil, err
			} else if len(refs) == 0 {
				log.Error("unable to resolve PR head ref")
			} else {
				apiPullRequest.Head.Sha = refs[0].Object.String()
			}
		}

		if pr.HasMerged {
			apiPullRequest.Merged = pr.MergedUnix.AsTimePtr()
			apiPullRequest.MergedCommitID = &pr.MergedCommitID
			apiPullRequest.MergedBy = ToUser(ctx, pr.Merger, nil)
		}

		// Do not provide "ChangeFiles/Additions/Deletions" for the PR list, because the "diff" is quite slow
		// If callers are interested in these values, they should do a separate request to get the PR details
		if apiPullRequest.ChangedFiles != nil || apiPullRequest.Additions != nil || apiPullRequest.Deletions != nil {
			setting.PanicInDevOrTesting("ChangedFiles/Additions/Deletions should not be set in PR list")
		}

		apiPullRequests = append(apiPullRequests, apiPullRequest)
	}

	return apiPullRequests, nil
}
