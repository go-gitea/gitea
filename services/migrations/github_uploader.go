// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/repostats"
	system_model "gitea.dev/models/system"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	base "gitea.dev/modules/migration"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
	issue_service "gitea.dev/services/issue"

	"github.com/google/go-github/v92/github"
	"xorm.io/builder"
)

type githubUploader struct {
	*GiteaLocalUploader
	downloader        *GithubDownloaderV3
	sourceCloneURL    string
	migrateIssues     bool
	reviewComments    map[int64]*issues_model.Comment
	replies           map[int64]int64
	comments          map[int64]*issues_model.Comment
	unavailableIssues map[int64]struct{}
	preservedCommits  map[string]struct{}
}

func githubMigrationUploader(uploader *GiteaLocalUploader, downloader base.Downloader, opts base.MigrateOptions) base.Uploader {
	if retry, ok := downloader.(*base.RetryDownloader); ok {
		downloader = retry.Downloader
	}
	if graphQL, ok := downloader.(*GithubGraphQLDownloader); ok {
		downloader = graphQL.GithubDownloaderV3
	}
	if github, ok := downloader.(*GithubDownloaderV3); ok {
		return &githubUploader{GiteaLocalUploader: uploader, downloader: github, migrateIssues: opts.Issues}
	}
	return uploader
}

func (g *githubUploader) CreateRepo(ctx context.Context, repo *base.Repository, opts base.MigrateOptions) error {
	if opts.MigrateToRepoID > 0 {
		r, err := repo_model.GetRepositoryByID(ctx, opts.MigrateToRepoID)
		if err != nil {
			return err
		}
		if err := resetGithubMigrationData(ctx, r); err != nil {
			return err
		}
	}
	if err := g.GiteaLocalUploader.CreateRepo(ctx, repo, opts); err != nil {
		return err
	}
	g.sourceCloneURL = repo.CloneURL
	g.downloader.archive = func(ctx context.Context, kind string, id int64, value any) error {
		f, err := git.CreateRepoFile(ctx, g.repo, fmt.Sprintf("migration/github/%s/%d.json", kind, id))
		if err != nil {
			return err
		}
		writeErr := json.NewEncoder(f).Encode(value)
		closeErr := f.Close()
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	}
	return nil
}

func (g *githubUploader) CreateComments(ctx context.Context, comments ...*base.Comment) error {
	var regular, events []*base.Comment
	for _, comment := range comments {
		if comment.CommentType != "" && comment.CommentType != issues_model.CommentTypeComment.String() {
			events = append(events, comment)
		} else {
			regular = append(regular, comment)
		}
	}
	if len(events) > 0 {
		if err := g.GiteaLocalUploader.CreateComments(ctx, events...); err != nil {
			return err
		}
	}
	comments = regular
	if g.migrateIssues {
		for _, comment := range comments {
			if _, ok := g.issues[comment.IssueIndex]; ok {
				continue
			}
			if _, ok := g.unavailableIssues[comment.IssueIndex]; ok {
				continue
			}
			var issue *base.Issue
			if err := retryGithubMigration(ctx, func() error {
				var err error
				issue, err = g.downloader.GetIssue(ctx, comment.IssueIndex)
				return err
			}); err != nil {
				if response, ok := errors.AsType[*github.ErrorResponse](err); errors.Is(err, errGithubIssueUnavailable) && ok && response.Response != nil {
					if g.unavailableIssues == nil {
						g.unavailableIssues = make(map[int64]struct{})
					}
					g.unavailableIssues[comment.IssueIndex] = struct{}{}
					log.Warn("migration-comment-parent-unavailable repo=%s/%s issue=%d status=%d: skipping comments without creating an issue", g.downloader.repoOwner, g.downloader.repoName, comment.IssueIndex, response.Response.StatusCode)
					continue
				}
				return fmt.Errorf("recover missing IssueIndex %d: %w", comment.IssueIndex, err)
			}
			if err := g.CreateIssues(ctx, issue); err != nil {
				return err
			}
		}
	}
	rows := make([]*issues_model.Comment, 0, len(comments))
	indexes := make([]int64, 0, len(comments))
	for _, c := range comments {
		issue := g.issues[c.IssueIndex]
		if issue == nil {
			if _, ok := g.unavailableIssues[c.IssueIndex]; ok {
				continue
			}
			return fmt.Errorf("comment references non existent IssueIndex %d", c.IssueIndex)
		}
		row := &issues_model.Comment{Type: issues_model.CommentTypeComment, IssueID: issue.ID, Content: c.Content, CreatedUnix: timeutil.TimeStamp(c.Created.Unix()), UpdatedUnix: timeutil.TimeStamp(c.Updated.Unix())}
		if c.Created.IsZero() {
			row.CreatedUnix = issue.CreatedUnix
		}
		if c.Updated.IsZero() {
			row.UpdatedUnix = row.CreatedUnix
		}
		if err := g.remapUser(ctx, c, row); err != nil {
			return err
		}
		for _, source := range c.Reactions {
			r := &issues_model.Reaction{Type: source.Content, CreatedUnix: row.CreatedUnix}
			if err := g.remapUser(ctx, source, r); err != nil {
				return err
			}
			row.Reactions = append(row.Reactions, r)
		}
		rows = append(rows, row)
		indexes = append(indexes, c.Index)
	}
	if len(rows) == 0 {
		return nil
	}
	if err := issues_model.InsertIssueComments(ctx, rows); err != nil {
		return err
	}
	if g.comments == nil {
		g.comments = make(map[int64]*issues_model.Comment)
	}
	for i, index := range indexes {
		g.comments[index] = rows[i]
	}
	return nil
}

func (g *githubUploader) CreatePullRequests(ctx context.Context, prs ...*base.PullRequest) error {
	prepared := make([]*base.PullRequest, 0, len(prs))
	for _, pr := range prs {
		if !pr.EnsuredSafe {
			return fmt.Errorf("the PR[%d] was not checked for safety", pr.Number)
		}
		if g.gitRepo != nil {
			if pr.Head.SHA == "" || pr.Base.SHA == "" {
				return fmt.Errorf("PR %d has no head or base commit", pr.Number)
			}
			if _, err := g.gitRepo.GetCommit(ctx, pr.Head.SHA); err != nil {
				if g.sourceCloneURL == "" {
					return fmt.Errorf("GitHub source clone URL is unavailable for PR %d", pr.Number)
				}
				if err := retryGithubMigration(ctx, func() error {
					_, _, fetchErr := gitcmd.NewCommand("fetch", "--no-tags").AddDynamicArguments(g.sourceCloneURL, fmt.Sprintf("refs/pull/%d/head", pr.Number)).WithRepo(g.repo).RunStdString(ctx)
					if fetchErr != nil {
						return util.SanitizeErrorCredentialURLs(fetchErr)
					}
					return nil
				}); err != nil {
					return fmt.Errorf("fetch PR %d head: %w", pr.Number, err)
				}
			}
			commits := []string{pr.Head.SHA, pr.Base.SHA}
			if pr.Merged && pr.MergeCommitSHA != "" {
				commits = append(commits, pr.MergeCommitSHA)
			}
			for _, sha := range commits {
				if err := g.preserveCommit(ctx, sha); err != nil {
					return fmt.Errorf("PR %d commit %s unavailable: %w", pr.Number, sha, err)
				}
			}
			if err := git.UpdateRef(ctx, g.repo, pr.GetGitHeadRefName(), pr.Head.SHA); err != nil {
				return err
			}
		}
		if pr.PatchURL != "" {
			if err := retryGithubMigration(ctx, func() error { return g.downloadPatch(ctx, pr) }); err != nil {
				return err
			}
		}
		preparedPR := *pr
		if g.gitRepo != nil {
			mergeBase, err := git.MergeBase(ctx, g.repo, pr.Base.SHA, pr.Head.SHA)
			if err != nil {
				return fmt.Errorf("PR %d merge base: %w", pr.Number, err)
			}
			preparedPR.Base.SHA = mergeBase
		}
		// The patch is already stored in the destination repository.
		preparedPR.PatchURL = ""
		prepared = append(prepared, &preparedPR)
	}
	return g.GiteaLocalUploader.CreatePullRequests(ctx, prepared...)
}

func (g *githubUploader) preserveCommit(ctx context.Context, sha string) error {
	if _, ok := g.preservedCommits[sha]; ok {
		return nil
	}
	if _, err := g.gitRepo.GetCommit(ctx, sha); err != nil {
		if g.sourceCloneURL == "" {
			return errors.New("GitHub source clone URL is unavailable")
		}
		if err := retryGithubMigration(ctx, func() error {
			_, _, fetchErr := gitcmd.NewCommand("fetch", "--no-tags").AddDynamicArguments(g.sourceCloneURL, sha).WithRepo(g.repo).RunStdString(ctx)
			if fetchErr != nil {
				return util.SanitizeErrorCredentialURLs(fetchErr)
			}
			return nil
		}); err != nil {
			return err
		}
		if _, err := g.gitRepo.GetCommit(ctx, sha); err != nil {
			return err
		}
	}
	if err := git.UpdateRef(ctx, g.repo, "refs/migration/github/commits/"+sha, sha); err != nil {
		return err
	}
	if g.preservedCommits == nil {
		g.preservedCommits = make(map[string]struct{})
	}
	g.preservedCommits[sha] = struct{}{}
	return nil
}

func (g *githubUploader) downloadPatch(ctx context.Context, pr *base.PullRequest) error {
	var content io.Reader
	if g.downloader != nil {
		g.downloader.waitAndPickClient(ctx)
		patch, resp, err := g.downloader.getClient().PullRequests.GetRaw(ctx, g.downloader.repoOwner, g.downloader.repoName, int(pr.Number), github.RawOptions{Type: github.Patch})
		if resp != nil {
			g.downloader.setRate(&resp.Rate)
		}
		if err != nil {
			if apiErr, ok := errors.AsType[*github.ErrorResponse](err); ok && g.gitRepo != nil && apiErr.Response != nil && apiErr.Response.StatusCode == http.StatusNotAcceptable && slices.ContainsFunc(apiErr.Errors, func(e github.Error) bool {
				return e.Resource == "PullRequest" && e.Field == "diff" && e.Code == "too_large"
			}) {
				return g.generatePatch(ctx, pr)
			}
			return err
		}
		content = strings.NewReader(patch)
	} else {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pr.PatchURL, nil)
		if err != nil {
			return err
		}
		resp, err := getMigrationHTTPClient().Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("download PR %d patch: HTTP %d", pr.Number, resp.StatusCode)
		}
		content = resp.Body
	}
	f, err := git.CreateRepoFile(ctx, g.repo, fmt.Sprintf("pulls/%d.patch", pr.Number))
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, content)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func (g *githubUploader) generatePatch(ctx context.Context, pr *base.PullRequest) error {
	if pr.Base.SHA == "" || pr.Head.SHA == "" {
		return fmt.Errorf("PR %d has no head or base commit", pr.Number)
	}
	f, err := git.CreateRepoFile(ctx, g.repo, fmt.Sprintf("pulls/%d.patch", pr.Number))
	if err != nil {
		return err
	}
	// A tree diff retains merge resolutions that format-patch would omit.
	writeErr := g.gitRepo.GetDiffBinary(ctx, pr.Base.SHA+"..."+pr.Head.SHA, f)
	closeErr := f.Close()
	if writeErr != nil {
		return fmt.Errorf("generate PR %d patch: %w", pr.Number, writeErr)
	}
	return closeErr
}

func (g *githubUploader) CreateReviews(ctx context.Context, reviews ...*base.Review) error {
	if g.reviewComments == nil {
		g.reviewComments = make(map[int64]*issues_model.Comment)
		g.replies = make(map[int64]int64)
	}
	comments := make(map[int64]*issues_model.Comment)
	var rows []*issues_model.Review
	for _, source := range reviews {
		issue, ok := g.issues[source.IssueIndex]
		if !ok {
			return fmt.Errorf("review references missing IssueIndex %d", source.IssueIndex)
		}
		row := &issues_model.Review{IssueID: issue.ID, Type: convertReviewState(source.State), Content: source.Content, Official: source.Official, Dismissed: source.Dismissed, CreatedUnix: timeutil.TimeStamp(source.CreatedAt.Unix()), UpdatedUnix: timeutil.TimeStamp(source.CreatedAt.Unix())}
		if source.CreatedAt.IsZero() {
			row.CreatedUnix = issue.CreatedUnix
			row.UpdatedUnix = issue.CreatedUnix
		}
		if err := g.remapUser(ctx, source, row); err != nil {
			return err
		}
		for _, c := range source.Comments {
			if _, ok := g.reviewComments[c.ID]; ok {
				return fmt.Errorf("duplicate GitHub review comment %d", c.ID)
			}
			if _, ok := comments[c.ID]; ok {
				return fmt.Errorf("duplicate GitHub review comment %d", c.ID)
			}
			if c.CommitID == "" || c.TreePath == "" {
				return fmt.Errorf("GitHub review comment %d lacks commit or path", c.ID)
			}
			if g.gitRepo != nil {
				if err := g.preserveCommit(ctx, c.CommitID); err != nil {
					return fmt.Errorf("review comment %d commit %s unavailable: %w", c.ID, c.CommitID, err)
				}
			}
			line := c.Line
			if line == 0 {
				_, _, line, _ = git.ParseDiffHunkString(c.DiffHunk)
				line += c.Position - 1
			}
			comment := &issues_model.Comment{Type: issues_model.CommentTypeCode, IssueID: issue.ID, Content: c.Content, Line: int64(line), TreePath: util.PathJoinRel(c.TreePath), CommitSHA: c.CommitID, Patch: c.DiffHunk, CreatedUnix: timeutil.TimeStamp(c.CreatedAt.Unix()), UpdatedUnix: timeutil.TimeStamp(c.UpdatedAt.Unix())}
			if c.Resolved {
				comment.ResolveDoerID = g.doer.ID
			}
			if c.CreatedAt.IsZero() {
				comment.CreatedUnix = row.CreatedUnix
			}
			if c.UpdatedAt.IsZero() {
				comment.UpdatedUnix = comment.CreatedUnix
			}
			if err := g.remapUser(ctx, &base.Comment{PosterID: c.PosterID}, comment); err != nil {
				return err
			}
			for _, reaction := range c.Reactions {
				r := &issues_model.Reaction{Type: reaction.Content, IssueID: issue.ID, CreatedUnix: comment.CreatedUnix}
				if err := g.remapUser(ctx, reaction, r); err != nil {
					return err
				}
				comment.Reactions = append(comment.Reactions, r)
			}
			row.Comments = append(row.Comments, comment)
			comments[c.ID] = comment
		}
		rows = append(rows, row)
	}
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		for _, row := range rows {
			if _, err := db.GetEngine(ctx).NoAutoTime().Insert(row); err != nil {
				return err
			}
			summary := &issues_model.Comment{Type: issues_model.CommentTypeReview, IssueID: row.IssueID, ReviewID: row.ID, Content: row.Content, PosterID: row.ReviewerID, OriginalAuthor: row.OriginalAuthor, OriginalAuthorID: row.OriginalAuthorID, CreatedUnix: row.CreatedUnix, UpdatedUnix: row.UpdatedUnix}
			if _, err := db.GetEngine(ctx).NoAutoTime().Insert(summary); err != nil {
				return err
			}
			for _, comment := range row.Comments {
				comment.ReviewID = row.ID
				if _, err := db.GetEngine(ctx).NoAutoTime().Insert(comment); err != nil {
					return err
				}
			}
			if err := issues_model.UpdateIssueNumComments(ctx, row.IssueID); err != nil {
				return err
			}
		}
		for _, c := range comments {
			for _, r := range c.Reactions {
				r.CommentID = c.ID
				if err := db.Insert(ctx, r); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}
	maps.Copy(g.reviewComments, comments)
	for _, r := range reviews {
		for _, c := range r.Comments {
			if c.InReplyTo != 0 {
				g.replies[c.ID] = c.InReplyTo
			}
		}
	}
	return nil
}

func (g *githubUploader) Finish(ctx context.Context) error {
	for child, parent := range g.replies {
		c, p := g.reviewComments[child], g.reviewComments[parent]
		if p == nil || c == nil || c.IssueID != p.IssueID {
			return fmt.Errorf("GitHub review comment %d references missing or unrelated parent %d", child, parent)
		}
		c.Content += fmt.Sprintf("\n\n[Reply to GitHub comment %d](%s)", parent, p.HTMLURL(ctx))
		if _, err := db.GetEngine(ctx).NoAutoTime().ID(c.ID).Cols("content").Update(c); err != nil {
			return err
		}
	}
	for _, issue := range g.issues {
		if g.downloader != nil && g.downloader.archive != nil {
			if err := g.preserveTimeline(ctx, issue); err != nil {
				return err
			}
		}
		if content := g.rewriteLinks(ctx, issue.Content); content != issue.Content {
			issue.Content = content
			if _, err := db.GetEngine(ctx).NoAutoTime().ID(issue.ID).Cols("content").Update(issue); err != nil {
				return err
			}
		}
		if err := issue.AddCrossReferences(ctx, g.doer, false); err != nil {
			return err
		}
	}
	var comments []*issues_model.Comment
	if err := db.GetEngine(ctx).In("issue_id", builder.Select("id").From("issue").Where(builder.Eq{"repo_id": g.repo.ID})).In("type", []issues_model.CommentType{issues_model.CommentTypeComment, issues_model.CommentTypeCode, issues_model.CommentTypeReview}).Find(&comments); err != nil {
		return err
	}
	for _, comment := range comments {
		if content := g.rewriteLinks(ctx, comment.Content); content != comment.Content {
			comment.Content = content
			if _, err := db.GetEngine(ctx).NoAutoTime().ID(comment.ID).Cols("content").Update(comment); err != nil {
				return err
			}
		}
		if err := comment.AddCrossReferences(ctx, g.doer, false); err != nil {
			return err
		}
	}
	if g.downloader != nil {
		mapping := make(map[string]map[int64]int64)
		mapping["issues"] = make(map[int64]int64)
		mapping["comments"] = make(map[int64]int64)
		mapping["review_comments"] = make(map[int64]int64)
		for id, issue := range g.issues {
			mapping["issues"][id] = issue.ID
		}
		for id, comment := range g.comments {
			mapping["comments"][id] = comment.ID
		}
		for id, comment := range g.reviewComments {
			mapping["review_comments"][id] = comment.ID
		}
		if err := g.downloader.archiveSource(ctx, "mapping", 0, mapping); err != nil {
			return err
		}
	}
	return g.GiteaLocalUploader.Finish(ctx)
}

func (g *githubUploader) rewriteLinks(ctx context.Context, content string) string {
	if g.downloader == nil {
		return content
	}
	prefix := g.downloader.baseURL + "/" + g.downloader.repoOwner + "/" + g.downloader.repoName
	pattern := regexp.MustCompile(regexp.QuoteMeta(prefix) + `/(?:issues|pull)/([0-9]+)(?:#(issuecomment-|discussion_r)([0-9]+))?(?:[^a-zA-Z0-9_/#?=-]|$)`)
	return pattern.ReplaceAllStringFunc(content, func(link string) string {
		match := pattern.FindStringSubmatch(link)
		index, _ := strconv.ParseInt(match[1], 10, 64)
		issue := g.issues[index]
		if issue == nil {
			return link
		}
		original := prefix
		if strings.HasPrefix(link[len(prefix):], "/pull/") {
			original += "/pull/"
		} else {
			original += "/issues/"
		}
		original += match[1]
		target := issue.HTMLURL(ctx)
		if match[2] != "" {
			id, _ := strconv.ParseInt(match[3], 10, 64)
			comment := g.comments[id]
			if match[2] == "discussion_r" {
				comment = g.reviewComments[id]
			}
			if comment == nil || comment.IssueID != issue.ID {
				return link
			}
			target = comment.HTMLURL(ctx)
			original += "#" + match[2] + match[3]
		}
		return target + strings.TrimPrefix(link, original)
	})
}

func (g *githubUploader) preserveTimeline(ctx context.Context, issue *issues_model.Issue) error {
	for page := 1; ; {
		var events []*github.Timeline
		var resp *github.Response
		if err := retryGithubMigration(ctx, func() error {
			g.downloader.waitAndPickClient(ctx)
			var err error
			events, resp, err = g.downloader.getClient().Issues.ListIssueTimeline(ctx, g.downloader.repoOwner, g.downloader.repoName, int(issue.Index), &github.ListOptions{Page: page, PerPage: 100})
			if resp != nil {
				g.downloader.setRate(&resp.Rate)
			}
			return err
		}); err != nil {
			return err
		}
		if err := g.downloader.archiveSource(ctx, fmt.Sprintf("timelines/%d", issue.Index), int64(page), events); err != nil {
			return err
		}
		for _, event := range events {
			if event.GetEvent() != "cross-referenced" && event.GetEvent() != "closed" && event.GetEvent() != "referenced" {
				continue
			}
			link := g.downloader.baseURL + "/" + g.downloader.repoOwner + "/" + g.downloader.repoName + fmt.Sprintf("/issues/%d", issue.Index)
			if source := event.GetSource().GetIssue(); source != nil {
				link = source.GetHTMLURL()
				if strings.EqualFold(source.GetRepositoryURL(), g.downloader.getClient().BaseURL()+"repos/"+g.downloader.repoOwner+"/"+g.downloader.repoName) {
					if target := g.issues[int64(source.GetNumber())]; target != nil {
						if err := target.LoadRepo(ctx); err != nil {
							return err
						}
						link = target.HTMLURL(ctx)
					}
				}
			} else if event.GetCommitID() != "" {
				link = g.downloader.baseURL + "/" + g.downloader.repoOwner + "/" + g.downloader.repoName + "/commit/" + event.GetCommitID()
			}
			comment := &issues_model.Comment{Type: issues_model.CommentTypeComment, IssueID: issue.ID, Content: fmt.Sprintf("GitHub event: %s — [source](%s)", event.GetEvent(), link), CreatedUnix: timeutil.TimeStamp(event.GetCreatedAt().Unix()), UpdatedUnix: timeutil.TimeStamp(event.GetCreatedAt().Unix())}
			if err := g.remapUser(ctx, &base.Comment{PosterID: event.GetActor().GetID(), PosterName: event.GetActor().GetLogin()}, comment); err != nil {
				return err
			}
			if err := issues_model.InsertIssueComments(ctx, []*issues_model.Comment{comment}); err != nil {
				return err
			}
		}
		if resp.NextPage == 0 {
			return nil
		}
		page = resp.NextPage
	}
}

func retryGithubMigration(ctx context.Context, work func() error) error {
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := work()
		if err == nil || attempt+1 >= max(1, setting.Migrations.MaxAttempts) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(setting.Migrations.RetryBackoff) * time.Second):
		}
	}
}

func resetGithubMigrationData(ctx context.Context, repo *repo_model.Repository) error {
	if !repo.IsBeingMigrated() {
		return fmt.Errorf("repository %d is not being migrated", repo.ID)
	}
	var attachmentPaths []string
	err := db.WithTx(ctx, func(ctx context.Context) error {
		var attachments []*repo_model.Attachment
		if err := db.GetEngine(ctx).Where("repo_id = ?", repo.ID).Find(&attachments); err != nil {
			return err
		}
		for _, attachment := range attachments {
			attachmentPaths = append(attachmentPaths, attachment.RelativePath())
		}
		if _, err := repo_model.DeleteAttachments(ctx, attachments, false); err != nil {
			return err
		}
		issueIDs := builder.Select("id").From("issue").Where(builder.Eq{"repo_id": repo.ID})
		if _, err := db.GetEngine(ctx).In("issue_id", issueIDs).Delete(new(issues_model.Review)); err != nil {
			return err
		}
		if err := issues_model.DeletePullsByBaseRepoID(ctx, repo.ID); err != nil {
			return err
		}
		paths, err := issue_service.DeleteIssuesByRepoID(ctx, repo.ID)
		if err != nil {
			return err
		}
		attachmentPaths = append(attachmentPaths, paths...)
		if err := issues_model.DeleteLabelsByRepoID(ctx, repo.ID); err != nil {
			return err
		}
		if err := db.DeleteBeans(ctx, &issues_model.Milestone{RepoID: repo.ID}, &repo_model.Release{RepoID: repo.ID}); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).ID(repo.ID).Cols("num_milestones", "num_closed_milestones").Update(new(repo_model.Repository)); err != nil {
			return err
		}
		return repostats.UpdateRepoStats(ctx, repo.ID)
	})
	if err != nil {
		return err
	}
	repo.NumIssues, repo.NumClosedIssues, repo.NumPulls, repo.NumClosedPulls = 0, 0, 0, 0
	repo.NumMilestones, repo.NumClosedMilestones = 0, 0
	for _, path := range attachmentPaths {
		system_model.RemoveStorageWithNotice(ctx, storage.Attachments, "Delete migration attachment", path)
	}
	return nil
}
