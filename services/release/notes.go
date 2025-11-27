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

	version "github.com/hashicorp/go-version"
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

func newErrReleaseNotesTagNotFound(tagName string) error {
	return util.ErrorWrapTranslatable(ErrReleaseNotesTagNotFound{TagName: tagName}, "repo.release.generate_notes_tag_not_found", tagName)
}

// ErrReleaseNotesNoBaseTag indicates there is no tag to diff against.
type ErrReleaseNotesNoBaseTag struct{}

func (err ErrReleaseNotesNoBaseTag) Error() string {
	return "no previous tag found for release notes"
}

func (err ErrReleaseNotesNoBaseTag) Unwrap() error {
	return util.ErrNotExist
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

func newErrReleaseNotesTargetNotFound(ref string) error {
	return util.ErrorWrapTranslatable(ErrReleaseNotesTargetNotFound{Ref: ref}, "repo.release.generate_notes_target_not_found", ref)
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
		return nil, newErrReleaseNotesTargetNotFound(ref)
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
		return buildBaseSelectionForTag(gitRepo, requestedBase)
	}

	candidate, err := autoPreviousReleaseTag(ctx, repo, tagName)
	if err != nil {
		return nil, err
	}
	if candidate != "" {
		return buildBaseSelectionForTag(gitRepo, candidate)
	}

	tagInfos, _, err := gitRepo.GetTagInfos(0, 0)
	if err != nil {
		return nil, fmt.Errorf("GetTagInfos: %w", err)
	}

	if previousTag, ok := findPreviousTagName(tagInfos, tagName); ok {
		return buildBaseSelectionForTag(gitRepo, previousTag)
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

func buildBaseSelectionForTag(gitRepo *git.Repository, tagName string) (*baseSelection, error) {
	baseCommit, err := gitRepo.GetCommit(tagName)
	if err != nil {
		return nil, newErrReleaseNotesTagNotFound(tagName)
	}
	return &baseSelection{
		CompareBase: tagName,
		PreviousTag: tagName,
		Commit:      baseCommit,
	}, nil
}

func autoPreviousReleaseTag(ctx context.Context, repo *repo_model.Repository, tagName string) (string, error) {
	currentRelease, err := repo_model.GetRelease(ctx, repo.ID, tagName)
	switch {
	case err == nil:
		return findPreviousPublishedReleaseTag(ctx, repo, currentRelease)
	case repo_model.IsErrReleaseNotExist(err):
		// this tag has no stored release, fall back to latest release below
	default:
		return "", fmt.Errorf("GetRelease: %w", err)
	}

	rel, err := repo_model.GetLatestReleaseByRepoID(ctx, repo.ID)
	switch {
	case err == nil:
		if strings.EqualFold(rel.TagName, tagName) {
			return "", nil
		}
		return rel.TagName, nil
	case repo_model.IsErrReleaseNotExist(err):
		return "", nil
	default:
		return "", fmt.Errorf("GetLatestReleaseByRepoID: %w", err)
	}
}

func findPreviousPublishedReleaseTag(ctx context.Context, repo *repo_model.Repository, current *repo_model.Release) (string, error) {
	prev, err := repo_model.GetPreviousPublishedRelease(ctx, repo.ID, current)
	switch {
	case err == nil:
	case repo_model.IsErrReleaseNotExist(err):
		return "", nil
	default:
		return "", fmt.Errorf("GetPreviousPublishedRelease: %w", err)
	}

	return prev.TagName, nil
}

func findPreviousTagName(tags []*git.Tag, target string) (string, bool) {
	foundTarget := false
	targetVersion := parseSemanticVersion(target)

	for _, tag := range tags {
		name := strings.TrimSpace(tag.Name)
		if strings.EqualFold(name, target) {
			foundTarget = true
			continue
		}
		if foundTarget {
			if targetVersion != nil {
				if candidateVersion := parseSemanticVersion(name); candidateVersion != nil && candidateVersion.GreaterThan(targetVersion) {
					continue
				}
			}
			return name, true
		}
	}
	if len(tags) > 0 {
		return strings.TrimSpace(tags[0].Name), true
	}
	return "", false
}

func parseSemanticVersion(tag string) *version.Version {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "v")
	tag = strings.TrimPrefix(tag, "V")
	v, err := version.NewVersion(tag)
	if err != nil {
		return nil
	}
	return v
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
	hasMergedBefore, err := issues_model.HasMergedPullRequestInRepoBefore(ctx, repoID, posterID, int64(pr.MergedUnix), pr.ID)
	if err != nil {
		return false, fmt.Errorf("check merged PRs for contributor: %w", err)
	}
	return !hasMergedBefore, nil
}
