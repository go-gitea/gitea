// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"
	"slices"

	"gitea.dev/models/db"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// VisibilityScope describes which confidential issues a viewer may see; a nil scope is an anonymous viewer.
type VisibilityScope struct {
	Unrestricted  bool    // site admin with full access: no condition
	DoerID        int64   // 0 for anonymous and public-only tokens
	WriterRepoIDs []int64 // repos where the viewer has Write on the Issues unit
}

// applyVisibilityConditions adds the confidential-issue rule and the optional confidential filter of opts to sess.
func applyVisibilityConditions(sess db.Session, opts *IssuesOptions) {
	sess.And(opts.Visibility.Cond())
	if opts.Confidential.Has() {
		sess.And(builder.Eq{"issue.is_confidential": opts.Confidential.Value()})
	}
}

func newVisibilityScope(doer *user_model.User) *VisibilityScope {
	scope := &VisibilityScope{Unrestricted: user_model.DoerIsSiteAdminFullAccess(doer)}
	if doer != nil && !user_model.IsPublicOnlyDoer(doer) {
		scope.DoerID = doer.ID
	}
	return scope
}

// NewVisibilityScopeForRepo builds the scope for a single repository from an already computed permission, without SQL.
func NewVisibilityScopeForRepo(doer *user_model.User, repoID int64, perm access_model.Permission) *VisibilityScope {
	scope := newVisibilityScope(doer)
	if scope.DoerID > 0 && perm.CanWriteIssuesOrPulls(false) {
		scope.WriterRepoIDs = []int64{repoID}
	}
	return scope
}

// BuildVisibilityScope builds the scope for the given repositories (and all public ones if allPublic) with one query
// over confidential issues; permissions are only computed for repositories that have confidential issues.
func BuildVisibilityScope(ctx context.Context, doer *user_model.User, repoIDs []int64, allPublic bool) (*VisibilityScope, error) {
	cond := builder.In("issue.repo_id", repoIDs)
	if allPublic {
		cond = builder.Or(cond, builder.In("issue.repo_id", builder.Select("id").From("repository").Where(repo_model.PublicRepoUnderPublicOwnerCond())))
	}
	scope, err := buildVisibilityScopeByIssueCond(ctx, doer, cond)
	if err != nil {
		return nil, fmt.Errorf("BuildVisibilityScope: %w", err)
	}
	return scope, nil
}

// BuildVisibilityScopeByCond is like BuildVisibilityScope, repoCond selects repository.id.
func BuildVisibilityScopeByCond(ctx context.Context, doer *user_model.User, repoCond builder.Cond) (*VisibilityScope, error) {
	if repoCond == nil {
		repoCond = builder.NewCond()
	}
	scope, err := buildVisibilityScopeByIssueCond(ctx, doer, builder.In("issue.repo_id", builder.Select("id").From("repository").Where(repoCond)))
	if err != nil {
		return nil, fmt.Errorf("BuildVisibilityScopeByCond: %w", err)
	}
	return scope, nil
}

func buildVisibilityScopeByIssueCond(ctx context.Context, doer *user_model.User, issueCond builder.Cond) (*VisibilityScope, error) {
	scope := newVisibilityScope(doer)
	if scope.Unrestricted || scope.DoerID == 0 {
		return scope, nil
	}

	var repoIDs []int64
	if err := db.GetEngine(ctx).Table("issue").Distinct("repo_id").
		Where(builder.Eq{"is_confidential": true}).And(issueCond).
		Find(&repoIDs); err != nil {
		return nil, err
	}
	repos, err := repo_model.GetRepositoriesMapByIDs(ctx, repoIDs)
	if err != nil {
		return nil, err
	}
	for _, repoID := range repoIDs {
		repo, ok := repos[repoID]
		if !ok {
			continue
		}
		perm, err := access_model.GetDoerRepoPermission(ctx, repo, doer)
		if err != nil {
			return nil, err
		}
		if perm.CanWriteIssuesOrPulls(false) {
			scope.WriterRepoIDs = append(scope.WriterRepoIDs, repoID)
		}
	}
	return scope, nil
}

// Cond returns the SQL equivalent of IsIssueVisible's confidential rule; it needs the "issue" table in the query.
func (s *VisibilityScope) Cond() builder.Cond {
	if s != nil && s.Unrestricted {
		return builder.NewCond()
	}
	cond := builder.Or(builder.Eq{"issue.is_confidential": false})
	if s == nil {
		return cond
	}
	if s.DoerID > 0 {
		cond = cond.Or(
			builder.Eq{"issue.poster_id": s.DoerID, "issue.original_author_id": 0}, // same as Issue.IsPoster
			builder.In("issue.id", builder.Select("issue_id").From("issue_assignees").Where(builder.Eq{"assignee_id": s.DoerID})),
		)
	}
	if len(s.WriterRepoIDs) > 0 {
		cond = cond.Or(builder.In("issue.repo_id", s.WriterRepoIDs))
	}
	return cond
}

// SeesAllIssuesOf returns true if the viewer sees every issue of the repository, confidential or not.
func (s *VisibilityScope) SeesAllIssuesOf(repoID int64) bool {
	if s == nil {
		return false
	}
	return s.Unrestricted || slices.Contains(s.WriterRepoIDs, repoID)
}

// IsIssueVisible tells whether doer may see the issue; perm is doer's permission on issue.Repo.
func IsIssueVisible(ctx context.Context, issue *Issue, doer *user_model.User, perm access_model.Permission) (bool, error) {
	if !perm.CanReadIssuesOrPulls(issue.IsPull) {
		return false, nil
	}
	if !issue.IsConfidential {
		return true, nil
	}
	scope := NewVisibilityScopeForRepo(doer, issue.RepoID, perm)
	if scope.Unrestricted || scope.SeesAllIssuesOf(issue.RepoID) {
		return true, nil
	}
	if scope.DoerID == 0 {
		return false, nil
	}
	if issue.IsPoster(scope.DoerID) {
		return true, nil
	}
	isAssigned, err := IsUserAssignedToIssue(ctx, issue, scope.DoerID)
	if err != nil {
		return false, fmt.Errorf("IsIssueVisible: %w", err)
	}
	return isAssigned, nil
}

// CheckIssueVisible returns ErrIssueNotExist when the issue is hidden from doer, so a hidden issue looks like a missing one.
func CheckIssueVisible(ctx context.Context, issue *Issue, doer *user_model.User, perm access_model.Permission) error {
	visible, err := IsIssueVisible(ctx, issue, doer, perm)
	if err != nil {
		return fmt.Errorf("CheckIssueVisible: %w", err)
	}
	if !visible {
		return ErrIssueNotExist{0, issue.RepoID, issue.Index}
	}
	return nil
}

// IsIssueVisibleForDoer is IsIssueVisible with the permission computed from issue.Repo.
func IsIssueVisibleForDoer(ctx context.Context, issue *Issue, doer *user_model.User) (bool, error) {
	if err := issue.LoadRepo(ctx); err != nil {
		return false, fmt.Errorf("IsIssueVisibleForDoer: %w", err)
	}
	perm, err := access_model.GetDoerRepoPermission(ctx, issue.Repo, doer)
	if err != nil {
		return false, fmt.Errorf("IsIssueVisibleForDoer: %w", err)
	}
	return IsIssueVisible(ctx, issue, doer, perm)
}

// hiddenConfidentialCond selects the confidential issues matching issueCond that the scope cannot see.
func hiddenConfidentialCond(scope *VisibilityScope, issueCond builder.Cond) builder.Cond {
	cond := builder.And(builder.Eq{"issue.is_confidential": true}, builder.Not{scope.Cond()})
	if issueCond != nil {
		cond = cond.And(issueCond)
	}
	return cond
}

// HiddenConfidentialIssuesOfActionsCond matches action rows whose Content ("index|title") refers to a confidential issue hidden by scope.
func HiddenConfidentialIssuesOfActionsCond(scope *VisibilityScope) builder.Cond {
	indexLike := "CONCAT(issue.`index`, '|%')"
	if setting.Database.Type.IsSQLite3() {
		indexLike = "CAST(issue.`index` AS TEXT) || '|%'" // CONCAT needs SQLite 3.44+
	}
	return builder.Exists(builder.Select("1").From("issue").Where(builder.And(
		builder.Expr("issue.repo_id = action.repo_id"),
		hiddenConfidentialCond(scope, nil),
		builder.Expr("action.content LIKE "+indexLike),
	)))
}

// FindHiddenConfidentialIssues returns the confidential issues matching cond (on the issue table) hidden from doer.
// It returns nil without SQL for a full-access site admin. Only id, repo_id, index and is_pull are loaded.
func FindHiddenConfidentialIssues(ctx context.Context, doer *user_model.User, cond builder.Cond) (IssueList, error) {
	if user_model.DoerIsSiteAdminFullAccess(doer) {
		return nil, nil
	}
	scope, err := buildVisibilityScopeByIssueCond(ctx, doer, orAll(cond))
	if err != nil {
		return nil, fmt.Errorf("FindHiddenConfidentialIssues: %w", err)
	}
	issues := IssueList{}
	if err := db.GetEngine(ctx).Table("issue").Cols("id", "repo_id", "index", "is_pull").
		Where(hiddenConfidentialCond(scope, cond)).Find(&issues); err != nil {
		return nil, fmt.Errorf("FindHiddenConfidentialIssues: %w", err)
	}
	return issues, nil
}

func orAll(cond builder.Cond) builder.Cond {
	if cond == nil {
		return builder.NewCond()
	}
	return cond
}

// HiddenIssueCounts is the number of open and closed issues hidden from a viewer.
type HiddenIssueCounts struct{ Open, Closed int }

// CountHiddenConfidentialIssues counts the confidential issues matching cond that scope cannot see; no SQL when Unrestricted.
func CountHiddenConfidentialIssues(ctx context.Context, scope *VisibilityScope, cond builder.Cond) (int64, error) {
	if scope != nil && scope.Unrestricted {
		return 0, nil
	}
	count, err := db.GetEngine(ctx).Table("issue").Where(hiddenConfidentialCond(scope, cond)).Count()
	if err != nil {
		return 0, fmt.Errorf("CountHiddenConfidentialIssues: %w", err)
	}
	return count, nil
}

func countHiddenConfidentialIssuesBy(ctx context.Context, scope *VisibilityScope, idColumn string, ids []int64, join func(sess db.Session)) (map[int64]HiddenIssueCounts, error) {
	counts := make(map[int64]HiddenIssueCounts, len(ids))
	if len(ids) == 0 || (scope != nil && scope.Unrestricted) {
		return counts, nil
	}
	rows := make([]struct {
		ID       int64
		IsClosed bool
		Count    int
	}, 0, len(ids))
	sess := db.GetEngine(ctx).Table("issue")
	join(sess)
	if err := sess.Where(hiddenConfidentialCond(scope, builder.In(idColumn, ids))).
		Select(idColumn + " AS id, issue.is_closed AS is_closed, COUNT(*) AS count").
		GroupBy(idColumn + ", issue.is_closed").Find(&rows); err != nil {
		return nil, err
	}
	for _, row := range rows {
		c := counts[row.ID]
		if row.IsClosed {
			c.Closed += row.Count
		} else {
			c.Open += row.Count
		}
		counts[row.ID] = c
	}
	return counts, nil
}

// CountHiddenConfidentialIssuesByMilestone returns the hidden issue counts per milestone ID.
func CountHiddenConfidentialIssuesByMilestone(ctx context.Context, scope *VisibilityScope, milestoneIDs []int64) (map[int64]HiddenIssueCounts, error) {
	counts, err := countHiddenConfidentialIssuesBy(ctx, scope, "issue.milestone_id", milestoneIDs, func(db.Session) {})
	if err != nil {
		return nil, fmt.Errorf("CountHiddenConfidentialIssuesByMilestone: %w", err)
	}
	return counts, nil
}

// CountHiddenConfidentialIssuesByLabel returns the hidden issue counts per label ID.
func CountHiddenConfidentialIssuesByLabel(ctx context.Context, scope *VisibilityScope, labelIDs []int64) (map[int64]HiddenIssueCounts, error) {
	counts, err := countHiddenConfidentialIssuesBy(ctx, scope, "issue_label.label_id", labelIDs, func(sess db.Session) {
		sess.Join("INNER", "issue_label", "issue_label.issue_id = issue.id")
	})
	if err != nil {
		return nil, fmt.Errorf("CountHiddenConfidentialIssuesByLabel: %w", err)
	}
	return counts, nil
}

// CountOpenIssuesForViewer returns the number of open issues of the repository the viewer can see.
// The stored repository.num_issues counters are never changed.
func CountOpenIssuesForViewer(ctx context.Context, repo *repo_model.Repository, scope *VisibilityScope) (int, error) {
	if scope.SeesAllIssuesOf(repo.ID) {
		return repo.NumOpenIssues, nil
	}
	hidden, err := CountHiddenConfidentialIssues(ctx, scope, builder.Eq{"issue.repo_id": repo.ID, "issue.is_pull": false, "issue.is_closed": false})
	if err != nil {
		return 0, fmt.Errorf("CountOpenIssuesForViewer: %w", err)
	}
	return max(repo.NumOpenIssues-int(hidden), 0), nil
}

// IssueConfidentialOptions defines options for making an issue confidential or public.
type IssueConfidentialOptions struct {
	Doer  *user_model.User
	Issue *Issue
}

// SetIssueConfidential changes the confidential flag and records a timeline comment in one transaction.
// It returns (nil, nil) when the flag already has the requested value.
func SetIssueConfidential(ctx context.Context, opts *IssueConfidentialOptions, confidential bool) (*Comment, error) {
	issue := opts.Issue
	if issue.IsConfidential == confidential {
		return nil, nil //nolint:nilnil // documented contract: nil comment means nothing changed
	}
	if confidential && issue.IsPull {
		return nil, util.NewInvalidArgumentErrorf("pull requests cannot be confidential")
	}

	commentType := CommentTypeMakePublic
	if confidential {
		commentType = CommentTypeMakeConfidential
	}

	var comment *Comment
	issue.IsConfidential = confidential
	err := db.WithTx(ctx, func(ctx context.Context) (err error) {
		if err = UpdateIssueCols(ctx, issue, "is_confidential"); err != nil {
			return err
		}
		comment, err = CreateComment(ctx, &CreateCommentOptions{
			Doer:  opts.Doer,
			Issue: issue,
			Repo:  issue.Repo,
			Type:  commentType,
		})
		return err
	})
	if err != nil {
		issue.IsConfidential = !confidential
		return nil, fmt.Errorf("SetIssueConfidential: %w", err)
	}
	return comment, nil
}
