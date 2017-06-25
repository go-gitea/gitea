// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"bytes"
	"fmt"

	"github.com/Unknwon/com"
	"github.com/Unknwon/paginater"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

const (
	tplDashborad base.TplName = "user/dashboard/dashboard"
	tplIssues    base.TplName = "user/dashboard/issues"
	tplProfile   base.TplName = "user/profile"
	tplOrgHome   base.TplName = "org/home"
)

// getDashboardContextUser finds out dashboard is viewing as which context user.
func getDashboardContextUser(ctx *context.Context) *models.User {
	ctxUser := ctx.User
	orgName := ctx.Params(":org")
	if len(orgName) > 0 {
		// Organization.
		org, err := models.GetUserByName(orgName)
		if err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.Handle(404, "GetUserByName", err)
			} else {
				ctx.Handle(500, "GetUserByName", err)
			}
			return nil
		}
		ctxUser = org
	}
	ctx.Data["ContextUser"] = ctxUser

	if err := ctx.User.GetOrganizations(true); err != nil {
		ctx.Handle(500, "GetOrganizations", err)
		return nil
	}
	ctx.Data["Orgs"] = ctx.User.Orgs

	return ctxUser
}

// retrieveFeeds loads feeds for the specified user
func retrieveFeeds(ctx *context.Context, user *models.User, includePrivate, isProfile bool, includeDeletedComments bool) {
	var requestingID int64
	if ctx.User != nil {
		requestingID = ctx.User.ID
	}
	actions, err := models.GetFeeds(models.GetFeedsOptions{
		RequestedUser:    user,
		RequestingUserID: requestingID,
		IncludePrivate:   includePrivate,
		OnlyPerformedBy:  isProfile,
		IncludeDeleted:   includeDeletedComments,
	})
	if err != nil {
		ctx.Handle(500, "GetFeeds", err)
		return
	}

	userCache := map[int64]*models.User{user.ID: user}
	if ctx.User != nil {
		userCache[ctx.User.ID] = ctx.User
	}
	repoCache := map[int64]*models.Repository{}
	for _, act := range actions {
		// Cache results to reduce queries.
		u, ok := userCache[act.ActUserID]
		if !ok {
			u, err = models.GetUserByID(act.ActUserID)
			if err != nil {
				if models.IsErrUserNotExist(err) {
					continue
				}
				ctx.Handle(500, "GetUserByID", err)
				return
			}
			userCache[act.ActUserID] = u
		}
		act.ActUser = u

		repo, ok := repoCache[act.RepoID]
		if !ok {
			repo, err = models.GetRepositoryByID(act.RepoID)
			if err != nil {
				if models.IsErrRepoNotExist(err) {
					continue
				}
				ctx.Handle(500, "GetRepositoryByID", err)
				return
			}
		}
		act.Repo = repo

		repoOwner, ok := userCache[repo.OwnerID]
		if !ok {
			repoOwner, err = models.GetUserByID(repo.OwnerID)
			if err != nil {
				if models.IsErrUserNotExist(err) {
					continue
				}
				ctx.Handle(500, "GetUserByID", err)
				return
			}
		}
		repo.Owner = repoOwner
	}
	ctx.Data["Feeds"] = actions
}

// Dashboard render the dashborad page
func Dashboard(ctx *context.Context) {
	ctxUser := getDashboardContextUser(ctx)
	if ctx.Written() {
		return
	}

	ctx.Data["Title"] = ctxUser.DisplayName() + " - " + ctx.Tr("dashboard")
	ctx.Data["PageIsDashboard"] = true
	ctx.Data["PageIsNews"] = true

	// Only user can have collaborative repositories.
	if !ctxUser.IsOrganization() {
		collaborateRepos, err := ctx.User.GetAccessibleRepositories(setting.UI.User.RepoPagingNum)
		if err != nil {
			ctx.Handle(500, "GetAccessibleRepositories", err)
			return
		} else if err = models.RepositoryList(collaborateRepos).LoadAttributes(); err != nil {
			ctx.Handle(500, "RepositoryList.LoadAttributes", err)
			return
		}
		ctx.Data["CollaborativeRepos"] = collaborateRepos
	}

	var err error
	var repos, mirrors []*models.Repository
	if ctxUser.IsOrganization() {
		env, err := ctxUser.AccessibleReposEnv(ctx.User.ID)
		if err != nil {
			ctx.Handle(500, "AccessibleReposEnv", err)
			return
		}
		repos, err = env.Repos(1, setting.UI.User.RepoPagingNum)
		if err != nil {
			ctx.Handle(500, "env.Repos", err)
			return
		}

		mirrors, err = env.MirrorRepos()
		if err != nil {
			ctx.Handle(500, "env.MirrorRepos", err)
			return
		}
	} else {
		if err = ctxUser.GetRepositories(1, setting.UI.User.RepoPagingNum); err != nil {
			ctx.Handle(500, "GetRepositories", err)
			return
		}
		repos = ctxUser.Repos

		mirrors, err = ctxUser.GetMirrorRepositories()
		if err != nil {
			ctx.Handle(500, "GetMirrorRepositories", err)
			return
		}
	}
	ctx.Data["Repos"] = repos
	ctx.Data["MaxShowRepoNum"] = setting.UI.User.RepoPagingNum

	if err := models.MirrorRepositoryList(mirrors).LoadAttributes(); err != nil {
		ctx.Handle(500, "MirrorRepositoryList.LoadAttributes", err)
		return
	}
	ctx.Data["MirrorCount"] = len(mirrors)
	ctx.Data["Mirrors"] = mirrors

	retrieveFeeds(ctx, ctxUser, true, false, false)
	if ctx.Written() {
		return
	}
	ctx.HTML(200, tplDashborad)
}

// Issues render the user issues page
func Issues(ctx *context.Context) {
	isPullList := ctx.Params(":type") == "pulls"
	if isPullList {
		ctx.Data["Title"] = ctx.Tr("pull_requests")
		ctx.Data["PageIsPulls"] = true
	} else {
		ctx.Data["Title"] = ctx.Tr("issues")
		ctx.Data["PageIsIssues"] = true
	}

	ctxUser := getDashboardContextUser(ctx)
	if ctx.Written() {
		return
	}

	// Organization does not have view type and filter mode.
	var (
		viewType   string
		sortType   = ctx.Query("sort")
		filterMode = models.FilterModeAll
	)

	if ctxUser.IsOrganization() {
		viewType = "all"
	} else {
		viewType = ctx.Query("type")
		types := []string{"all", "assigned", "created_by"}
		if !com.IsSliceContainsStr(types, viewType) {
			viewType = "all"
		}

		switch viewType {
		case "all":
			filterMode = models.FilterModeAll
		case "assigned":
			filterMode = models.FilterModeAssign
		case "created_by":
			filterMode = models.FilterModeCreate
		}
	}

	page := ctx.QueryInt("page")
	if page <= 1 {
		page = 1
	}

	repoID := ctx.QueryInt64("repo")
	isShowClosed := ctx.Query("state") == "closed"

	// Get repositories.
	var err error
	var userRepoIDs []int64
	if ctxUser.IsOrganization() {
		env, err := ctxUser.AccessibleReposEnv(ctx.User.ID)
		if err != nil {
			ctx.Handle(500, "AccessibleReposEnv", err)
			return
		}
		userRepoIDs, err = env.RepoIDs(1, ctxUser.NumRepos)
		if err != nil {
			ctx.Handle(500, "env.RepoIDs", err)
			return
		}
	} else {
		userRepoIDs, err = ctxUser.GetAccessRepoIDs()
		if err != nil {
			ctx.Handle(500, "ctxUser.GetAccessRepoIDs", err)
			return
		}
	}

	if len(userRepoIDs) <= 0 {
		userRepoIDs = []int64{-1}
	}

	var issues []*models.Issue
	switch filterMode {
	case models.FilterModeAll:
		// Get all issues from repositories from this user.
		issues, err = models.Issues(&models.IssuesOptions{
			RepoIDs:  userRepoIDs,
			RepoID:   repoID,
			Page:     page,
			IsClosed: util.OptionalBoolOf(isShowClosed),
			IsPull:   util.OptionalBoolOf(isPullList),
			SortType: sortType,
		})

	case models.FilterModeAssign:
		// Get all issues assigned to this user.
		issues, err = models.Issues(&models.IssuesOptions{
			RepoID:     repoID,
			AssigneeID: ctxUser.ID,
			Page:       page,
			IsClosed:   util.OptionalBoolOf(isShowClosed),
			IsPull:     util.OptionalBoolOf(isPullList),
			SortType:   sortType,
		})

	case models.FilterModeCreate:
		// Get all issues created by this user.
		issues, err = models.Issues(&models.IssuesOptions{
			RepoID:   repoID,
			PosterID: ctxUser.ID,
			Page:     page,
			IsClosed: util.OptionalBoolOf(isShowClosed),
			IsPull:   util.OptionalBoolOf(isPullList),
			SortType: sortType,
		})
	case models.FilterModeMention:
		// Get all issues created by this user.
		issues, err = models.Issues(&models.IssuesOptions{
			RepoID:      repoID,
			MentionedID: ctxUser.ID,
			Page:        page,
			IsClosed:    util.OptionalBoolOf(isShowClosed),
			IsPull:      util.OptionalBoolOf(isPullList),
			SortType:    sortType,
		})
	}

	if err != nil {
		ctx.Handle(500, "Issues", err)
		return
	}

	showRepos, err := models.IssueList(issues).LoadRepositories()
	if err != nil {
		ctx.Handle(500, "LoadRepositories", fmt.Errorf("%v", err))
		return
	}

	if repoID > 0 {
		var theRepo *models.Repository
		for _, repo := range showRepos {
			if repo.ID == repoID {
				theRepo = repo
				break
			}
		}

		if theRepo == nil {
			theRepo, err = models.GetRepositoryByID(repoID)
			if err != nil {
				ctx.Handle(500, "GetRepositoryByID", fmt.Errorf("[#%d]%v", repoID, err))
				return
			}
			showRepos = append(showRepos, theRepo)
		}

		// Check if user has access to given repository.
		if !theRepo.IsOwnedBy(ctxUser.ID) && !theRepo.HasAccess(ctxUser) {
			ctx.Handle(404, "Issues", fmt.Errorf("#%d", repoID))
			return
		}
	}

	err = models.RepositoryList(showRepos).LoadAttributes()
	if err != nil {
		ctx.Handle(500, "LoadAttributes", fmt.Errorf("%v", err))
		return
	}

	issueStats := models.GetUserIssueStats(repoID, ctxUser.ID, userRepoIDs, filterMode, isPullList)

	var total int
	if !isShowClosed {
		total = int(issueStats.OpenCount)
	} else {
		total = int(issueStats.ClosedCount)
	}

	ctx.Data["Issues"] = issues
	ctx.Data["Repos"] = showRepos
	ctx.Data["Page"] = paginater.New(total, setting.UI.IssuePagingNum, page, 5)
	ctx.Data["IssueStats"] = issueStats
	ctx.Data["ViewType"] = viewType
	ctx.Data["SortType"] = sortType
	ctx.Data["RepoID"] = repoID
	ctx.Data["IsShowClosed"] = isShowClosed

	if isShowClosed {
		ctx.Data["State"] = "closed"
	} else {
		ctx.Data["State"] = "open"
	}

	ctx.HTML(200, tplIssues)
}

// ShowSSHKeys output all the ssh keys of user by uid
func ShowSSHKeys(ctx *context.Context, uid int64) {
	keys, err := models.ListPublicKeys(uid)
	if err != nil {
		ctx.Handle(500, "ListPublicKeys", err)
		return
	}

	var buf bytes.Buffer
	for i := range keys {
		buf.WriteString(keys[i].OmitEmail())
		buf.WriteString("\n")
	}
	ctx.PlainText(200, buf.Bytes())
}

func showOrgProfile(ctx *context.Context) {
	ctx.SetParams(":org", ctx.Params(":username"))
	context.HandleOrgAssignment(ctx)
	if ctx.Written() {
		return
	}

	org := ctx.Org.Organization
	ctx.Data["Title"] = org.DisplayName()

	page := ctx.QueryInt("page")
	if page <= 0 {
		page = 1
	}

	var (
		repos []*models.Repository
		count int64
		err   error
	)
	if ctx.IsSigned && !ctx.User.IsAdmin {
		env, err := org.AccessibleReposEnv(ctx.User.ID)
		if err != nil {
			ctx.Handle(500, "AccessibleReposEnv", err)
			return
		}
		repos, err = env.Repos(page, setting.UI.User.RepoPagingNum)
		if err != nil {
			ctx.Handle(500, "env.Repos", err)
			return
		}
		count, err = env.CountRepos()
		if err != nil {
			ctx.Handle(500, "env.CountRepos", err)
			return
		}
		ctx.Data["Repos"] = repos
	} else {
		showPrivate := ctx.IsSigned && ctx.User.IsAdmin
		repos, err = models.GetUserRepositories(org.ID, showPrivate, page, setting.UI.User.RepoPagingNum, "")
		if err != nil {
			ctx.Handle(500, "GetRepositories", err)
			return
		}
		ctx.Data["Repos"] = repos
		count = models.CountUserRepositories(org.ID, showPrivate)
	}
	ctx.Data["Page"] = paginater.New(int(count), setting.UI.User.RepoPagingNum, page, 5)

	if err := org.GetMembers(); err != nil {
		ctx.Handle(500, "GetMembers", err)
		return
	}
	ctx.Data["Members"] = org.Members

	ctx.Data["Teams"] = org.Teams

	ctx.HTML(200, tplOrgHome)
}

// Email2User show user page via email
func Email2User(ctx *context.Context) {
	u, err := models.GetUserByEmail(ctx.Query("email"))
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Handle(404, "GetUserByEmail", err)
		} else {
			ctx.Handle(500, "GetUserByEmail", err)
		}
		return
	}
	ctx.Redirect(setting.AppSubURL + "/user/" + u.Name)
}
