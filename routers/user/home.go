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

// retrieveFeeds loads feeds from database by given context user.
// The user could be organization so it is not always the logged in user,
// which is why we have to explicitly pass the context user ID.
func retrieveFeeds(ctx *context.Context, ctxUser *models.User, userID, offset int64, isProfile bool) {
	actions, err := models.GetFeeds(ctxUser, userID, offset, isProfile)
	if err != nil {
		ctx.Handle(500, "GetFeeds", err)
		return
	}

	// Check access of private repositories.
	feeds := make([]*models.Action, 0, len(actions))
	unameAvatars := make(map[string]string)
	for _, act := range actions {
		// Cache results to reduce queries.
		_, ok := unameAvatars[act.ActUserName]
		if !ok {
			u, err := models.GetUserByName(act.ActUserName)
			if err != nil {
				if models.IsErrUserNotExist(err) {
					continue
				}
				ctx.Handle(500, "GetUserByName", err)
				return
			}
			unameAvatars[act.ActUserName] = u.RelAvatarLink()
		}

		act.ActAvatar = unameAvatars[act.ActUserName]
		feeds = append(feeds, act)
	}
	ctx.Data["Feeds"] = feeds
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
		repos, _, err = ctxUser.GetUserRepositories(ctx.User.ID, 1, setting.UI.User.RepoPagingNum)
		if err != nil {
			ctx.Handle(500, "GetUserRepositories", err)
			return
		}

		mirrors, err = ctxUser.GetUserMirrorRepositories(ctx.User.ID)
		if err != nil {
			ctx.Handle(500, "GetUserMirrorRepositories", err)
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

	retrieveFeeds(ctx, ctxUser, ctx.User.ID, 0, false)
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
		assigneeID int64
		posterID   int64
	)
	if ctxUser.IsOrganization() {
		viewType = "all"
	} else {
		viewType = ctx.Query("type")
		types := []string{"assigned", "created_by"}
		if !com.IsSliceContainsStr(types, viewType) {
			viewType = "all"
		}

		switch viewType {
		case "assigned":
			filterMode = models.FilterModeAssign
			assigneeID = ctxUser.ID
		case "created_by":
			filterMode = models.FilterModeCreate
			posterID = ctxUser.ID
		}
	}

	repoID := ctx.QueryInt64("repo")
	isShowClosed := ctx.Query("state") == "closed"

	// Get repositories.
	var err error
	var repos []*models.Repository
	if ctxUser.IsOrganization() {
		repos, _, err = ctxUser.GetUserRepositories(ctx.User.ID, 1, ctxUser.NumRepos)
		if err != nil {
			ctx.Handle(500, "GetRepositories", err)
			return
		}
	} else {
		if err := ctxUser.GetRepositories(1, ctx.User.NumRepos); err != nil {
			ctx.Handle(500, "GetRepositories", err)
			return
		}
		repos = ctxUser.Repos
	}

	allCount := 0
	repoIDs := make([]int64, 0, len(repos))
	showRepos := make([]*models.Repository, 0, len(repos))
	for _, repo := range repos {
		if (isPullList && repo.NumPulls == 0) ||
			(!isPullList &&
				(!repo.EnableIssues || repo.EnableExternalTracker || repo.NumIssues == 0)) {
			continue
		}

		repoIDs = append(repoIDs, repo.ID)

		if isPullList {
			allCount += repo.NumOpenPulls
			repo.NumOpenIssues = repo.NumOpenPulls
			repo.NumClosedIssues = repo.NumClosedPulls
		} else {
			allCount += repo.NumOpenIssues
		}

		if filterMode != models.FilterModeAll {
			// Calculate repository issue count with filter mode.
			numOpen, numClosed := repo.IssueStats(ctxUser.ID, filterMode, isPullList)
			repo.NumOpenIssues, repo.NumClosedIssues = int(numOpen), int(numClosed)
		}

		if repo.ID == repoID ||
			(isShowClosed && repo.NumClosedIssues > 0) ||
			(!isShowClosed && repo.NumOpenIssues > 0) {
			showRepos = append(showRepos, repo)
		}
	}
	ctx.Data["Repos"] = showRepos
	if len(repoIDs) == 0 {
		repoIDs = []int64{-1}
	}

	issueStats := models.GetUserIssueStats(repoID, ctxUser.ID, repoIDs, filterMode, isPullList)
	issueStats.AllCount = int64(allCount)

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
	ctx.Data["Page"] = paginater.New(total, setting.UI.IssuePagingNum, page, 5)

	// Get issues.
	issues, err := models.Issues(&models.IssuesOptions{
		AssigneeID: assigneeID,
		RepoID:     repoID,
		PosterID:   posterID,
		RepoIDs:    repoIDs,
		Page:       page,
		IsClosed:   isShowClosed,
		IsPull:     isPullList,
		SortType:   sortType,
	})
	if err != nil {
		ctx.Handle(500, "Issues", err)
		return
	}

	// Get posters and repository.
	for i := range issues {
		issues[i].Repo, err = models.GetRepositoryByID(issues[i].RepoID)
		if err != nil {
			ctx.Handle(500, "GetRepositoryByID", fmt.Errorf("[#%d]%v", issues[i].ID, err))
			return
		}

		if err = issues[i].Repo.GetOwner(); err != nil {
			ctx.Handle(500, "GetOwner", fmt.Errorf("[#%d]%v", issues[i].ID, err))
			return
		}
	}
	ctx.Data["Issues"] = issues

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
	ctx.Data["Title"] = org.FullName

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
		repos, count, err = org.GetUserRepositories(ctx.User.ID, page, setting.UI.User.RepoPagingNum)
		if err != nil {
			ctx.Handle(500, "GetUserRepositories", err)
			return
		}
		ctx.Data["Repos"] = repos
	} else {
		showPrivate := ctx.IsSigned && ctx.User.IsAdmin
		repos, err = models.GetUserRepositories(org.ID, showPrivate, page, setting.UI.User.RepoPagingNum)
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
