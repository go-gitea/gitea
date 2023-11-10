// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	system_model "code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/references"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

// UpdateIssueCols updates cols of issue
func UpdateIssueCols(ctx context.Context, issue *Issue, cols ...string) error {
	if _, err := db.GetEngine(ctx).ID(issue.ID).Cols(cols...).Update(issue); err != nil {
		return err
	}
	return nil
}

func changeIssueStatus(ctx context.Context, issue *Issue, doer *user_model.User, isClosed, isMergePull bool) (*Comment, error) {
	// Reload the issue
	currentIssue, err := GetIssueByID(ctx, issue.ID)
	if err != nil {
		return nil, err
	}

	// Nothing should be performed if current status is same as target status
	if currentIssue.IsClosed == isClosed {
		if !issue.IsPull {
			return nil, ErrIssueWasClosed{
				ID: issue.ID,
			}
		}
		return nil, ErrPullWasClosed{
			ID: issue.ID,
		}
	}

	issue.IsClosed = isClosed
	return doChangeIssueStatus(ctx, issue, doer, isMergePull)
}

func doChangeIssueStatus(ctx context.Context, issue *Issue, doer *user_model.User, isMergePull bool) (*Comment, error) {
	// Check for open dependencies
	if issue.IsClosed && issue.Repo.IsDependenciesEnabled(ctx) {
		// only check if dependencies are enabled and we're about to close an issue, otherwise reopening an issue would fail when there are unsatisfied dependencies
		noDeps, err := IssueNoDependenciesLeft(ctx, issue)
		if err != nil {
			return nil, err
		}

		if !noDeps {
			return nil, ErrDependenciesLeft{issue.ID}
		}
	}

	if issue.IsClosed {
		issue.ClosedUnix = timeutil.TimeStampNow()
	} else {
		issue.ClosedUnix = 0
	}

	if err := UpdateIssueCols(ctx, issue, "is_closed", "closed_unix"); err != nil {
		return nil, err
	}

	// Update issue count of labels
	if err := issue.LoadLabels(ctx); err != nil {
		return nil, err
	}
	for idx := range issue.Labels {
		if err := updateLabelCols(ctx, issue.Labels[idx], "num_issues", "num_closed_issue"); err != nil {
			return nil, err
		}
	}

	// Update issue count of milestone
	if issue.MilestoneID > 0 {
		if err := UpdateMilestoneCounters(ctx, issue.MilestoneID); err != nil {
			return nil, err
		}
	}

	// update repository's issue closed number
	if err := repo_model.UpdateRepoIssueNumbers(ctx, issue.RepoID, issue.IsPull, true); err != nil {
		return nil, err
	}

	// New action comment
	cmtType := CommentTypeClose
	if !issue.IsClosed {
		cmtType = CommentTypeReopen
	} else if isMergePull {
		cmtType = CommentTypeMergePull
	}

	return CreateComment(ctx, &CreateCommentOptions{
		Type:  cmtType,
		Doer:  doer,
		Repo:  issue.Repo,
		Issue: issue,
	})
}

// ChangeIssueStatus changes issue status to open or closed.
func ChangeIssueStatus(ctx context.Context, issue *Issue, doer *user_model.User, isClosed bool) (*Comment, error) {
	if err := issue.LoadRepo(ctx); err != nil {
		return nil, err
	}
	if err := issue.LoadPoster(ctx); err != nil {
		return nil, err
	}

	return changeIssueStatus(ctx, issue, doer, isClosed, false)
}

// ChangeIssueTitle changes the title of this issue, as the given user.
func ChangeIssueTitle(ctx context.Context, issue *Issue, doer *user_model.User, oldTitle string) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = UpdateIssueCols(ctx, issue, "name"); err != nil {
		return fmt.Errorf("updateIssueCols: %w", err)
	}

	if err = issue.LoadRepo(ctx); err != nil {
		return fmt.Errorf("loadRepo: %w", err)
	}

	opts := &CreateCommentOptions{
		Type:     CommentTypeChangeTitle,
		Doer:     doer,
		Repo:     issue.Repo,
		Issue:    issue,
		OldTitle: oldTitle,
		NewTitle: issue.Title,
	}
	if _, err = CreateComment(ctx, opts); err != nil {
		return fmt.Errorf("createComment: %w", err)
	}
	if err = issue.AddCrossReferences(ctx, doer, true); err != nil {
		return err
	}

	return committer.Commit()
}

// ChangeIssueRef changes the branch of this issue, as the given user.
func ChangeIssueRef(ctx context.Context, issue *Issue, doer *user_model.User, oldRef string) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err = UpdateIssueCols(ctx, issue, "ref"); err != nil {
		return fmt.Errorf("updateIssueCols: %w", err)
	}

	if err = issue.LoadRepo(ctx); err != nil {
		return fmt.Errorf("loadRepo: %w", err)
	}
	oldRefFriendly := strings.TrimPrefix(oldRef, git.BranchPrefix)
	newRefFriendly := strings.TrimPrefix(issue.Ref, git.BranchPrefix)

	opts := &CreateCommentOptions{
		Type:   CommentTypeChangeIssueRef,
		Doer:   doer,
		Repo:   issue.Repo,
		Issue:  issue,
		OldRef: oldRefFriendly,
		NewRef: newRefFriendly,
	}
	if _, err = CreateComment(ctx, opts); err != nil {
		return fmt.Errorf("createComment: %w", err)
	}

	return committer.Commit()
}

// AddDeletePRBranchComment adds delete branch comment for pull request issue
func AddDeletePRBranchComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, issueID int64, branchName string) error {
	issue, err := GetIssueByID(ctx, issueID)
	if err != nil {
		return err
	}
	opts := &CreateCommentOptions{
		Type:   CommentTypeDeleteBranch,
		Doer:   doer,
		Repo:   repo,
		Issue:  issue,
		OldRef: branchName,
	}
	_, err = CreateComment(ctx, opts)
	return err
}

// UpdateIssueAttachments update attachments by UUIDs for the issue
func UpdateIssueAttachments(ctx context.Context, issueID int64, uuids []string) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()
	attachments, err := repo_model.GetAttachmentsByUUIDs(ctx, uuids)
	if err != nil {
		return fmt.Errorf("getAttachmentsByUUIDs [uuids: %v]: %w", uuids, err)
	}
	for i := 0; i < len(attachments); i++ {
		attachments[i].IssueID = issueID
		if err := repo_model.UpdateAttachment(ctx, attachments[i]); err != nil {
			return fmt.Errorf("update attachment [id: %d]: %w", attachments[i].ID, err)
		}
	}
	return committer.Commit()
}

// ChangeIssueContent changes issue content, as the given user.
func ChangeIssueContent(ctx context.Context, issue *Issue, doer *user_model.User, content string) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	hasContentHistory, err := HasIssueContentHistory(ctx, issue.ID, 0)
	if err != nil {
		return fmt.Errorf("HasIssueContentHistory: %w", err)
	}
	if !hasContentHistory {
		if err = SaveIssueContentHistory(ctx, issue.PosterID, issue.ID, 0,
			issue.CreatedUnix, issue.Content, true); err != nil {
			return fmt.Errorf("SaveIssueContentHistory: %w", err)
		}
	}

	issue.Content = content

	if err = UpdateIssueCols(ctx, issue, "content"); err != nil {
		return fmt.Errorf("UpdateIssueCols: %w", err)
	}

	if err = SaveIssueContentHistory(ctx, doer.ID, issue.ID, 0,
		timeutil.TimeStampNow(), issue.Content, false); err != nil {
		return fmt.Errorf("SaveIssueContentHistory: %w", err)
	}

	if err = issue.AddCrossReferences(ctx, doer, true); err != nil {
		return fmt.Errorf("addCrossReferences: %w", err)
	}

	return committer.Commit()
}

// NewIssueOptions represents the options of a new issue.
type NewIssueOptions struct {
	Repo        *repo_model.Repository
	Issue       *Issue
	LabelIDs    []int64
	Attachments []string // In UUID format.
	IsPull      bool
}

// NewIssueWithIndex creates issue with given index
func NewIssueWithIndex(ctx context.Context, doer *user_model.User, opts NewIssueOptions) (err error) {
	e := db.GetEngine(ctx)
	opts.Issue.Title = strings.TrimSpace(opts.Issue.Title)

	if opts.Issue.MilestoneID > 0 {
		milestone, err := GetMilestoneByRepoID(ctx, opts.Issue.RepoID, opts.Issue.MilestoneID)
		if err != nil && !IsErrMilestoneNotExist(err) {
			return fmt.Errorf("getMilestoneByID: %w", err)
		}

		// Assume milestone is invalid and drop silently.
		opts.Issue.MilestoneID = 0
		if milestone != nil {
			opts.Issue.MilestoneID = milestone.ID
			opts.Issue.Milestone = milestone
		}
	}

	if opts.Issue.Index <= 0 {
		return fmt.Errorf("no issue index provided")
	}
	if opts.Issue.ID > 0 {
		return fmt.Errorf("issue exist")
	}

	if _, err := e.Insert(opts.Issue); err != nil {
		return err
	}

	if opts.Issue.MilestoneID > 0 {
		if err := UpdateMilestoneCounters(ctx, opts.Issue.MilestoneID); err != nil {
			return err
		}

		opts := &CreateCommentOptions{
			Type:           CommentTypeMilestone,
			Doer:           doer,
			Repo:           opts.Repo,
			Issue:          opts.Issue,
			OldMilestoneID: 0,
			MilestoneID:    opts.Issue.MilestoneID,
		}
		if _, err = CreateComment(ctx, opts); err != nil {
			return err
		}
	}

	if err := repo_model.UpdateRepoIssueNumbers(ctx, opts.Issue.RepoID, opts.IsPull, false); err != nil {
		return err
	}

	if len(opts.LabelIDs) > 0 {
		// During the session, SQLite3 driver cannot handle retrieve objects after update something.
		// So we have to get all needed labels first.
		labels := make([]*Label, 0, len(opts.LabelIDs))
		if err = e.In("id", opts.LabelIDs).Find(&labels); err != nil {
			return fmt.Errorf("find all labels [label_ids: %v]: %w", opts.LabelIDs, err)
		}

		if err = opts.Issue.LoadPoster(ctx); err != nil {
			return err
		}

		for _, label := range labels {
			// Silently drop invalid labels.
			if label.RepoID != opts.Repo.ID && label.OrgID != opts.Repo.OwnerID {
				continue
			}

			if err = newIssueLabel(ctx, opts.Issue, label, opts.Issue.Poster); err != nil {
				return fmt.Errorf("addLabel [id: %d]: %w", label.ID, err)
			}
		}
	}

	if err = NewIssueUsers(ctx, opts.Repo, opts.Issue); err != nil {
		return err
	}

	if len(opts.Attachments) > 0 {
		attachments, err := repo_model.GetAttachmentsByUUIDs(ctx, opts.Attachments)
		if err != nil {
			return fmt.Errorf("getAttachmentsByUUIDs [uuids: %v]: %w", opts.Attachments, err)
		}

		for i := 0; i < len(attachments); i++ {
			attachments[i].IssueID = opts.Issue.ID
			if _, err = e.ID(attachments[i].ID).Update(attachments[i]); err != nil {
				return fmt.Errorf("update attachment [id: %d]: %w", attachments[i].ID, err)
			}
		}
	}
	if err = opts.Issue.LoadAttributes(ctx); err != nil {
		return err
	}

	return opts.Issue.AddCrossReferences(ctx, doer, false)
}

// NewIssue creates new issue with labels for repository.
func NewIssue(ctx context.Context, repo *repo_model.Repository, issue *Issue, labelIDs []int64, uuids []string) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	idx, err := db.GetNextResourceIndex(ctx, "issue_index", repo.ID)
	if err != nil {
		return fmt.Errorf("generate issue index failed: %w", err)
	}

	issue.Index = idx

	if err = NewIssueWithIndex(ctx, issue.Poster, NewIssueOptions{
		Repo:        repo,
		Issue:       issue,
		LabelIDs:    labelIDs,
		Attachments: uuids,
	}); err != nil {
		if repo_model.IsErrUserDoesNotHaveAccessToRepo(err) || IsErrNewIssueInsert(err) {
			return err
		}
		return fmt.Errorf("newIssue: %w", err)
	}

	if err = committer.Commit(); err != nil {
		return fmt.Errorf("Commit: %w", err)
	}

	return nil
}

// UpdateIssueMentions updates issue-user relations for mentioned users.
func UpdateIssueMentions(ctx context.Context, issueID int64, mentions []*user_model.User) error {
	if len(mentions) == 0 {
		return nil
	}
	ids := make([]int64, len(mentions))
	for i, u := range mentions {
		ids[i] = u.ID
	}
	if err := UpdateIssueUsersByMentions(ctx, issueID, ids); err != nil {
		return fmt.Errorf("UpdateIssueUsersByMentions: %w", err)
	}
	return nil
}

// UpdateIssueByAPI updates all allowed fields of given issue.
// If the issue status is changed a statusChangeComment is returned
// similarly if the title is changed the titleChanged bool is set to true
func UpdateIssueByAPI(ctx context.Context, issue *Issue, doer *user_model.User) (statusChangeComment *Comment, titleChanged bool, err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, false, err
	}
	defer committer.Close()

	if err := issue.LoadRepo(ctx); err != nil {
		return nil, false, fmt.Errorf("loadRepo: %w", err)
	}

	// Reload the issue
	currentIssue, err := GetIssueByID(ctx, issue.ID)
	if err != nil {
		return nil, false, err
	}

	if _, err := db.GetEngine(ctx).ID(issue.ID).Cols(
		"name", "content", "milestone_id", "priority",
		"deadline_unix", "updated_unix", "is_locked").
		Update(issue); err != nil {
		return nil, false, err
	}

	titleChanged = currentIssue.Title != issue.Title
	if titleChanged {
		opts := &CreateCommentOptions{
			Type:     CommentTypeChangeTitle,
			Doer:     doer,
			Repo:     issue.Repo,
			Issue:    issue,
			OldTitle: currentIssue.Title,
			NewTitle: issue.Title,
		}
		_, err := CreateComment(ctx, opts)
		if err != nil {
			return nil, false, fmt.Errorf("createComment: %w", err)
		}
	}

	if currentIssue.IsClosed != issue.IsClosed {
		statusChangeComment, err = doChangeIssueStatus(ctx, issue, doer, false)
		if err != nil {
			return nil, false, err
		}
	}

	if err := issue.AddCrossReferences(ctx, doer, true); err != nil {
		return nil, false, err
	}
	return statusChangeComment, titleChanged, committer.Commit()
}

// UpdateIssueDeadline updates an issue deadline and adds comments. Setting a deadline to 0 means deleting it.
func UpdateIssueDeadline(ctx context.Context, issue *Issue, deadlineUnix timeutil.TimeStamp, doer *user_model.User) (err error) {
	// if the deadline hasn't changed do nothing
	if issue.DeadlineUnix == deadlineUnix {
		return nil
	}
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	// Update the deadline
	if err = UpdateIssueCols(ctx, &Issue{ID: issue.ID, DeadlineUnix: deadlineUnix}, "deadline_unix"); err != nil {
		return err
	}

	// Make the comment
	if _, err = createDeadlineComment(ctx, doer, issue, deadlineUnix); err != nil {
		return fmt.Errorf("createRemovedDueDateComment: %w", err)
	}

	return committer.Commit()
}

// FindAndUpdateIssueMentions finds users mentioned in the given content string, and saves them in the database.
func FindAndUpdateIssueMentions(ctx context.Context, issue *Issue, doer *user_model.User, content string) (mentions []*user_model.User, err error) {
	rawMentions := references.FindAllMentionsMarkdown(content)
	mentions, err = ResolveIssueMentionsByVisibility(ctx, issue, doer, rawMentions)
	if err != nil {
		return nil, fmt.Errorf("UpdateIssueMentions [%d]: %w", issue.ID, err)
	}
	if err = UpdateIssueMentions(ctx, issue.ID, mentions); err != nil {
		return nil, fmt.Errorf("UpdateIssueMentions [%d]: %w", issue.ID, err)
	}
	return mentions, err
}

// ResolveIssueMentionsByVisibility returns the users mentioned in an issue, removing those that
// don't have access to reading it. Teams are expanded into their users, but organizations are ignored.
func ResolveIssueMentionsByVisibility(ctx context.Context, issue *Issue, doer *user_model.User, mentions []string) (users []*user_model.User, err error) {
	if len(mentions) == 0 {
		return nil, nil
	}
	if err = issue.LoadRepo(ctx); err != nil {
		return nil, err
	}

	resolved := make(map[string]bool, 10)
	var mentionTeams []string

	if err := issue.Repo.LoadOwner(ctx); err != nil {
		return nil, err
	}

	repoOwnerIsOrg := issue.Repo.Owner.IsOrganization()
	if repoOwnerIsOrg {
		mentionTeams = make([]string, 0, 5)
	}

	resolved[doer.LowerName] = true
	for _, name := range mentions {
		name := strings.ToLower(name)
		if _, ok := resolved[name]; ok {
			continue
		}
		if repoOwnerIsOrg && strings.Contains(name, "/") {
			names := strings.Split(name, "/")
			if len(names) < 2 || names[0] != issue.Repo.Owner.LowerName {
				continue
			}
			mentionTeams = append(mentionTeams, names[1])
			resolved[name] = true
		} else {
			resolved[name] = false
		}
	}

	if issue.Repo.Owner.IsOrganization() && len(mentionTeams) > 0 {
		teams := make([]*organization.Team, 0, len(mentionTeams))
		if err := db.GetEngine(ctx).
			Join("INNER", "team_repo", "team_repo.team_id = team.id").
			Where("team_repo.repo_id=?", issue.Repo.ID).
			In("team.lower_name", mentionTeams).
			Find(&teams); err != nil {
			return nil, fmt.Errorf("find mentioned teams: %w", err)
		}
		if len(teams) != 0 {
			checked := make([]int64, 0, len(teams))
			unittype := unit.TypeIssues
			if issue.IsPull {
				unittype = unit.TypePullRequests
			}
			for _, team := range teams {
				if team.AccessMode >= perm.AccessModeAdmin {
					checked = append(checked, team.ID)
					resolved[issue.Repo.Owner.LowerName+"/"+team.LowerName] = true
					continue
				}
				has, err := db.GetEngine(ctx).Get(&organization.TeamUnit{OrgID: issue.Repo.Owner.ID, TeamID: team.ID, Type: unittype})
				if err != nil {
					return nil, fmt.Errorf("get team units (%d): %w", team.ID, err)
				}
				if has {
					checked = append(checked, team.ID)
					resolved[issue.Repo.Owner.LowerName+"/"+team.LowerName] = true
				}
			}
			if len(checked) != 0 {
				teamusers := make([]*user_model.User, 0, 20)
				if err := db.GetEngine(ctx).
					Join("INNER", "team_user", "team_user.uid = `user`.id").
					In("`team_user`.team_id", checked).
					And("`user`.is_active = ?", true).
					And("`user`.prohibit_login = ?", false).
					Find(&teamusers); err != nil {
					return nil, fmt.Errorf("get teams users: %w", err)
				}
				if len(teamusers) > 0 {
					users = make([]*user_model.User, 0, len(teamusers))
					for _, user := range teamusers {
						if already, ok := resolved[user.LowerName]; !ok || !already {
							users = append(users, user)
							resolved[user.LowerName] = true
						}
					}
				}
			}
		}
	}

	// Remove names already in the list to avoid querying the database if pending names remain
	mentionUsers := make([]string, 0, len(resolved))
	for name, already := range resolved {
		if !already {
			mentionUsers = append(mentionUsers, name)
		}
	}
	if len(mentionUsers) == 0 {
		return users, err
	}

	if users == nil {
		users = make([]*user_model.User, 0, len(mentionUsers))
	}

	unchecked := make([]*user_model.User, 0, len(mentionUsers))
	if err := db.GetEngine(ctx).
		Where("`user`.is_active = ?", true).
		And("`user`.prohibit_login = ?", false).
		In("`user`.lower_name", mentionUsers).
		Find(&unchecked); err != nil {
		return nil, fmt.Errorf("find mentioned users: %w", err)
	}
	for _, user := range unchecked {
		if already := resolved[user.LowerName]; already || user.IsOrganization() {
			continue
		}
		// Normal users must have read access to the referencing issue
		perm, err := access_model.GetUserRepoPermission(ctx, issue.Repo, user)
		if err != nil {
			return nil, fmt.Errorf("GetUserRepoPermission [%d]: %w", user.ID, err)
		}
		if !perm.CanReadIssuesOrPulls(issue.IsPull) {
			continue
		}
		users = append(users, user)
	}

	return users, err
}

// UpdateIssuesMigrationsByType updates all migrated repositories' issues from gitServiceType to replace originalAuthorID to posterID
func UpdateIssuesMigrationsByType(ctx context.Context, gitServiceType api.GitServiceType, originalAuthorID string, posterID int64) error {
	_, err := db.GetEngine(ctx).Table("issue").
		Where("repo_id IN (SELECT id FROM repository WHERE original_service_type = ?)", gitServiceType).
		And("original_author_id = ?", originalAuthorID).
		Update(map[string]any{
			"poster_id":          posterID,
			"original_author":    "",
			"original_author_id": 0,
		})
	return err
}

// UpdateReactionsMigrationsByType updates all migrated repositories' reactions from gitServiceType to replace originalAuthorID to posterID
func UpdateReactionsMigrationsByType(ctx context.Context, gitServiceType api.GitServiceType, originalAuthorID string, userID int64) error {
	_, err := db.GetEngine(ctx).Table("reaction").
		Where("original_author_id = ?", originalAuthorID).
		And(migratedIssueCond(gitServiceType)).
		Update(map[string]any{
			"user_id":            userID,
			"original_author":    "",
			"original_author_id": 0,
		})
	return err
}

// DeleteIssuesByRepoID deletes issues by repositories id
func DeleteIssuesByRepoID(ctx context.Context, repoID int64) (attachmentPaths []string, err error) {
	// MariaDB has a performance bug: https://jira.mariadb.org/browse/MDEV-16289
	// so here it uses "DELETE ... WHERE IN" with pre-queried IDs.
	sess := db.GetEngine(ctx)

	for {
		issueIDs := make([]int64, 0, db.DefaultMaxInSize)

		err := sess.Table(&Issue{}).Where("repo_id = ?", repoID).OrderBy("id").Limit(db.DefaultMaxInSize).Cols("id").Find(&issueIDs)
		if err != nil {
			return nil, err
		}

		if len(issueIDs) == 0 {
			break
		}

		// Delete content histories
		_, err = sess.In("issue_id", issueIDs).Delete(&ContentHistory{})
		if err != nil {
			return nil, err
		}

		// Delete comments and attachments
		_, err = sess.In("issue_id", issueIDs).Delete(&Comment{})
		if err != nil {
			return nil, err
		}

		// Dependencies for issues in this repository
		_, err = sess.In("issue_id", issueIDs).Delete(&IssueDependency{})
		if err != nil {
			return nil, err
		}

		// Delete dependencies for issues in other repositories
		_, err = sess.In("dependency_id", issueIDs).Delete(&IssueDependency{})
		if err != nil {
			return nil, err
		}

		_, err = sess.In("issue_id", issueIDs).Delete(&IssueUser{})
		if err != nil {
			return nil, err
		}

		_, err = sess.In("issue_id", issueIDs).Delete(&Reaction{})
		if err != nil {
			return nil, err
		}

		_, err = sess.In("issue_id", issueIDs).Delete(&IssueWatch{})
		if err != nil {
			return nil, err
		}

		_, err = sess.In("issue_id", issueIDs).Delete(&Stopwatch{})
		if err != nil {
			return nil, err
		}

		_, err = sess.In("issue_id", issueIDs).Delete(&TrackedTime{})
		if err != nil {
			return nil, err
		}

		_, err = sess.In("issue_id", issueIDs).Delete(&project_model.ProjectIssue{})
		if err != nil {
			return nil, err
		}

		_, err = sess.In("dependent_issue_id", issueIDs).Delete(&Comment{})
		if err != nil {
			return nil, err
		}

		var attachments []*repo_model.Attachment
		err = sess.In("issue_id", issueIDs).Find(&attachments)
		if err != nil {
			return nil, err
		}

		for j := range attachments {
			attachmentPaths = append(attachmentPaths, attachments[j].RelativePath())
		}

		_, err = sess.In("issue_id", issueIDs).Delete(&repo_model.Attachment{})
		if err != nil {
			return nil, err
		}

		_, err = sess.In("id", issueIDs).Delete(&Issue{})
		if err != nil {
			return nil, err
		}
	}

	return attachmentPaths, err
}

// DeleteOrphanedIssues delete issues without a repo
func DeleteOrphanedIssues(ctx context.Context) error {
	var attachmentPaths []string
	err := db.WithTx(ctx, func(ctx context.Context) error {
		var ids []int64

		if err := db.GetEngine(ctx).Table("issue").Distinct("issue.repo_id").
			Join("LEFT", "repository", "issue.repo_id=repository.id").
			Where(builder.IsNull{"repository.id"}).GroupBy("issue.repo_id").
			Find(&ids); err != nil {
			return err
		}

		for i := range ids {
			paths, err := DeleteIssuesByRepoID(ctx, ids[i])
			if err != nil {
				return err
			}
			attachmentPaths = append(attachmentPaths, paths...)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Remove issue attachment files.
	for i := range attachmentPaths {
		system_model.RemoveAllWithNotice(ctx, "Delete issue attachment", attachmentPaths[i])
	}
	return nil
}
