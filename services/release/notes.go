// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package release

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

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
	TagTarget   string
	PreviousTag string
}

// GenerateReleaseNotes builds the markdown snippet for release notes.
func GenerateReleaseNotes(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, opts GenerateReleaseNotesOptions) (string, error) {
	headCommit, err := resolveHeadCommit(gitRepo, opts.TagName, opts.TagTarget)
	if err != nil {
		return "", err
	}

	if opts.PreviousTag == "" {
		// no previous tag, usually due to there is no tag in the repo, use the same content as GitHub
		content := fmt.Sprintf("**Full Changelog**: %s/commits/tag/%s\n", repo.HTMLURL(ctx), util.PathEscapeSegments(opts.TagName))
		return content, nil
	}

	baseCommit, err := gitRepo.GetCommit(opts.PreviousTag)
	if err != nil {
		return "", util.ErrorWrapTranslatable(util.ErrNotExist, "repo.release.generate_notes_tag_not_found", opts.TagName)
	}

	commits, err := gitRepo.CommitsBetweenIDs(headCommit.ID.String(), baseCommit.ID.String())
	if err != nil {
		return "", fmt.Errorf("CommitsBetweenIDs: %w", err)
	}

	prs, err := collectPullRequestsFromCommits(ctx, repo.ID, commits)
	if err != nil {
		return "", err
	}

	contributors, newContributors, err := collectContributors(ctx, repo.ID, prs)
	if err != nil {
		return "", err
	}

	content := buildReleaseNotesContent(ctx, repo, opts.TagName, opts.PreviousTag, prs, contributors, newContributors)
	return content, nil
}

func resolveHeadCommit(gitRepo *git.Repository, tagName, tagTarget string) (*git.Commit, error) {
	ref := tagName
	if !gitRepo.IsTagExist(tagName) {
		ref = tagTarget
	}

	commit, err := gitRepo.GetCommit(ref)
	if err != nil {
		return nil, util.ErrorWrapTranslatable(util.ErrNotExist, "repo.release.generate_notes_target_not_found", ref)
	}
	return commit, nil
}

func collectPullRequestsFromCommits(ctx context.Context, repoID int64, commits []*git.Commit) ([]*issues_model.PullRequest, error) {
	prs := make([]*issues_model.PullRequest, 0, len(commits))

	for _, commit := range commits {
		pr, err := issues_model.GetPullRequestByMergedCommit(ctx, repoID, commit.ID.String())
		if err != nil {
			if issues_model.IsErrPullRequestNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("GetPullRequestByMergedCommit: %w", err)
		}

		if err = pr.LoadIssue(ctx); err != nil {
			return nil, fmt.Errorf("LoadIssue: %w", err)
		}
		if err = pr.Issue.LoadAttributes(ctx); err != nil {
			return nil, fmt.Errorf("LoadIssueAttributes: %w", err)
		}

		prs = append(prs, pr)
	}

	slices.SortFunc(prs, func(a, b *issues_model.PullRequest) int {
		if cmpRes := cmp.Compare(b.MergedUnix, a.MergedUnix); cmpRes != 0 {
			return cmpRes
		}
		return cmp.Compare(b.Issue.Index, a.Issue.Index)
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
	builder.WriteByte('\n')
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
	hasMergedBefore, err := issues_model.HasMergedPullRequestInRepoBefore(ctx, repoID, posterID, pr.MergedUnix, pr.ID)
	if err != nil {
		return false, fmt.Errorf("check merged PRs for contributor: %w", err)
	}
	return !hasMergedBefore, nil
}
