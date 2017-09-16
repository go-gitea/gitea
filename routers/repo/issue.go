// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/Unknwon/com"
	"github.com/Unknwon/paginater"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markdown"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

const (
	tplIssues    base.TplName = "repo/issue/list"
	tplIssueNew  base.TplName = "repo/issue/new"
	tplIssueView base.TplName = "repo/issue/view"

	tplMilestone     base.TplName = "repo/issue/milestones"
	tplMilestoneNew  base.TplName = "repo/issue/milestone_new"
	tplMilestoneEdit base.TplName = "repo/issue/milestone_edit"

	issueTemplateKey = "IssueTemplate"
)

var (
	// ErrFileTypeForbidden not allowed file type error
	ErrFileTypeForbidden = errors.New("File type is not allowed")
	// ErrTooManyFiles upload too many files
	ErrTooManyFiles = errors.New("Maximum number of files to upload exceeded")
	// IssueTemplateCandidates issue templates
	IssueTemplateCandidates = []string{
		"ISSUE_TEMPLATE.md",
		"issue_template.md",
		".gitea/ISSUE_TEMPLATE.md",
		".gitea/issue_template.md",
		".github/ISSUE_TEMPLATE.md",
		".github/issue_template.md",
	}
)

// MustEnableIssues check if repository enable internal issues
func MustEnableIssues(ctx *context.Context) {
	if !ctx.Repo.Repository.UnitEnabled(models.UnitTypeIssues) &&
		!ctx.Repo.Repository.UnitEnabled(models.UnitTypeExternalTracker) {
		ctx.Handle(404, "MustEnableIssues", nil)
		return
	}

	unit, err := ctx.Repo.Repository.GetUnit(models.UnitTypeExternalTracker)
	if err == nil {
		ctx.Redirect(unit.ExternalTrackerConfig().ExternalTrackerURL)
		return
	}
}

// MustAllowPulls check if repository enable pull requests
func MustAllowPulls(ctx *context.Context) {
	if !ctx.Repo.Repository.AllowsPulls() {
		ctx.Handle(404, "MustAllowPulls", nil)
		return
	}

	// User can send pull request if owns a forked repository.
	if ctx.IsSigned && ctx.User.HasForkedRepo(ctx.Repo.Repository.ID) {
		ctx.Repo.PullRequest.Allowed = true
		ctx.Repo.PullRequest.HeadInfo = ctx.User.Name + ":" + ctx.Repo.BranchName
	}
}

// Issues render issues page
func Issues(ctx *context.Context) {
	isPullList := ctx.Params(":type") == "pulls"
	if isPullList {
		MustAllowPulls(ctx)
		if ctx.Written() {
			return
		}
		ctx.Data["Title"] = ctx.Tr("repo.pulls")
		ctx.Data["PageIsPullList"] = true

	} else {
		MustEnableIssues(ctx)
		if ctx.Written() {
			return
		}
		ctx.Data["Title"] = ctx.Tr("repo.issues")
		ctx.Data["PageIsIssueList"] = true
	}

	viewType := ctx.Query("type")
	sortType := ctx.Query("sort")
	types := []string{"all", "assigned", "created_by", "mentioned"}
	if !com.IsSliceContainsStr(types, viewType) {
		viewType = "all"
	}

	var (
		assigneeID  = ctx.QueryInt64("assignee")
		posterID    int64
		mentionedID int64
		forceEmpty  bool
	)

	if ctx.IsSigned {
		switch viewType {
		case "created_by":
			posterID = ctx.User.ID
		case "mentioned":
			mentionedID = ctx.User.ID
		}
	}

	repo := ctx.Repo.Repository
	selectLabels := ctx.Query("labels")
	milestoneID := ctx.QueryInt64("milestone")
	isShowClosed := ctx.Query("state") == "closed"

	keyword := strings.Trim(ctx.Query("q"), " ")
	if bytes.Contains([]byte(keyword), []byte{0x00}) {
		keyword = ""
	}

	var issueIDs []int64
	var err error
	if len(keyword) > 0 {
		issueIDs, err = models.SearchIssuesByKeyword(repo.ID, keyword)
		if len(issueIDs) == 0 {
			forceEmpty = true
		}
	}

	var issueStats *models.IssueStats
	if forceEmpty {
		issueStats = &models.IssueStats{}
	} else {
		var err error
		issueStats, err = models.GetIssueStats(&models.IssueStatsOptions{
			RepoID:      repo.ID,
			Labels:      selectLabels,
			MilestoneID: milestoneID,
			AssigneeID:  assigneeID,
			MentionedID: mentionedID,
			PosterID:    posterID,
			IsPull:      isPullList,
			IssueIDs:    issueIDs,
		})
		if err != nil {
			ctx.Handle(500, "GetIssueStats", err)
			return
		}
	}
	page := ctx.QueryInt("page")
	if page <= 1 {
		page = 1
	}

	var total int
	if !isShowClosed {
		total = int(issueStats.OpenCount)
	} else {
		total = int(issueStats.ClosedCount)
	}
	pager := paginater.New(total, setting.UI.IssuePagingNum, page, 5)
	ctx.Data["Page"] = pager

	var issues []*models.Issue
	if forceEmpty {
		issues = []*models.Issue{}
	} else {
		issues, err = models.Issues(&models.IssuesOptions{
			AssigneeID:  assigneeID,
			RepoID:      repo.ID,
			PosterID:    posterID,
			MentionedID: mentionedID,
			MilestoneID: milestoneID,
			Page:        pager.Current(),
			PageSize:    setting.UI.IssuePagingNum,
			IsClosed:    util.OptionalBoolOf(isShowClosed),
			IsPull:      util.OptionalBoolOf(isPullList),
			Labels:      selectLabels,
			SortType:    sortType,
			IssueIDs:    issueIDs,
		})
		if err != nil {
			ctx.Handle(500, "Issues", err)
			return
		}
	}

	// Get posters.
	for i := range issues {
		// Check read status
		if !ctx.IsSigned {
			issues[i].IsRead = true
		} else if err = issues[i].GetIsRead(ctx.User.ID); err != nil {
			ctx.Handle(500, "GetIsRead", err)
			return
		}
	}
	ctx.Data["Issues"] = issues

	// Get milestones.
	ctx.Data["Milestones"], err = models.GetMilestonesByRepoID(repo.ID)
	if err != nil {
		ctx.Handle(500, "GetAllRepoMilestones", err)
		return
	}

	// Get assignees.
	ctx.Data["Assignees"], err = repo.GetAssignees()
	if err != nil {
		ctx.Handle(500, "GetAssignees", err)
		return
	}

	if ctx.QueryInt64("assignee") == 0 {
		assigneeID = 0 // Reset ID to prevent unexpected selection of assignee.
	}

	ctx.Data["IssueStats"] = issueStats
	ctx.Data["SelectLabels"] = com.StrTo(selectLabels).MustInt64()
	ctx.Data["ViewType"] = viewType
	ctx.Data["SortType"] = sortType
	ctx.Data["MilestoneID"] = milestoneID
	ctx.Data["AssigneeID"] = assigneeID
	ctx.Data["IsShowClosed"] = isShowClosed
	ctx.Data["Keyword"] = keyword
	if isShowClosed {
		ctx.Data["State"] = "closed"
	} else {
		ctx.Data["State"] = "open"
	}

	ctx.HTML(200, tplIssues)
}

// RetrieveRepoMilestonesAndAssignees find all the milestones and assignees of a repository
func RetrieveRepoMilestonesAndAssignees(ctx *context.Context, repo *models.Repository) {
	var err error
	ctx.Data["OpenMilestones"], err = models.GetMilestones(repo.ID, -1, false, "")
	if err != nil {
		ctx.Handle(500, "GetMilestones", err)
		return
	}
	ctx.Data["ClosedMilestones"], err = models.GetMilestones(repo.ID, -1, true, "")
	if err != nil {
		ctx.Handle(500, "GetMilestones", err)
		return
	}

	ctx.Data["Assignees"], err = repo.GetAssignees()
	if err != nil {
		ctx.Handle(500, "GetAssignees", err)
		return
	}
}

// RetrieveRepoMetas find all the meta information of a repository
func RetrieveRepoMetas(ctx *context.Context, repo *models.Repository) []*models.Label {
	if !ctx.Repo.IsWriter() {
		return nil
	}

	labels, err := models.GetLabelsByRepoID(repo.ID, "")
	if err != nil {
		ctx.Handle(500, "GetLabelsByRepoID", err)
		return nil
	}
	ctx.Data["Labels"] = labels

	RetrieveRepoMilestonesAndAssignees(ctx, repo)
	if ctx.Written() {
		return nil
	}

	brs, err := ctx.Repo.GitRepo.GetBranches()
	if err != nil {
		ctx.Handle(500, "GetBranches", err)
		return nil
	}
	ctx.Data["Branches"] = brs

	return labels
}

func getFileContentFromDefaultBranch(ctx *context.Context, filename string) (string, bool) {
	var r io.Reader
	var bytes []byte

	if ctx.Repo.Commit == nil {
		var err error
		ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
		if err != nil {
			return "", false
		}
	}

	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(filename)
	if err != nil {
		return "", false
	}
	r, err = entry.Blob().Data()
	if err != nil {
		return "", false
	}
	bytes, err = ioutil.ReadAll(r)
	if err != nil {
		return "", false
	}
	return string(bytes), true
}

func setTemplateIfExists(ctx *context.Context, ctxDataKey string, possibleFiles []string) {
	for _, filename := range possibleFiles {
		content, found := getFileContentFromDefaultBranch(ctx, filename)
		if found {
			ctx.Data[ctxDataKey] = content
			return
		}
	}
}

// NewIssue render createing issue page
func NewIssue(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.issues.new")
	ctx.Data["PageIsIssueList"] = true
	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["RequireSimpleMDE"] = true
	setTemplateIfExists(ctx, issueTemplateKey, IssueTemplateCandidates)
	renderAttachmentSettings(ctx)

	RetrieveRepoMetas(ctx, ctx.Repo.Repository)
	if ctx.Written() {
		return
	}

	ctx.HTML(200, tplIssueNew)
}

// ValidateRepoMetas check and returns repository's meta informations
func ValidateRepoMetas(ctx *context.Context, form auth.CreateIssueForm) ([]int64, int64, int64) {
	var (
		repo = ctx.Repo.Repository
		err  error
	)

	labels := RetrieveRepoMetas(ctx, ctx.Repo.Repository)
	if ctx.Written() {
		return nil, 0, 0
	}

	if !ctx.Repo.IsWriter() {
		return nil, 0, 0
	}

	var labelIDs []int64
	hasSelected := false
	// Check labels.
	if len(form.LabelIDs) > 0 {
		labelIDs, err = base.StringsToInt64s(strings.Split(form.LabelIDs, ","))
		if err != nil {
			return nil, 0, 0
		}
		labelIDMark := base.Int64sToMap(labelIDs)

		for i := range labels {
			if labelIDMark[labels[i].ID] {
				labels[i].IsChecked = true
				hasSelected = true
			}
		}
	}

	ctx.Data["Labels"] = labels
	ctx.Data["HasSelectedLabel"] = hasSelected
	ctx.Data["label_ids"] = form.LabelIDs

	// Check milestone.
	milestoneID := form.MilestoneID
	if milestoneID > 0 {
		ctx.Data["Milestone"], err = repo.GetMilestoneByID(milestoneID)
		if err != nil {
			ctx.Handle(500, "GetMilestoneByID", err)
			return nil, 0, 0
		}
		ctx.Data["milestone_id"] = milestoneID
	}

	// Check assignee.
	assigneeID := form.AssigneeID
	if assigneeID > 0 {
		ctx.Data["Assignee"], err = repo.GetAssigneeByID(assigneeID)
		if err != nil {
			ctx.Handle(500, "GetAssigneeByID", err)
			return nil, 0, 0
		}
		ctx.Data["assignee_id"] = assigneeID
	}

	return labelIDs, milestoneID, assigneeID
}

// NewIssuePost response for creating new issue
func NewIssuePost(ctx *context.Context, form auth.CreateIssueForm) {
	ctx.Data["Title"] = ctx.Tr("repo.issues.new")
	ctx.Data["PageIsIssueList"] = true
	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["RequireSimpleMDE"] = true
	ctx.Data["ReadOnly"] = false
	renderAttachmentSettings(ctx)

	var (
		repo        = ctx.Repo.Repository
		attachments []string
	)

	labelIDs, milestoneID, assigneeID := ValidateRepoMetas(ctx, form)
	if ctx.Written() {
		return
	}

	if setting.AttachmentEnabled {
		attachments = form.Files
	}

	if ctx.HasError() {
		ctx.HTML(200, tplIssueNew)
		return
	}

	issue := &models.Issue{
		RepoID:      repo.ID,
		Title:       form.Title,
		PosterID:    ctx.User.ID,
		Poster:      ctx.User,
		MilestoneID: milestoneID,
		AssigneeID:  assigneeID,
		Content:     form.Content,
		Ref:         form.Ref,
	}
	if err := models.NewIssue(repo, issue, labelIDs, attachments); err != nil {
		ctx.Handle(500, "NewIssue", err)
		return
	}

	notification.Service.NotifyIssue(issue, ctx.User.ID)

	log.Trace("Issue created: %d/%d", repo.ID, issue.ID)
	ctx.Redirect(ctx.Repo.RepoLink + "/issues/" + com.ToStr(issue.Index))
}

// ViewIssue render issue view page
func ViewIssue(ctx *context.Context) {
	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["RequireDropzone"] = true
	renderAttachmentSettings(ctx)

	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.Handle(404, "GetIssueByIndex", err)
		} else {
			ctx.Handle(500, "GetIssueByIndex", err)
		}
		return
	}
	ctx.Data["Title"] = fmt.Sprintf("#%d - %s", issue.Index, issue.Title)

	var iw *models.IssueWatch
	var exists bool
	if ctx.User != nil {
		iw, exists, err = models.GetIssueWatch(ctx.User.ID, issue.ID)
		if err != nil {
			ctx.Handle(500, "GetIssueWatch", err)
			return
		}
		if !exists {
			iw = &models.IssueWatch{
				UserID:     ctx.User.ID,
				IssueID:    issue.ID,
				IsWatching: models.IsWatching(ctx.User.ID, ctx.Repo.Repository.ID),
			}
		}
	}
	ctx.Data["IssueWatch"] = iw

	// Make sure type and URL matches.
	if ctx.Params(":type") == "issues" && issue.IsPull {
		ctx.Redirect(ctx.Repo.RepoLink + "/pulls/" + com.ToStr(issue.Index))
		return
	} else if ctx.Params(":type") == "pulls" && !issue.IsPull {
		ctx.Redirect(ctx.Repo.RepoLink + "/issues/" + com.ToStr(issue.Index))
		return
	}

	if issue.IsPull {
		MustAllowPulls(ctx)
		if ctx.Written() {
			return
		}
		ctx.Data["PageIsPullList"] = true
		ctx.Data["PageIsPullConversation"] = true
	} else {
		MustEnableIssues(ctx)
		if ctx.Written() {
			return
		}
		ctx.Data["PageIsIssueList"] = true
	}

	issue.RenderedContent = string(markdown.Render([]byte(issue.Content), ctx.Repo.RepoLink,
		ctx.Repo.Repository.ComposeMetas()))

	repo := ctx.Repo.Repository

	// Get more information if it's a pull request.
	if issue.IsPull {
		if issue.PullRequest.HasMerged {
			ctx.Data["DisableStatusChange"] = issue.PullRequest.HasMerged
			PrepareMergedViewPullInfo(ctx, issue)
		} else {
			PrepareViewPullInfo(ctx, issue)
		}
		if ctx.Written() {
			return
		}
	}

	// Metas.
	// Check labels.
	labelIDMark := make(map[int64]bool)
	for i := range issue.Labels {
		labelIDMark[issue.Labels[i].ID] = true
	}
	labels, err := models.GetLabelsByRepoID(repo.ID, "")
	if err != nil {
		ctx.Handle(500, "GetLabelsByRepoID", err)
		return
	}
	hasSelected := false
	for i := range labels {
		if labelIDMark[labels[i].ID] {
			labels[i].IsChecked = true
			hasSelected = true
		}
	}
	ctx.Data["HasSelectedLabel"] = hasSelected
	ctx.Data["Labels"] = labels

	// Check milestone and assignee.
	if ctx.Repo.IsWriter() {
		RetrieveRepoMilestonesAndAssignees(ctx, repo)
		if ctx.Written() {
			return
		}
	}

	if ctx.IsSigned {
		// Update issue-user.
		if err = issue.ReadBy(ctx.User.ID); err != nil {
			ctx.Handle(500, "ReadBy", err)
			return
		}
	}

	var (
		tag          models.CommentTag
		ok           bool
		marked       = make(map[int64]models.CommentTag)
		comment      *models.Comment
		participants = make([]*models.User, 1, 10)
	)
	if ctx.Repo.Repository.IsTimetrackerEnabled() {
		if ctx.IsSigned {
			// Deal with the stopwatch
			ctx.Data["IsStopwatchRunning"] = models.StopwatchExists(ctx.User.ID, issue.ID)
			if !ctx.Data["IsStopwatchRunning"].(bool) {
				var exists bool
				var sw *models.Stopwatch
				if exists, sw, err = models.HasUserStopwatch(ctx.User.ID); err != nil {
					ctx.Handle(500, "HasUserStopwatch", err)
					return
				}
				ctx.Data["HasUserStopwatch"] = exists
				if exists {
					// Add warning if the user has already a stopwatch
					var otherIssue *models.Issue
					if otherIssue, err = models.GetIssueByID(sw.IssueID); err != nil {
						ctx.Handle(500, "GetIssueByID", err)
						return
					}
					// Add link to the issue of the already running stopwatch
					ctx.Data["OtherStopwatchURL"] = otherIssue.HTMLURL()
				}
			}
			ctx.Data["CanUseTimetracker"] = ctx.Repo.CanUseTimetracker(issue, ctx.User)
		} else {
			ctx.Data["CanUseTimetracker"] = false
		}
		if ctx.Data["WorkingUsers"], err = models.TotalTimes(models.FindTrackedTimesOptions{IssueID: issue.ID}); err != nil {
			ctx.Handle(500, "TotalTimes", err)
			return
		}
	}

	// Render comments and and fetch participants.
	participants[0] = issue.Poster
	for _, comment = range issue.Comments {
		if comment.Type == models.CommentTypeComment {
			comment.RenderedContent = string(markdown.Render([]byte(comment.Content), ctx.Repo.RepoLink,
				ctx.Repo.Repository.ComposeMetas()))

			// Check tag.
			tag, ok = marked[comment.PosterID]
			if ok {
				comment.ShowTag = tag
				continue
			}

			if repo.IsOwnedBy(comment.PosterID) ||
				(repo.Owner.IsOrganization() && repo.Owner.IsOwnedBy(comment.PosterID)) {
				comment.ShowTag = models.CommentTagOwner
			} else if comment.Poster.IsWriterOfRepo(repo) {
				comment.ShowTag = models.CommentTagWriter
			} else if comment.PosterID == issue.PosterID {
				comment.ShowTag = models.CommentTagPoster
			}

			marked[comment.PosterID] = comment.ShowTag

			isAdded := false
			for j := range participants {
				if comment.Poster == participants[j] {
					isAdded = true
					break
				}
			}
			if !isAdded && !issue.IsPoster(comment.Poster.ID) {
				participants = append(participants, comment.Poster)
			}
		} else if comment.Type == models.CommentTypeLabel {
			if err = comment.LoadLabel(); err != nil {
				ctx.Handle(500, "LoadLabel", err)
				return
			}
		} else if comment.Type == models.CommentTypeMilestone {
			if err = comment.LoadMilestone(); err != nil {
				ctx.Handle(500, "LoadMilestone", err)
				return
			}
			ghostMilestone := &models.Milestone{
				ID:   -1,
				Name: ctx.Tr("repo.issues.deleted_milestone"),
			}
			if comment.OldMilestoneID > 0 && comment.OldMilestone == nil {
				comment.OldMilestone = ghostMilestone
			}
			if comment.MilestoneID > 0 && comment.Milestone == nil {
				comment.Milestone = ghostMilestone
			}
		} else if comment.Type == models.CommentTypeAssignees {
			if err = comment.LoadAssignees(); err != nil {
				ctx.Handle(500, "LoadAssignees", err)
				return
			}
		}
	}

	if issue.IsPull {
		pull := issue.PullRequest
		canDelete := false

		if ctx.IsSigned {
			if err := pull.GetHeadRepo(); err != nil {
				log.Error(4, "GetHeadRepo: %v", err)
			} else if pull.HeadRepo != nil && pull.HeadBranch != pull.HeadRepo.DefaultBranch && ctx.User.IsWriterOfRepo(pull.HeadRepo) {
				// Check if branch is not protected
				if protected, err := pull.HeadRepo.IsProtectedBranch(pull.HeadBranch, ctx.User); err != nil {
					log.Error(4, "IsProtectedBranch: %v", err)
				} else if !protected {
					canDelete = true
					ctx.Data["DeleteBranchLink"] = ctx.Repo.RepoLink + "/pulls/" + com.ToStr(issue.Index) + "/cleanup"
				}
			}
		}

		ctx.Data["IsPullBranchDeletable"] = canDelete && pull.HeadRepo != nil && git.IsBranchExist(pull.HeadRepo.RepoPath(), pull.HeadBranch)
	}

	ctx.Data["Participants"] = participants
	ctx.Data["NumParticipants"] = len(participants)
	ctx.Data["Issue"] = issue
	ctx.Data["ReadOnly"] = true
	ctx.Data["IsIssueOwner"] = ctx.Repo.IsWriter() || (ctx.IsSigned && issue.IsPoster(ctx.User.ID))
	ctx.Data["SignInLink"] = setting.AppSubURL + "/user/login?redirect_to=" + ctx.Data["Link"].(string)
	ctx.HTML(200, tplIssueView)
}

// GetActionIssue will return the issue which is used in the context.
func GetActionIssue(ctx *context.Context) *models.Issue {
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.Error(404, "GetIssueByIndex")
		} else {
			ctx.Handle(500, "GetIssueByIndex", err)
		}
		return nil
	}
	return issue
}

func getActionIssues(ctx *context.Context) []*models.Issue {
	commaSeparatedIssueIDs := ctx.Query("issue_ids")
	if len(commaSeparatedIssueIDs) == 0 {
		return nil
	}
	issueIDs := make([]int64, 0, 10)
	for _, stringIssueID := range strings.Split(commaSeparatedIssueIDs, ",") {
		issueID, err := strconv.ParseInt(stringIssueID, 10, 64)
		if err != nil {
			ctx.Handle(500, "ParseInt", err)
			return nil
		}
		issueIDs = append(issueIDs, issueID)
	}
	issues, err := models.GetIssuesByIDs(issueIDs)
	if err != nil {
		ctx.Handle(500, "GetIssuesByIDs", err)
		return nil
	}
	return issues
}

// UpdateIssueTitle change issue's title
func UpdateIssueTitle(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (!issue.IsPoster(ctx.User.ID) && !ctx.Repo.IsWriter()) {
		ctx.Error(403)
		return
	}

	title := ctx.QueryTrim("title")
	if len(title) == 0 {
		ctx.Error(204)
		return
	}

	if err := issue.ChangeTitle(ctx.User, title); err != nil {
		ctx.Handle(500, "ChangeTitle", err)
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"title": issue.Title,
	})
}

// UpdateIssueContent change issue's content
func UpdateIssueContent(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (ctx.User.ID != issue.PosterID && !ctx.Repo.IsWriter()) {
		ctx.Error(403)
		return
	}

	content := ctx.Query("content")
	if err := issue.ChangeContent(ctx.User, content); err != nil {
		ctx.Handle(500, "ChangeContent", err)
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"content": string(markdown.Render([]byte(issue.Content), ctx.Query("context"), ctx.Repo.Repository.ComposeMetas())),
	})
}

// UpdateIssueMilestone change issue's milestone
func UpdateIssueMilestone(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	milestoneID := ctx.QueryInt64("id")
	for _, issue := range issues {
		oldMilestoneID := issue.MilestoneID
		if oldMilestoneID == milestoneID {
			continue
		}
		issue.MilestoneID = milestoneID
		if err := models.ChangeMilestoneAssign(issue, ctx.User, oldMilestoneID); err != nil {
			ctx.Handle(500, "ChangeMilestoneAssign", err)
			return
		}
	}

	ctx.JSON(200, map[string]interface{}{
		"ok": true,
	})
}

// UpdateIssueAssignee change issue's assignee
func UpdateIssueAssignee(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	assigneeID := ctx.QueryInt64("id")
	for _, issue := range issues {
		if issue.AssigneeID == assigneeID {
			continue
		}
		if err := issue.ChangeAssignee(ctx.User, assigneeID); err != nil {
			ctx.Handle(500, "ChangeAssignee", err)
			return
		}
	}
	ctx.JSON(200, map[string]interface{}{
		"ok": true,
	})
}

// UpdateIssueStatus change issue's status
func UpdateIssueStatus(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	var isClosed bool
	switch action := ctx.Query("action"); action {
	case "open":
		isClosed = false
	case "close":
		isClosed = true
	default:
		log.Warn("Unrecognized action: %s", action)
	}

	if _, err := models.IssueList(issues).LoadRepositories(); err != nil {
		ctx.Handle(500, "LoadRepositories", err)
		return
	}
	for _, issue := range issues {
		if err := issue.ChangeStatus(ctx.User, issue.Repo, isClosed); err != nil {
			ctx.Handle(500, "ChangeStatus", err)
			return
		}
	}
	ctx.JSON(200, map[string]interface{}{
		"ok": true,
	})
}

// NewComment create a comment for issue
func NewComment(ctx *context.Context, form auth.CreateCommentForm) {
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		ctx.NotFoundOrServerError("GetIssueByIndex", models.IsErrIssueNotExist, err)
		return
	}

	var attachments []string
	if setting.AttachmentEnabled {
		attachments = form.Files
	}

	if ctx.HasError() {
		ctx.Flash.Error(ctx.Data["ErrorMsg"].(string))
		ctx.Redirect(fmt.Sprintf("%s/issues/%d", ctx.Repo.RepoLink, issue.Index))
		return
	}

	var comment *models.Comment
	defer func() {
		// Check if issue admin/poster changes the status of issue.
		if (ctx.Repo.IsWriter() || (ctx.IsSigned && issue.IsPoster(ctx.User.ID))) &&
			(form.Status == "reopen" || form.Status == "close") &&
			!(issue.IsPull && issue.PullRequest.HasMerged) {

			// Duplication and conflict check should apply to reopen pull request.
			var pr *models.PullRequest

			if form.Status == "reopen" && issue.IsPull {
				pull := issue.PullRequest
				pr, err = models.GetUnmergedPullRequest(pull.HeadRepoID, pull.BaseRepoID, pull.HeadBranch, pull.BaseBranch)
				if err != nil {
					if !models.IsErrPullRequestNotExist(err) {
						ctx.Handle(500, "GetUnmergedPullRequest", err)
						return
					}
				}

				// Regenerate patch and test conflict.
				if pr == nil {
					if err = issue.PullRequest.UpdatePatch(); err != nil {
						ctx.Handle(500, "UpdatePatch", err)
						return
					}

					issue.PullRequest.AddToTaskQueue()
				}
			}

			if pr != nil {
				ctx.Flash.Info(ctx.Tr("repo.pulls.open_unmerged_pull_exists", pr.Index))
			} else {
				if err = issue.ChangeStatus(ctx.User, ctx.Repo.Repository, form.Status == "close"); err != nil {
					log.Error(4, "ChangeStatus: %v", err)
				} else {
					log.Trace("Issue [%d] status changed to closed: %v", issue.ID, issue.IsClosed)

					notification.Service.NotifyIssue(issue, ctx.User.ID)
				}
			}
		}

		// Redirect to comment hashtag if there is any actual content.
		typeName := "issues"
		if issue.IsPull {
			typeName = "pulls"
		}
		if comment != nil {
			ctx.Redirect(fmt.Sprintf("%s/%s/%d#%s", ctx.Repo.RepoLink, typeName, issue.Index, comment.HashTag()))
		} else {
			ctx.Redirect(fmt.Sprintf("%s/%s/%d", ctx.Repo.RepoLink, typeName, issue.Index))
		}
	}()

	// Fix #321: Allow empty comments, as long as we have attachments.
	if len(form.Content) == 0 && len(attachments) == 0 {
		return
	}

	comment, err = models.CreateIssueComment(ctx.User, ctx.Repo.Repository, issue, form.Content, attachments)
	if err != nil {
		ctx.Handle(500, "CreateIssueComment", err)
		return
	}

	notification.Service.NotifyIssue(issue, ctx.User.ID)

	log.Trace("Comment created: %d/%d/%d", ctx.Repo.Repository.ID, issue.ID, comment.ID)
}

// UpdateCommentContent change comment of issue's content
func UpdateCommentContent(ctx *context.Context) {
	comment, err := models.GetCommentByID(ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommentByID", models.IsErrCommentNotExist, err)
		return
	}

	if !ctx.IsSigned || (ctx.User.ID != comment.PosterID && !ctx.Repo.IsAdmin()) {
		ctx.Error(403)
		return
	} else if comment.Type != models.CommentTypeComment {
		ctx.Error(204)
		return
	}

	comment.Content = ctx.Query("content")
	if len(comment.Content) == 0 {
		ctx.JSON(200, map[string]interface{}{
			"content": "",
		})
		return
	}
	if err = models.UpdateComment(comment); err != nil {
		ctx.Handle(500, "UpdateComment", err)
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"content": string(markdown.Render([]byte(comment.Content), ctx.Query("context"), ctx.Repo.Repository.ComposeMetas())),
	})
}

// DeleteComment delete comment of issue
func DeleteComment(ctx *context.Context) {
	comment, err := models.GetCommentByID(ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommentByID", models.IsErrCommentNotExist, err)
		return
	}

	if !ctx.IsSigned || (ctx.User.ID != comment.PosterID && !ctx.Repo.IsAdmin()) {
		ctx.Error(403)
		return
	} else if comment.Type != models.CommentTypeComment {
		ctx.Error(204)
		return
	}

	if err = models.DeleteComment(comment); err != nil {
		ctx.Handle(500, "DeleteCommentByID", err)
		return
	}

	ctx.Status(200)
}

// Milestones render milestones page
func Milestones(ctx *context.Context) {
	MustEnableIssues(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Title"] = ctx.Tr("repo.milestones")
	ctx.Data["PageIsIssueList"] = true
	ctx.Data["PageIsMilestones"] = true

	isShowClosed := ctx.Query("state") == "closed"
	openCount, closedCount := models.MilestoneStats(ctx.Repo.Repository.ID)
	ctx.Data["OpenCount"] = openCount
	ctx.Data["ClosedCount"] = closedCount

	sortType := ctx.Query("sort")
	page := ctx.QueryInt("page")
	if page <= 1 {
		page = 1
	}

	var total int
	if !isShowClosed {
		total = int(openCount)
	} else {
		total = int(closedCount)
	}
	ctx.Data["Page"] = paginater.New(total, setting.UI.IssuePagingNum, page, 5)

	miles, err := models.GetMilestones(ctx.Repo.Repository.ID, page, isShowClosed, sortType)
	if err != nil {
		ctx.Handle(500, "GetMilestones", err)
		return
	}
	for _, m := range miles {
		m.RenderedContent = string(markdown.Render([]byte(m.Content), ctx.Repo.RepoLink, ctx.Repo.Repository.ComposeMetas()))
	}
	ctx.Data["Milestones"] = miles

	if isShowClosed {
		ctx.Data["State"] = "closed"
	} else {
		ctx.Data["State"] = "open"
	}

	ctx.Data["SortType"] = sortType
	ctx.Data["IsShowClosed"] = isShowClosed
	ctx.HTML(200, tplMilestone)
}

// NewMilestone render creating milestone page
func NewMilestone(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.milestones.new")
	ctx.Data["PageIsIssueList"] = true
	ctx.Data["PageIsMilestones"] = true
	ctx.Data["RequireDatetimepicker"] = true
	ctx.Data["DateLang"] = setting.DateLang(ctx.Locale.Language())
	ctx.HTML(200, tplMilestoneNew)
}

// NewMilestonePost response for creating milestone
func NewMilestonePost(ctx *context.Context, form auth.CreateMilestoneForm) {
	ctx.Data["Title"] = ctx.Tr("repo.milestones.new")
	ctx.Data["PageIsIssueList"] = true
	ctx.Data["PageIsMilestones"] = true
	ctx.Data["RequireDatetimepicker"] = true
	ctx.Data["DateLang"] = setting.DateLang(ctx.Locale.Language())

	if ctx.HasError() {
		ctx.HTML(200, tplMilestoneNew)
		return
	}

	if len(form.Deadline) == 0 {
		form.Deadline = "9999-12-31"
	}
	deadline, err := time.ParseInLocation("2006-01-02", form.Deadline, time.Local)
	if err != nil {
		ctx.Data["Err_Deadline"] = true
		ctx.RenderWithErr(ctx.Tr("repo.milestones.invalid_due_date_format"), tplMilestoneNew, &form)
		return
	}

	if err = models.NewMilestone(&models.Milestone{
		RepoID:   ctx.Repo.Repository.ID,
		Name:     form.Title,
		Content:  form.Content,
		Deadline: deadline,
	}); err != nil {
		ctx.Handle(500, "NewMilestone", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.milestones.create_success", form.Title))
	ctx.Redirect(ctx.Repo.RepoLink + "/milestones")
}

// EditMilestone render edting milestone page
func EditMilestone(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.milestones.edit")
	ctx.Data["PageIsMilestones"] = true
	ctx.Data["PageIsEditMilestone"] = true
	ctx.Data["RequireDatetimepicker"] = true
	ctx.Data["DateLang"] = setting.DateLang(ctx.Locale.Language())

	m, err := models.GetMilestoneByRepoID(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrMilestoneNotExist(err) {
			ctx.Handle(404, "", nil)
		} else {
			ctx.Handle(500, "GetMilestoneByRepoID", err)
		}
		return
	}
	ctx.Data["title"] = m.Name
	ctx.Data["content"] = m.Content
	if len(m.DeadlineString) > 0 {
		ctx.Data["deadline"] = m.DeadlineString
	}
	ctx.HTML(200, tplMilestoneNew)
}

// EditMilestonePost response for edting milestone
func EditMilestonePost(ctx *context.Context, form auth.CreateMilestoneForm) {
	ctx.Data["Title"] = ctx.Tr("repo.milestones.edit")
	ctx.Data["PageIsMilestones"] = true
	ctx.Data["PageIsEditMilestone"] = true
	ctx.Data["RequireDatetimepicker"] = true
	ctx.Data["DateLang"] = setting.DateLang(ctx.Locale.Language())

	if ctx.HasError() {
		ctx.HTML(200, tplMilestoneNew)
		return
	}

	if len(form.Deadline) == 0 {
		form.Deadline = "9999-12-31"
	}
	deadline, err := time.ParseInLocation("2006-01-02", form.Deadline, time.Local)
	if err != nil {
		ctx.Data["Err_Deadline"] = true
		ctx.RenderWithErr(ctx.Tr("repo.milestones.invalid_due_date_format"), tplMilestoneNew, &form)
		return
	}

	m, err := models.GetMilestoneByRepoID(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrMilestoneNotExist(err) {
			ctx.Handle(404, "", nil)
		} else {
			ctx.Handle(500, "GetMilestoneByRepoID", err)
		}
		return
	}
	m.Name = form.Title
	m.Content = form.Content
	m.Deadline = deadline
	if err = models.UpdateMilestone(m); err != nil {
		ctx.Handle(500, "UpdateMilestone", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.milestones.edit_success", m.Name))
	ctx.Redirect(ctx.Repo.RepoLink + "/milestones")
}

// ChangeMilestonStatus response for change a milestone's status
func ChangeMilestonStatus(ctx *context.Context) {
	m, err := models.GetMilestoneByRepoID(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrMilestoneNotExist(err) {
			ctx.Handle(404, "", err)
		} else {
			ctx.Handle(500, "GetMilestoneByRepoID", err)
		}
		return
	}

	switch ctx.Params(":action") {
	case "open":
		if m.IsClosed {
			if err = models.ChangeMilestoneStatus(m, false); err != nil {
				ctx.Handle(500, "ChangeMilestoneStatus", err)
				return
			}
		}
		ctx.Redirect(ctx.Repo.RepoLink + "/milestones?state=open")
	case "close":
		if !m.IsClosed {
			m.ClosedDate = time.Now()
			if err = models.ChangeMilestoneStatus(m, true); err != nil {
				ctx.Handle(500, "ChangeMilestoneStatus", err)
				return
			}
		}
		ctx.Redirect(ctx.Repo.RepoLink + "/milestones?state=closed")
	default:
		ctx.Redirect(ctx.Repo.RepoLink + "/milestones")
	}
}

// DeleteMilestone delete a milestone
func DeleteMilestone(ctx *context.Context) {
	if err := models.DeleteMilestoneByRepoID(ctx.Repo.Repository.ID, ctx.QueryInt64("id")); err != nil {
		ctx.Flash.Error("DeleteMilestoneByRepoID: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.milestones.deletion_success"))
	}

	ctx.JSON(200, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/milestones",
	})
}
