// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package release

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/util"
)

// GenerateReleaseNotesOptions describes how to build release notes content.
type GenerateReleaseNotesOptions struct {
	TagName     string
	Target      string
	PreviousTag string
}

// GenerateReleaseNotesResult holds the rendered notes and the base tag used.
type GenerateReleaseNotesResult struct {
	Content     string
	PreviousTag string
}

// ErrReleaseNotesTagNotFound indicates a requested tag does not exist in git.
type ErrReleaseNotesTagNotFound struct {
	TagName string
}

func (err ErrReleaseNotesTagNotFound) Error() string {
	return fmt.Sprintf("tag %q not found", err.TagName)
}

func (err ErrReleaseNotesTagNotFound) Unwrap() error {
	return util.ErrNotExist
}

// ErrReleaseNotesNoBaseTag indicates there is no tag to diff against.
type ErrReleaseNotesNoBaseTag struct{}

func (err ErrReleaseNotesNoBaseTag) Error() string {
	return "no previous tag found for release notes"
}

func (err ErrReleaseNotesNoBaseTag) Unwrap() error {
	return util.ErrNotExist
}

// IsErrReleaseNotesNoBaseTag reports whether the error is ErrReleaseNotesNoBaseTag.
func IsErrReleaseNotesNoBaseTag(err error) bool {
	_, ok := err.(ErrReleaseNotesNoBaseTag)
	return ok
}

// ErrReleaseNotesTargetNotFound indicates the release target ref cannot be resolved.
type ErrReleaseNotesTargetNotFound struct {
	Ref string
}

func (err ErrReleaseNotesTargetNotFound) Error() string {
	return fmt.Sprintf("release target %q not found", err.Ref)
}

func (err ErrReleaseNotesTargetNotFound) Unwrap() error {
	return util.ErrNotExist
}

// GenerateReleaseNotes builds the markdown snippet for release notes.
func GenerateReleaseNotes(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, opts GenerateReleaseNotesOptions) (*GenerateReleaseNotesResult, error) {
	tagName := strings.TrimSpace(opts.TagName)
	if tagName == "" {
		return nil, util.NewInvalidArgumentErrorf("empty target tag name for release notes")
	}

	headCommit, err := resolveHeadCommit(repo, gitRepo, tagName, opts.Target)
	if err != nil {
		return nil, err
	}

	baseSelection, err := resolveBaseTag(ctx, repo, gitRepo, headCommit, tagName, opts.PreviousTag)
	if err != nil {
		return nil, err
	}

	commits, err := gitRepo.CommitsBetweenIDs(headCommit.ID.String(), baseSelection.Commit.ID.String())
	if err != nil {
		return nil, fmt.Errorf("CommitsBetweenIDs: %w", err)
	}

	prs, err := collectPullRequestsFromCommits(ctx, repo.ID, commits)
	if err != nil {
		return nil, err
	}

	contributors, newContributors, err := collectContributors(ctx, repo.ID, prs)
	if err != nil {
		return nil, err
	}

	content := buildReleaseNotesContent(ctx, repo, tagName, baseSelection.CompareBase, prs, contributors, newContributors)
	return &GenerateReleaseNotesResult{
		Content:     content,
		PreviousTag: baseSelection.PreviousTag,
	}, nil
}

func resolveHeadCommit(repo *repo_model.Repository, gitRepo *git.Repository, tagName, target string) (*git.Commit, error) {
	ref := tagName
	if !gitRepo.IsTagExist(tagName) {
		ref = strings.TrimSpace(target)
		if ref == "" {
			ref = repo.DefaultBranch
		}
	}

	commit, err := gitRepo.GetCommit(ref)
	if err != nil {
		return nil, ErrReleaseNotesTargetNotFound{Ref: ref}
	}
	return commit, nil
}

type baseSelection struct {
	CompareBase string
	PreviousTag string
	Commit      *git.Commit
}

func resolveBaseTag(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, headCommit *git.Commit, tagName, requestedBase string) (*baseSelection, error) {
	requestedBase = strings.TrimSpace(requestedBase)
	if requestedBase != "" {
		if gitRepo.IsTagExist(requestedBase) {
			baseCommit, err := gitRepo.GetCommit(requestedBase)
			if err != nil {
				return nil, ErrReleaseNotesTagNotFound{TagName: requestedBase}
			}
			return &baseSelection{
				CompareBase: requestedBase,
				PreviousTag: requestedBase,
				Commit:      baseCommit,
			}, nil
		}
		return nil, ErrReleaseNotesTagNotFound{TagName: requestedBase}
	}

	rel, err := repo_model.GetLatestReleaseByRepoID(ctx, repo.ID)
	switch {
	case err == nil:
		candidate := strings.TrimSpace(rel.TagName)
		if !strings.EqualFold(candidate, tagName) {
			if gitRepo.IsTagExist(candidate) {
				baseCommit, err := gitRepo.GetCommit(candidate)
				if err != nil {
					return nil, ErrReleaseNotesTagNotFound{TagName: candidate}
				}
				return &baseSelection{
					CompareBase: candidate,
					PreviousTag: candidate,
					Commit:      baseCommit,
				}, nil
			}
			return nil, ErrReleaseNotesTagNotFound{TagName: candidate}
		}
	case repo_model.IsErrReleaseNotExist(err):
		// fall back to tags below
	default:
		return nil, fmt.Errorf("GetLatestReleaseByRepoID: %w", err)
	}

	tagInfos, _, err := gitRepo.GetTagInfos(0, 0)
	if err != nil {
		return nil, fmt.Errorf("GetTagInfos: %w", err)
	}

	for _, tag := range tagInfos {
		if strings.EqualFold(tag.Name, tagName) {
			continue
		}
		baseCommit, err := gitRepo.GetCommit(tag.Name)
		if err != nil {
			return nil, ErrReleaseNotesTagNotFound{TagName: tag.Name}
		}
		return &baseSelection{
			CompareBase: tag.Name,
			PreviousTag: tag.Name,
			Commit:      baseCommit,
		}, nil
	}

	initialCommit, err := findInitialCommit(headCommit)
	if err != nil {
		return nil, err
	}
	return &baseSelection{
		CompareBase: initialCommit.ID.String(),
		PreviousTag: "",
		Commit:      initialCommit,
	}, nil
}

func findInitialCommit(commit *git.Commit) (*git.Commit, error) {
	current := commit
	for current.ParentCount() > 0 {
		parent, err := current.Parent(0)
		if err != nil {
			return nil, fmt.Errorf("Parent: %w", err)
		}
		current = parent
	}
	return current, nil
}

func collectPullRequestsFromCommits(ctx context.Context, repoID int64, commits []*git.Commit) ([]*issues_model.PullRequest, error) {
	seen := container.Set[int64]{}
	prs := make([]*issues_model.PullRequest, 0, len(commits))

	for _, commit := range commits {
		pr, err := issues_model.GetPullRequestByMergedCommit(ctx, repoID, commit.ID.String())
		if err != nil {
			if issues_model.IsErrPullRequestNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("GetPullRequestByMergedCommit: %w", err)
		}

		if !pr.HasMerged || seen.Contains(pr.ID) {
			continue
		}

		if err = pr.LoadIssue(ctx); err != nil {
			return nil, fmt.Errorf("LoadIssue: %w", err)
		}
		if err = pr.Issue.LoadAttributes(ctx); err != nil {
			return nil, fmt.Errorf("LoadIssueAttributes: %w", err)
		}

		seen.Add(pr.ID)
		prs = append(prs, pr)
	}

	sort.Slice(prs, func(i, j int) bool {
		if prs[i].MergedUnix != prs[j].MergedUnix {
			return prs[i].MergedUnix > prs[j].MergedUnix
		}
		return prs[i].Issue.Index > prs[j].Issue.Index
	})

	return prs, nil
}

func buildReleaseNotesContent(ctx context.Context, repo *repo_model.Repository, tagName, baseRef string, prs []*issues_model.PullRequest, contributors []*user_model.User, newContributors []*issues_model.PullRequest) string {
	var builder strings.Builder
	builder.WriteString("## What's Changed\n")

	for _, pr := range prs {
		prURL := pr.Issue.HTMLURL(ctx)
		builder.WriteString(fmt.Sprintf("* %s in [#%d](%s)\n", pr.Issue.Title, pr.Issue.Index, prURL))
	}

	builder.WriteString("\n")

	if len(contributors) > 0 {
		builder.WriteString("## Contributors\n")
		for _, contributor := range contributors {
			builder.WriteString(fmt.Sprintf("* @%s\n", contributor.Name))
		}
		builder.WriteString("\n")
	}

	if len(newContributors) > 0 {
		builder.WriteString("## New Contributors\n")
		for _, contributor := range newContributors {
			prURL := contributor.Issue.HTMLURL(ctx)
			builder.WriteString(fmt.Sprintf("* @%s made their first contribution in [#%d](%s)\n", contributor.Issue.Poster.Name, contributor.Issue.Index, prURL))
		}
		builder.WriteString("\n")
	}

	builder.WriteString("**Full Changelog**: ")
	compareURL := fmt.Sprintf("%s/compare/%s...%s", repo.HTMLURL(ctx), util.PathEscapeSegments(baseRef), util.PathEscapeSegments(tagName))
	builder.WriteString(fmt.Sprintf("[%s...%s](%s)", baseRef, tagName, compareURL))
	return builder.String()
}

func collectContributors(ctx context.Context, repoID int64, prs []*issues_model.PullRequest) ([]*user_model.User, []*issues_model.PullRequest, error) {
	contributors := make([]*user_model.User, 0, len(prs))
	newContributors := make([]*issues_model.PullRequest, 0, len(prs))
	seenContributors := container.Set[int64]{}
	seenNew := container.Set[int64]{}

	for _, pr := range prs {
		poster := pr.Issue.Poster
		posterID := poster.ID

		if posterID == 0 {
			// Migrated PRs may not have a linked local user (PosterID == 0). Skip them for now.
			continue
		}

		if !seenContributors.Contains(posterID) {
			contributors = append(contributors, poster)
			seenContributors.Add(posterID)
		}

		if seenNew.Contains(posterID) {
			continue
		}

		isFirst, err := isFirstContribution(ctx, repoID, posterID, pr)
		if err != nil {
			return nil, nil, err
		}
		if isFirst {
			seenNew.Add(posterID)
			newContributors = append(newContributors, pr)
		}
	}

	return contributors, newContributors, nil
}

func isFirstContribution(ctx context.Context, repoID, posterID int64, pr *issues_model.PullRequest) (bool, error) {
	count, err := db.GetEngine(ctx).
		Table("issue").
		Join("INNER", "pull_request", "pull_request.issue_id = issue.id").
		Where("issue.repo_id = ?", repoID).
		And("pull_request.has_merged = ?", true).
		And("issue.poster_id = ?", posterID).
		And("pull_request.id != ?", pr.ID).
		And("pull_request.merged_unix < ?", pr.MergedUnix).
		Count()
	if err != nil {
		return false, fmt.Errorf("count merged PRs for contributor: %w", err)
	}
	return count == 0, nil
}
