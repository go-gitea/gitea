// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"sort"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	project_model "code.gitea.io/gitea/models/project"
	pull_model "code.gitea.io/gitea/models/pull"
	"code.gitea.io/gitea/models/renderhelper"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/emoji"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/templates/vars"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/context/upload"
	issue_service "code.gitea.io/gitea/services/issue"
	pull_service "code.gitea.io/gitea/services/pull"
	user_service "code.gitea.io/gitea/services/user"
)

// roleDescriptor returns the role descriptor for a comment in/with the given repo, poster and issue
func roleDescriptor(ctx *context.Context, repo *repo_model.Repository, poster *user_model.User, permsCache map[int64]access_model.Permission, issue *issues_model.Issue, hasOriginalAuthor bool) (roleDesc issues_model.RoleDescriptor, err error) {
	if hasOriginalAuthor {
		// the poster is a migrated user, so no need to detect the role
		return roleDesc, nil
	}

	if poster.IsGhost() || !poster.IsIndividual() {
		return roleDesc, nil
	}

	roleDesc.IsPoster = issue.IsPoster(poster.ID) // check whether the comment's poster is the issue's poster

	// Guess the role of the poster in the repo by permission
	perm, hasPermCache := permsCache[poster.ID]
	if !hasPermCache {
		perm, err = access_model.GetUserRepoPermission(ctx, repo, poster)
		if err != nil {
			return roleDesc, err
		}
	}
	if permsCache != nil {
		permsCache[poster.ID] = perm
	}

	// Check if the poster is owner of the repo.
	if perm.IsOwner() {
		// If the poster isn't a site admin, then is must be the repo's owner
		if !poster.IsAdmin {
			roleDesc.RoleInRepo = issues_model.RoleRepoOwner
			return roleDesc, nil
		}
		// Otherwise (poster is site admin), check if poster is the real repo admin.
		isRealRepoAdmin, err := access_model.IsUserRealRepoAdmin(ctx, repo, poster)
		if err != nil {
			return roleDesc, err
		}
		if isRealRepoAdmin {
			roleDesc.RoleInRepo = issues_model.RoleRepoOwner
			return roleDesc, nil
		}
	}

	// If repo is organization, check Member role
	if err = repo.LoadOwner(ctx); err != nil {
		return roleDesc, err
	}
	if repo.Owner.IsOrganization() {
		if isMember, err := organization.IsOrganizationMember(ctx, repo.Owner.ID, poster.ID); err != nil {
			return roleDesc, err
		} else if isMember {
			roleDesc.RoleInRepo = issues_model.RoleRepoMember
			return roleDesc, nil
		}
	}

	// If the poster is the collaborator of the repo
	if isCollaborator, err := repo_model.IsCollaborator(ctx, repo.ID, poster.ID); err != nil {
		return roleDesc, err
	} else if isCollaborator {
		roleDesc.RoleInRepo = issues_model.RoleRepoCollaborator
		return roleDesc, nil
	}

	hasMergedPR, err := issues_model.HasMergedPullRequestInRepo(ctx, repo.ID, poster.ID)
	if err != nil {
		return roleDesc, err
	} else if hasMergedPR {
		roleDesc.RoleInRepo = issues_model.RoleRepoContributor
	} else if issue.IsPull {
		// only display first time contributor in the first opening pull request
		roleDesc.RoleInRepo = issues_model.RoleRepoFirstTimeContributor
	}

	return roleDesc, nil
}

func getBranchData(ctx *context.Context, issue *issues_model.Issue) {
	ctx.Data["BaseBranch"] = nil
	ctx.Data["HeadBranch"] = nil
	ctx.Data["HeadUserName"] = nil
	ctx.Data["BaseName"] = ctx.Repo.Repository.OwnerName
	if issue.IsPull {
		pull := issue.PullRequest
		ctx.Data["BaseBranch"] = pull.BaseBranch
		ctx.Data["HeadBranch"] = pull.HeadBranch
		ctx.Data["HeadUserName"] = pull.MustHeadUserName(ctx)
	}
}

// checkBlockedByIssues return canRead and notPermitted
func checkBlockedByIssues(ctx *context.Context, blockers []*issues_model.DependencyInfo) (canRead, notPermitted []*issues_model.DependencyInfo) {
	repoPerms := make(map[int64]access_model.Permission)
	repoPerms[ctx.Repo.Repository.ID] = ctx.Repo.Permission
	for _, blocker := range blockers {
		// Get the permissions for this repository
		// If the repo ID exists in the map, return the exist permissions
		// else get the permission and add it to the map
		var perm access_model.Permission
		existPerm, ok := repoPerms[blocker.RepoID]
		if ok {
			perm = existPerm
		} else {
			var err error
			perm, err = access_model.GetUserRepoPermission(ctx, &blocker.Repository, ctx.Doer)
			if err != nil {
				ctx.ServerError("GetUserRepoPermission", err)
				return nil, nil
			}
			repoPerms[blocker.RepoID] = perm
		}
		if perm.CanReadIssuesOrPulls(blocker.Issue.IsPull) {
			canRead = append(canRead, blocker)
		} else {
			notPermitted = append(notPermitted, blocker)
		}
	}
	sortDependencyInfo(canRead)
	sortDependencyInfo(notPermitted)
	return canRead, notPermitted
}

func sortDependencyInfo(blockers []*issues_model.DependencyInfo) {
	sort.Slice(blockers, func(i, j int) bool {
		if blockers[i].RepoID == blockers[j].RepoID {
			return blockers[i].Issue.CreatedUnix < blockers[j].Issue.CreatedUnix
		}
		return blockers[i].RepoID < blockers[j].RepoID
	})
}

func addParticipant(poster *user_model.User, participants []*user_model.User) []*user_model.User {
	for _, part := range participants {
		if poster.ID == part.ID {
			return participants
		}
	}
	return append(participants, poster)
}

func filterXRefComments(ctx *context.Context, issue *issues_model.Issue) error {
	// Remove comments that the user has no permissions to see
	for i := 0; i < len(issue.Comments); {
		c := issue.Comments[i]
		if issues_model.CommentTypeIsRef(c.Type) && c.RefRepoID != issue.RepoID && c.RefRepoID != 0 {
			var err error
			// Set RefRepo for description in template
			c.RefRepo, err = repo_model.GetRepositoryByID(ctx, c.RefRepoID)
			if err != nil {
				return err
			}
			perm, err := access_model.GetUserRepoPermission(ctx, c.RefRepo, ctx.Doer)
			if err != nil {
				return err
			}
			if !perm.CanReadIssuesOrPulls(c.RefIsPull) {
				issue.Comments = append(issue.Comments[:i], issue.Comments[i+1:]...)
				continue
			}
		}
		i++
	}
	return nil
}

// combineLabelComments combine the nearby label comments as one.
func combineLabelComments(issue *issues_model.Issue) {
	var prev, cur *issues_model.Comment
	for i := 0; i < len(issue.Comments); i++ {
		cur = issue.Comments[i]
		if i > 0 {
			prev = issue.Comments[i-1]
		}
		if i == 0 || cur.Type != issues_model.CommentTypeLabel ||
			(prev != nil && prev.PosterID != cur.PosterID) ||
			(prev != nil && cur.CreatedUnix-prev.CreatedUnix >= 60) {
			if cur.Type == issues_model.CommentTypeLabel && cur.Label != nil {
				if cur.Content != "1" {
					cur.RemovedLabels = append(cur.RemovedLabels, cur.Label)
				} else {
					cur.AddedLabels = append(cur.AddedLabels, cur.Label)
				}
			}
			continue
		}

		if cur.Label != nil { // now cur MUST be label comment
			if prev.Type == issues_model.CommentTypeLabel { // we can combine them only prev is a label comment
				if cur.Content != "1" {
					// remove labels from the AddedLabels list if the label that was removed is already
					// in this list, and if it's not in this list, add the label to RemovedLabels
					addedAndRemoved := false
					for i, label := range prev.AddedLabels {
						if cur.Label.ID == label.ID {
							prev.AddedLabels = append(prev.AddedLabels[:i], prev.AddedLabels[i+1:]...)
							addedAndRemoved = true
							break
						}
					}
					if !addedAndRemoved {
						prev.RemovedLabels = append(prev.RemovedLabels, cur.Label)
					}
				} else {
					// remove labels from the RemovedLabels list if the label that was added is already
					// in this list, and if it's not in this list, add the label to AddedLabels
					removedAndAdded := false
					for i, label := range prev.RemovedLabels {
						if cur.Label.ID == label.ID {
							prev.RemovedLabels = append(prev.RemovedLabels[:i], prev.RemovedLabels[i+1:]...)
							removedAndAdded = true
							break
						}
					}
					if !removedAndAdded {
						prev.AddedLabels = append(prev.AddedLabels, cur.Label)
					}
				}
				prev.CreatedUnix = cur.CreatedUnix
				// remove the current comment since it has been combined to prev comment
				issue.Comments = append(issue.Comments[:i], issue.Comments[i+1:]...)
				i--
			} else { // if prev is not a label comment, start a new group
				if cur.Content != "1" {
					cur.RemovedLabels = append(cur.RemovedLabels, cur.Label)
				} else {
					cur.AddedLabels = append(cur.AddedLabels, cur.Label)
				}
			}
		}
	}
}

// ViewIssue render issue view page
func ViewIssue(ctx *context.Context) {
	if ctx.PathParam("type") == "issues" {
		// If issue was requested we check if repo has external tracker and redirect
		extIssueUnit, err := ctx.Repo.Repository.GetUnit(ctx, unit.TypeExternalTracker)
		if err == nil && extIssueUnit != nil {
			if extIssueUnit.ExternalTrackerConfig().ExternalTrackerStyle == markup.IssueNameStyleNumeric || extIssueUnit.ExternalTrackerConfig().ExternalTrackerStyle == "" {
				metas := ctx.Repo.Repository.ComposeMetas(ctx)
				metas["index"] = ctx.PathParam("index")
				res, err := vars.Expand(extIssueUnit.ExternalTrackerConfig().ExternalTrackerFormat, metas)
				if err != nil {
					log.Error("unable to expand template vars for issue url. issue: %s, err: %v", metas["index"], err)
					ctx.ServerError("Expand", err)
					return
				}
				ctx.Redirect(res)
				return
			}
		} else if err != nil && !repo_model.IsErrUnitTypeNotExist(err) {
			ctx.ServerError("GetUnit", err)
			return
		}
	}

	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetIssueByIndex", err)
		}
		return
	}
	if issue.Repo == nil {
		issue.Repo = ctx.Repo.Repository
	}

	// Make sure type and URL matches.
	if ctx.PathParam("type") == "issues" && issue.IsPull {
		ctx.Redirect(issue.Link())
		return
	} else if ctx.PathParam("type") == "pulls" && !issue.IsPull {
		ctx.Redirect(issue.Link())
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
		ctx.Data["NewIssueChooseTemplate"] = issue_service.HasTemplatesOrContactLinks(ctx.Repo.Repository, ctx.Repo.GitRepo)
	}

	ctx.Data["IsProjectsEnabled"] = ctx.Repo.CanRead(unit.TypeProjects)
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "comment")

	if err = issue.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}

	if err = filterXRefComments(ctx, issue); err != nil {
		ctx.ServerError("filterXRefComments", err)
		return
	}

	ctx.Data["Title"] = fmt.Sprintf("#%d - %s", issue.Index, emoji.ReplaceAliases(issue.Title))

	if ctx.IsSigned {
		// Update issue-user.
		if err = activities_model.SetIssueReadBy(ctx, issue.ID, ctx.Doer.ID); err != nil {
			ctx.ServerError("ReadBy", err)
			return
		}
	}

	pageMetaData := retrieveRepoIssueMetaData(ctx, ctx.Repo.Repository, issue, issue.IsPull)
	if ctx.Written() {
		return
	}
	pageMetaData.LabelsData.SetSelectedLabels(issue.Labels)

	prepareFuncs := []func(*context.Context, *issues_model.Issue){
		prepareIssueViewContent,
		func(ctx *context.Context, issue *issues_model.Issue) {
			preparePullViewPullInfo(ctx, issue)
		},
		prepareIssueViewCommentsAndSidebarParticipants,
		preparePullViewReviewAndMerge,
		prepareIssueViewSidebarWatch,
		prepareIssueViewSidebarTimeTracker,
		prepareIssueViewSidebarDependency,
		prepareIssueViewSidebarPin,
	}

	for _, prepareFunc := range prepareFuncs {
		prepareFunc(ctx, issue)
		if ctx.Written() {
			return
		}
	}

	// Get more information if it's a pull request.
	if issue.IsPull {
		if issue.PullRequest.HasMerged {
			ctx.Data["DisableStatusChange"] = issue.PullRequest.HasMerged
		} else {
			ctx.Data["DisableStatusChange"] = ctx.Data["IsPullRequestBroken"] == true && issue.IsClosed
		}
	}

	ctx.Data["Reference"] = issue.Ref
	ctx.Data["SignInLink"] = setting.AppSubURL + "/user/login?redirect_to=" + url.QueryEscape(ctx.Data["Link"].(string))
	ctx.Data["IsIssuePoster"] = ctx.IsSigned && issue.IsPoster(ctx.Doer.ID)
	ctx.Data["HasIssuesOrPullsWritePermission"] = ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)
	ctx.Data["HasProjectsWritePermission"] = ctx.Repo.CanWrite(unit.TypeProjects)
	ctx.Data["IsRepoAdmin"] = ctx.IsSigned && (ctx.Repo.IsAdmin() || ctx.Doer.IsAdmin)
	ctx.Data["LockReasons"] = setting.Repository.Issue.LockReasons
	ctx.Data["RefEndName"] = git.RefName(issue.Ref).ShortName()

	tags, err := repo_model.GetTagNamesByRepoID(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("GetTagNamesByRepoID", err)
		return
	}
	ctx.Data["Tags"] = tags

	ctx.Data["CanBlockUser"] = func(blocker, blockee *user_model.User) bool {
		return user_service.CanBlockUser(ctx, ctx.Doer, blocker, blockee)
	}

	ctx.HTML(http.StatusOK, tplIssueView)
}

func prepareIssueViewSidebarDependency(ctx *context.Context, issue *issues_model.Issue) {
	if issue.IsPull && !ctx.Repo.CanRead(unit.TypeIssues) {
		ctx.Data["IssueDependencySearchType"] = "pulls"
	} else if !issue.IsPull && !ctx.Repo.CanRead(unit.TypePullRequests) {
		ctx.Data["IssueDependencySearchType"] = "issues"
	} else {
		ctx.Data["IssueDependencySearchType"] = "all"
	}

	// Check if the user can use the dependencies
	ctx.Data["CanCreateIssueDependencies"] = ctx.Repo.CanCreateIssueDependencies(ctx, ctx.Doer, issue.IsPull)

	// check if dependencies can be created across repositories
	ctx.Data["AllowCrossRepositoryDependencies"] = setting.Service.AllowCrossRepositoryDependencies

	// Get Dependencies
	blockedBy, err := issue.BlockedByDependencies(ctx, db.ListOptions{})
	if err != nil {
		ctx.ServerError("BlockedByDependencies", err)
		return
	}
	ctx.Data["BlockedByDependencies"], ctx.Data["BlockedByDependenciesNotPermitted"] = checkBlockedByIssues(ctx, blockedBy)
	if ctx.Written() {
		return
	}

	blocking, err := issue.BlockingDependencies(ctx)
	if err != nil {
		ctx.ServerError("BlockingDependencies", err)
		return
	}

	ctx.Data["BlockingDependencies"], ctx.Data["BlockingDependenciesNotPermitted"] = checkBlockedByIssues(ctx, blocking)
}

func preparePullViewSigning(ctx *context.Context, issue *issues_model.Issue) {
	if !issue.IsPull {
		return
	}
	pull := issue.PullRequest
	ctx.Data["WillSign"] = false
	if ctx.Doer != nil {
		sign, key, _, err := asymkey_service.SignMerge(ctx, pull, ctx.Doer, pull.BaseRepo.RepoPath(), pull.BaseBranch, pull.GetGitRefName())
		ctx.Data["WillSign"] = sign
		ctx.Data["SigningKey"] = key
		if err != nil {
			if asymkey_service.IsErrWontSign(err) {
				ctx.Data["WontSignReason"] = err.(*asymkey_service.ErrWontSign).Reason
			} else {
				ctx.Data["WontSignReason"] = "error"
				log.Error("Error whilst checking if could sign pr %d in repo %s. Error: %v", pull.ID, pull.BaseRepo.FullName(), err)
			}
		}
	} else {
		ctx.Data["WontSignReason"] = "not_signed_in"
	}
}

func prepareIssueViewSidebarWatch(ctx *context.Context, issue *issues_model.Issue) {
	iw := new(issues_model.IssueWatch)
	if ctx.Doer != nil {
		iw.UserID = ctx.Doer.ID
		iw.IssueID = issue.ID
		var err error
		iw.IsWatching, err = issues_model.CheckIssueWatch(ctx, ctx.Doer, issue)
		if err != nil {
			ctx.ServerError("CheckIssueWatch", err)
			return
		}
	}
	ctx.Data["IssueWatch"] = iw
}

func prepareIssueViewSidebarTimeTracker(ctx *context.Context, issue *issues_model.Issue) {
	if !ctx.Repo.Repository.IsTimetrackerEnabled(ctx) {
		return
	}

	if ctx.IsSigned {
		// Deal with the stopwatch
		ctx.Data["IsStopwatchRunning"] = issues_model.StopwatchExists(ctx, ctx.Doer.ID, issue.ID)
		if !ctx.Data["IsStopwatchRunning"].(bool) {
			exists, _, swIssue, err := issues_model.HasUserStopwatch(ctx, ctx.Doer.ID)
			if err != nil {
				ctx.ServerError("HasUserStopwatch", err)
				return
			}
			ctx.Data["HasUserStopwatch"] = exists
			if exists {
				// Add warning if the user has already a stopwatch
				// Add link to the issue of the already running stopwatch
				ctx.Data["OtherStopwatchURL"] = swIssue.Link()
			}
		}
		ctx.Data["CanUseTimetracker"] = ctx.Repo.CanUseTimetracker(ctx, issue, ctx.Doer)
	} else {
		ctx.Data["CanUseTimetracker"] = false
	}
	var err error
	if ctx.Data["WorkingUsers"], err = issues_model.TotalTimesForEachUser(ctx, &issues_model.FindTrackedTimesOptions{IssueID: issue.ID}); err != nil {
		ctx.ServerError("TotalTimesForEachUser", err)
		return
	}
}

func preparePullViewDeleteBranch(ctx *context.Context, issue *issues_model.Issue, canDelete bool) {
	if !issue.IsPull {
		return
	}
	pull := issue.PullRequest
	isPullBranchDeletable := canDelete &&
		pull.HeadRepo != nil &&
		git.IsBranchExist(ctx, pull.HeadRepo.RepoPath(), pull.HeadBranch) &&
		(!pull.HasMerged || ctx.Data["HeadBranchCommitID"] == ctx.Data["PullHeadCommitID"])

	if isPullBranchDeletable && pull.HasMerged {
		exist, err := issues_model.HasUnmergedPullRequestsByHeadInfo(ctx, pull.HeadRepoID, pull.HeadBranch)
		if err != nil {
			ctx.ServerError("HasUnmergedPullRequestsByHeadInfo", err)
			return
		}

		isPullBranchDeletable = !exist
	}
	ctx.Data["IsPullBranchDeletable"] = isPullBranchDeletable
}

func prepareIssueViewSidebarPin(ctx *context.Context, issue *issues_model.Issue) {
	var pinAllowed bool
	if err := issue.LoadPinOrder(ctx); err != nil {
		ctx.ServerError("LoadPinOrder", err)
		return
	}
	if issue.PinOrder == 0 {
		var err error
		pinAllowed, err = issues_model.IsNewPinAllowed(ctx, issue.RepoID, issue.IsPull)
		if err != nil {
			ctx.ServerError("IsNewPinAllowed", err)
			return
		}
	} else {
		pinAllowed = true
	}

	ctx.Data["NewPinAllowed"] = pinAllowed
	ctx.Data["PinEnabled"] = setting.Repository.Issue.MaxPinned != 0
}

func prepareIssueViewCommentsAndSidebarParticipants(ctx *context.Context, issue *issues_model.Issue) {
	var (
		role                 issues_model.RoleDescriptor
		ok                   bool
		marked               = make(map[int64]issues_model.RoleDescriptor)
		comment              *issues_model.Comment
		participants         = make([]*user_model.User, 1, 10)
		latestCloseCommentID int64
		err                  error
	)

	marked[issue.PosterID] = issue.ShowRole

	// Render comments and fetch participants.
	participants[0] = issue.Poster

	if err := issue.Comments.LoadAttachmentsByIssue(ctx); err != nil {
		ctx.ServerError("LoadAttachmentsByIssue", err)
		return
	}
	if err := issue.Comments.LoadPosters(ctx); err != nil {
		ctx.ServerError("LoadPosters", err)
		return
	}

	permCache := make(map[int64]access_model.Permission)

	for _, comment = range issue.Comments {
		comment.Issue = issue

		if comment.Type == issues_model.CommentTypeComment || comment.Type == issues_model.CommentTypeReview {
			rctx := renderhelper.NewRenderContextRepoComment(ctx, issue.Repo)
			comment.RenderedContent, err = markdown.RenderString(rctx, comment.Content)
			if err != nil {
				ctx.ServerError("RenderString", err)
				return
			}
			// Check tag.
			role, ok = marked[comment.PosterID]
			if ok {
				comment.ShowRole = role
				continue
			}

			comment.ShowRole, err = roleDescriptor(ctx, issue.Repo, comment.Poster, permCache, issue, comment.HasOriginalAuthor())
			if err != nil {
				ctx.ServerError("roleDescriptor", err)
				return
			}
			marked[comment.PosterID] = comment.ShowRole
			participants = addParticipant(comment.Poster, participants)
		} else if comment.Type == issues_model.CommentTypeLabel {
			if err = comment.LoadLabel(ctx); err != nil {
				ctx.ServerError("LoadLabel", err)
				return
			}
		} else if comment.Type == issues_model.CommentTypeMilestone {
			if err = comment.LoadMilestone(ctx); err != nil {
				ctx.ServerError("LoadMilestone", err)
				return
			}
			ghostMilestone := &issues_model.Milestone{
				ID:   -1,
				Name: ctx.Locale.TrString("repo.issues.deleted_milestone"),
			}
			if comment.OldMilestoneID > 0 && comment.OldMilestone == nil {
				comment.OldMilestone = ghostMilestone
			}
			if comment.MilestoneID > 0 && comment.Milestone == nil {
				comment.Milestone = ghostMilestone
			}
		} else if comment.Type == issues_model.CommentTypeProject {
			if err = comment.LoadProject(ctx); err != nil {
				ctx.ServerError("LoadProject", err)
				return
			}

			ghostProject := &project_model.Project{
				ID:    project_model.GhostProjectID,
				Title: ctx.Locale.TrString("repo.issues.deleted_project"),
			}

			if comment.OldProjectID > 0 && comment.OldProject == nil {
				comment.OldProject = ghostProject
			}

			if comment.ProjectID > 0 && comment.Project == nil {
				comment.Project = ghostProject
			}
		} else if comment.Type == issues_model.CommentTypeProjectColumn {
			if err = comment.LoadProject(ctx); err != nil {
				ctx.ServerError("LoadProject", err)
				return
			}
		} else if comment.Type == issues_model.CommentTypeAssignees || comment.Type == issues_model.CommentTypeReviewRequest {
			if err = comment.LoadAssigneeUserAndTeam(ctx); err != nil {
				ctx.ServerError("LoadAssigneeUserAndTeam", err)
				return
			}
		} else if comment.Type == issues_model.CommentTypeRemoveDependency || comment.Type == issues_model.CommentTypeAddDependency {
			if err = comment.LoadDepIssueDetails(ctx); err != nil {
				if !issues_model.IsErrIssueNotExist(err) {
					ctx.ServerError("LoadDepIssueDetails", err)
					return
				}
			}
		} else if comment.Type.HasContentSupport() {
			rctx := renderhelper.NewRenderContextRepoComment(ctx, issue.Repo)
			comment.RenderedContent, err = markdown.RenderString(rctx, comment.Content)
			if err != nil {
				ctx.ServerError("RenderString", err)
				return
			}
			if err = comment.LoadReview(ctx); err != nil && !issues_model.IsErrReviewNotExist(err) {
				ctx.ServerError("LoadReview", err)
				return
			}
			participants = addParticipant(comment.Poster, participants)
			if comment.Review == nil {
				continue
			}
			if err = comment.Review.LoadAttributes(ctx); err != nil {
				if !user_model.IsErrUserNotExist(err) {
					ctx.ServerError("Review.LoadAttributes", err)
					return
				}
				comment.Review.Reviewer = user_model.NewGhostUser()
			}
			if err = comment.Review.LoadCodeComments(ctx); err != nil {
				ctx.ServerError("Review.LoadCodeComments", err)
				return
			}
			for _, codeComments := range comment.Review.CodeComments {
				for _, lineComments := range codeComments {
					for _, c := range lineComments {
						// Check tag.
						role, ok = marked[c.PosterID]
						if ok {
							c.ShowRole = role
							continue
						}

						c.ShowRole, err = roleDescriptor(ctx, issue.Repo, c.Poster, permCache, issue, c.HasOriginalAuthor())
						if err != nil {
							ctx.ServerError("roleDescriptor", err)
							return
						}
						marked[c.PosterID] = c.ShowRole
						participants = addParticipant(c.Poster, participants)
					}
				}
			}
			if err = comment.LoadResolveDoer(ctx); err != nil {
				ctx.ServerError("LoadResolveDoer", err)
				return
			}
		} else if comment.Type == issues_model.CommentTypePullRequestPush {
			participants = addParticipant(comment.Poster, participants)
			if err = issue_service.LoadCommentPushCommits(ctx, comment); err != nil {
				ctx.ServerError("LoadCommentPushCommits", err)
				return
			}
			if !ctx.Repo.CanRead(unit.TypeActions) {
				for _, commit := range comment.Commits {
					commit.Status.HideActionsURL(ctx)
					git_model.CommitStatusesHideActionsURL(ctx, commit.Statuses)
				}
			}
		} else if comment.Type == issues_model.CommentTypeAddTimeManual ||
			comment.Type == issues_model.CommentTypeStopTracking ||
			comment.Type == issues_model.CommentTypeDeleteTimeManual {
			// drop error since times could be pruned from DB..
			_ = comment.LoadTime(ctx)
			if comment.Content != "" {
				// Content before v1.21 did store the formatted string instead of seconds,
				// so "|" is used as delimiter to mark the new format
				if comment.Content[0] != '|' {
					// handle old time comments that have formatted text stored
					comment.RenderedContent = templates.SanitizeHTML(comment.Content)
					comment.Content = ""
				} else {
					// else it's just a duration in seconds to pass on to the frontend
					comment.Content = comment.Content[1:]
				}
			}
		}

		if comment.Type == issues_model.CommentTypeClose || comment.Type == issues_model.CommentTypeMergePull {
			// record ID of the latest closed/merged comment.
			// if PR is closed, the comments whose type is CommentTypePullRequestPush(29) after latestCloseCommentID won't be rendered.
			latestCloseCommentID = comment.ID
		}
	}

	ctx.Data["LatestCloseCommentID"] = latestCloseCommentID

	// Combine multiple label assignments into a single comment
	combineLabelComments(issue)

	var hiddenCommentTypes *big.Int
	if ctx.IsSigned {
		val, err := user_model.GetUserSetting(ctx, ctx.Doer.ID, user_model.SettingsKeyHiddenCommentTypes)
		if err != nil {
			ctx.ServerError("GetUserSetting", err)
			return
		}
		hiddenCommentTypes, _ = new(big.Int).SetString(val, 10) // we can safely ignore the failed conversion here
	}
	ctx.Data["ShouldShowCommentType"] = func(commentType issues_model.CommentType) bool {
		return hiddenCommentTypes == nil || hiddenCommentTypes.Bit(int(commentType)) == 0
	}

	// prepare for sidebar participants
	ctx.Data["Participants"] = participants
	ctx.Data["NumParticipants"] = len(participants)
}

func preparePullViewReviewAndMerge(ctx *context.Context, issue *issues_model.Issue) {
	getBranchData(ctx, issue)
	if !issue.IsPull {
		return
	}

	pull := issue.PullRequest
	pull.Issue = issue
	canDelete := false
	allowMerge := false
	canWriteToHeadRepo := false

	if ctx.IsSigned {
		if err := pull.LoadHeadRepo(ctx); err != nil {
			log.Error("LoadHeadRepo: %v", err)
		} else if pull.HeadRepo != nil {
			perm, err := access_model.GetUserRepoPermission(ctx, pull.HeadRepo, ctx.Doer)
			if err != nil {
				ctx.ServerError("GetUserRepoPermission", err)
				return
			}
			if perm.CanWrite(unit.TypeCode) {
				// Check if branch is not protected
				if pull.HeadBranch != pull.HeadRepo.DefaultBranch {
					if protected, err := git_model.IsBranchProtected(ctx, pull.HeadRepo.ID, pull.HeadBranch); err != nil {
						log.Error("IsProtectedBranch: %v", err)
					} else if !protected {
						canDelete = true
						ctx.Data["DeleteBranchLink"] = issue.Link() + "/cleanup"
					}
				}
				canWriteToHeadRepo = true
			}
		}

		if err := pull.LoadBaseRepo(ctx); err != nil {
			log.Error("LoadBaseRepo: %v", err)
		}
		perm, err := access_model.GetUserRepoPermission(ctx, pull.BaseRepo, ctx.Doer)
		if err != nil {
			ctx.ServerError("GetUserRepoPermission", err)
			return
		}
		if !canWriteToHeadRepo { // maintainers maybe allowed to push to head repo even if they can't write to it
			canWriteToHeadRepo = pull.AllowMaintainerEdit && perm.CanWrite(unit.TypeCode)
		}
		allowMerge, err = pull_service.IsUserAllowedToMerge(ctx, pull, perm, ctx.Doer)
		if err != nil {
			ctx.ServerError("IsUserAllowedToMerge", err)
			return
		}

		if ctx.Data["CanMarkConversation"], err = issues_model.CanMarkConversation(ctx, issue, ctx.Doer); err != nil {
			ctx.ServerError("CanMarkConversation", err)
			return
		}
	}

	ctx.Data["CanWriteToHeadRepo"] = canWriteToHeadRepo
	ctx.Data["ShowMergeInstructions"] = canWriteToHeadRepo
	ctx.Data["AllowMerge"] = allowMerge

	prUnit, err := issue.Repo.GetUnit(ctx, unit.TypePullRequests)
	if err != nil {
		ctx.ServerError("GetUnit", err)
		return
	}
	prConfig := prUnit.PullRequestsConfig()

	ctx.Data["AutodetectManualMerge"] = prConfig.AutodetectManualMerge

	var mergeStyle repo_model.MergeStyle
	// Check correct values and select default
	if ms, ok := ctx.Data["MergeStyle"].(repo_model.MergeStyle); !ok ||
		!prConfig.IsMergeStyleAllowed(ms) {
		defaultMergeStyle := prConfig.GetDefaultMergeStyle()
		if prConfig.IsMergeStyleAllowed(defaultMergeStyle) && !ok {
			mergeStyle = defaultMergeStyle
		} else if prConfig.AllowMerge {
			mergeStyle = repo_model.MergeStyleMerge
		} else if prConfig.AllowRebase {
			mergeStyle = repo_model.MergeStyleRebase
		} else if prConfig.AllowRebaseMerge {
			mergeStyle = repo_model.MergeStyleRebaseMerge
		} else if prConfig.AllowSquash {
			mergeStyle = repo_model.MergeStyleSquash
		} else if prConfig.AllowFastForwardOnly {
			mergeStyle = repo_model.MergeStyleFastForwardOnly
		} else if prConfig.AllowManualMerge {
			mergeStyle = repo_model.MergeStyleManuallyMerged
		}
	}

	ctx.Data["MergeStyle"] = mergeStyle

	defaultMergeMessage, defaultMergeBody, err := pull_service.GetDefaultMergeMessage(ctx, ctx.Repo.GitRepo, pull, mergeStyle)
	if err != nil {
		ctx.ServerError("GetDefaultMergeMessage", err)
		return
	}
	ctx.Data["DefaultMergeMessage"] = defaultMergeMessage
	ctx.Data["DefaultMergeBody"] = defaultMergeBody

	defaultSquashMergeMessage, defaultSquashMergeBody, err := pull_service.GetDefaultMergeMessage(ctx, ctx.Repo.GitRepo, pull, repo_model.MergeStyleSquash)
	if err != nil {
		ctx.ServerError("GetDefaultSquashMergeMessage", err)
		return
	}
	ctx.Data["DefaultSquashMergeMessage"] = defaultSquashMergeMessage
	ctx.Data["DefaultSquashMergeBody"] = defaultSquashMergeBody

	pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, pull.BaseRepoID, pull.BaseBranch)
	if err != nil {
		ctx.ServerError("LoadProtectedBranch", err)
		return
	}

	if pb != nil {
		pb.Repo = pull.BaseRepo
		ctx.Data["ProtectedBranch"] = pb
		ctx.Data["IsBlockedByApprovals"] = !issues_model.HasEnoughApprovals(ctx, pb, pull)
		ctx.Data["IsBlockedByRejection"] = issues_model.MergeBlockedByRejectedReview(ctx, pb, pull)
		ctx.Data["IsBlockedByOfficialReviewRequests"] = issues_model.MergeBlockedByOfficialReviewRequests(ctx, pb, pull)
		ctx.Data["IsBlockedByOutdatedBranch"] = issues_model.MergeBlockedByOutdatedBranch(pb, pull)
		ctx.Data["GrantedApprovals"] = issues_model.GetGrantedApprovalsCount(ctx, pb, pull)
		ctx.Data["RequireSigned"] = pb.RequireSignedCommits
		ctx.Data["ChangedProtectedFiles"] = pull.ChangedProtectedFiles
		ctx.Data["IsBlockedByChangedProtectedFiles"] = len(pull.ChangedProtectedFiles) != 0
		ctx.Data["ChangedProtectedFilesNum"] = len(pull.ChangedProtectedFiles)
		ctx.Data["RequireApprovalsWhitelist"] = pb.EnableApprovalsWhitelist
	}

	preparePullViewSigning(ctx, issue)
	if ctx.Written() {
		return
	}

	preparePullViewDeleteBranch(ctx, issue, canDelete)
	if ctx.Written() {
		return
	}

	stillCanManualMerge := func() bool {
		if pull.HasMerged || issue.IsClosed || !ctx.IsSigned {
			return false
		}
		if pull.CanAutoMerge() || pull.IsWorkInProgress(ctx) || pull.IsChecking() {
			return false
		}
		if allowMerge && prConfig.AllowManualMerge {
			return true
		}

		return false
	}

	ctx.Data["StillCanManualMerge"] = stillCanManualMerge()

	// Check if there is a pending pr merge
	ctx.Data["HasPendingPullRequestMerge"], ctx.Data["PendingPullRequestMerge"], err = pull_model.GetScheduledMergeByPullID(ctx, pull.ID)
	if err != nil {
		ctx.ServerError("GetScheduledMergeByPullID", err)
		return
	}
}

func prepareIssueViewContent(ctx *context.Context, issue *issues_model.Issue) {
	var err error
	rctx := renderhelper.NewRenderContextRepoComment(ctx, ctx.Repo.Repository)
	issue.RenderedContent, err = markdown.RenderString(rctx, issue.Content)
	if err != nil {
		ctx.ServerError("RenderString", err)
		return
	}
	if issue.ShowRole, err = roleDescriptor(ctx, issue.Repo, issue.Poster, nil, issue, issue.HasOriginalAuthor()); err != nil {
		ctx.ServerError("roleDescriptor", err)
		return
	}
	ctx.Data["Issue"] = issue
}
