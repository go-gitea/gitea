// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/Unknwon/com"
	"github.com/keybase/go-crypto/openpgp"
	"github.com/keybase/go-crypto/openpgp/armor"
)

const (
	tplDashboard base.TplName = "user/dashboard/dashboard"
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
				ctx.NotFound("GetUserByName", err)
			} else {
				ctx.ServerError("GetUserByName", err)
			}
			return nil
		}
		ctxUser = org
	}
	ctx.Data["ContextUser"] = ctxUser

	if err := ctx.User.GetOrganizations(true); err != nil {
		ctx.ServerError("GetOrganizations", err)
		return nil
	}
	ctx.Data["Orgs"] = ctx.User.Orgs

	return ctxUser
}

// retrieveFeeds loads feeds for the specified user
func retrieveFeeds(ctx *context.Context, options models.GetFeedsOptions) {
	actions, err := models.GetFeeds(options)
	if err != nil {
		ctx.ServerError("GetFeeds", err)
		return
	}

	userCache := map[int64]*models.User{options.RequestedUser.ID: options.RequestedUser}
	if ctx.User != nil {
		userCache[ctx.User.ID] = ctx.User
	}
	for _, act := range actions {
		if act.ActUser != nil {
			userCache[act.ActUserID] = act.ActUser
		}

		repoOwner, ok := userCache[act.Repo.OwnerID]
		if !ok {
			repoOwner, err = models.GetUserByID(act.Repo.OwnerID)
			if err != nil {
				if models.IsErrUserNotExist(err) {
					continue
				}
				ctx.ServerError("GetUserByID", err)
				return
			}
			userCache[repoOwner.ID] = repoOwner
		}
		act.Repo.Owner = repoOwner
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
	ctx.Data["SearchLimit"] = setting.UI.User.RepoPagingNum
	ctx.Data["EnableHeatmap"] = setting.Service.EnableUserHeatmap
	ctx.Data["HeatmapUser"] = ctxUser.Name

	var err error
	var mirrors []*models.Repository
	if ctxUser.IsOrganization() {
		env, err := ctxUser.AccessibleReposEnv(ctx.User.ID)
		if err != nil {
			ctx.ServerError("AccessibleReposEnv", err)
			return
		}

		mirrors, err = env.MirrorRepos()
		if err != nil {
			ctx.ServerError("env.MirrorRepos", err)
			return
		}
	} else {
		mirrors, err = ctxUser.GetMirrorRepositories()
		if err != nil {
			ctx.ServerError("GetMirrorRepositories", err)
			return
		}
	}
	ctx.Data["MaxShowRepoNum"] = setting.UI.User.RepoPagingNum

	if err := models.MirrorRepositoryList(mirrors).LoadAttributes(); err != nil {
		ctx.ServerError("MirrorRepositoryList.LoadAttributes", err)
		return
	}
	ctx.Data["MirrorCount"] = len(mirrors)
	ctx.Data["Mirrors"] = mirrors

	retrieveFeeds(ctx, models.GetFeedsOptions{
		RequestedUser:   ctxUser,
		IncludePrivate:  true,
		OnlyPerformedBy: false,
		IncludeDeleted:  false,
	})

	if ctx.Written() {
		return
	}
	ctx.HTML(200, tplDashboard)
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
		switch viewType {
		case "assigned":
			filterMode = models.FilterModeAssign
		case "created_by":
			filterMode = models.FilterModeCreate
		case "all": // filterMode already set to All
		default:
			viewType = "all"
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
			ctx.ServerError("AccessibleReposEnv", err)
			return
		}
		userRepoIDs, err = env.RepoIDs(1, ctxUser.NumRepos)
		if err != nil {
			ctx.ServerError("env.RepoIDs", err)
			return
		}
	} else {
		unitType := models.UnitTypeIssues
		if isPullList {
			unitType = models.UnitTypePullRequests
		}
		userRepoIDs, err = ctxUser.GetAccessRepoIDs(unitType)
		if err != nil {
			ctx.ServerError("ctxUser.GetAccessRepoIDs", err)
			return
		}
	}
	if len(userRepoIDs) == 0 {
		userRepoIDs = []int64{-1}
	}

	opts := &models.IssuesOptions{
		IsClosed: util.OptionalBoolOf(isShowClosed),
		IsPull:   util.OptionalBoolOf(isPullList),
		SortType: sortType,
	}

	if repoID > 0 {
		opts.RepoIDs = []int64{repoID}
	}

	switch filterMode {
	case models.FilterModeAll:
		if repoID > 0 {
			if !com.IsSliceContainsInt64(userRepoIDs, repoID) {
				// force an empty result
				opts.RepoIDs = []int64{-1}
			}
		} else {
			opts.RepoIDs = userRepoIDs
		}
	case models.FilterModeAssign:
		opts.AssigneeID = ctxUser.ID
	case models.FilterModeCreate:
		opts.PosterID = ctxUser.ID
	case models.FilterModeMention:
		opts.MentionedID = ctxUser.ID
	}

	counts, err := models.CountIssuesByRepo(opts)
	if err != nil {
		ctx.ServerError("CountIssuesByRepo", err)
		return
	}

	opts.Page = page
	opts.PageSize = setting.UI.IssuePagingNum
	var labelIDs []int64
	selectLabels := ctx.Query("labels")
	if len(selectLabels) > 0 && selectLabels != "0" {
		labelIDs, err = base.StringsToInt64s(strings.Split(selectLabels, ","))
		if err != nil {
			ctx.ServerError("StringsToInt64s", err)
			return
		}
	}
	opts.LabelIDs = labelIDs

	issues, err := models.Issues(opts)
	if err != nil {
		ctx.ServerError("Issues", err)
		return
	}

	showReposMap := make(map[int64]*models.Repository, len(counts))
	for repoID := range counts {
		repo, err := models.GetRepositoryByID(repoID)
		if err != nil {
			ctx.ServerError("GetRepositoryByID", err)
			return
		}
		showReposMap[repoID] = repo
	}

	if repoID > 0 {
		if _, ok := showReposMap[repoID]; !ok {
			repo, err := models.GetRepositoryByID(repoID)
			if models.IsErrRepoNotExist(err) {
				ctx.NotFound("GetRepositoryByID", err)
				return
			} else if err != nil {
				ctx.ServerError("GetRepositoryByID", fmt.Errorf("[%d]%v", repoID, err))
				return
			}
			showReposMap[repoID] = repo
		}

		repo := showReposMap[repoID]

		// Check if user has access to given repository.
		perm, err := models.GetUserRepoPermission(repo, ctxUser)
		if err != nil {
			ctx.ServerError("GetUserRepoPermission", fmt.Errorf("[%d]%v", repoID, err))
			return
		}
		if !perm.CanRead(models.UnitTypeIssues) {
			if log.IsTrace() {
				log.Trace("Permission Denied: User %-v cannot read %-v of repo %-v\n"+
					"User in repo has Permissions: %-+v",
					ctxUser,
					models.UnitTypeIssues,
					repo,
					perm)
			}
			ctx.Status(404)
			return
		}
	}

	showRepos := models.RepositoryListOfMap(showReposMap)
	sort.Sort(showRepos)
	if err = showRepos.LoadAttributes(); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}

	var commitStatus = make(map[int64]*models.CommitStatus, len(issues))
	for _, issue := range issues {
		issue.Repo = showReposMap[issue.RepoID]

		if isPullList {
			commitStatus[issue.PullRequest.ID], _ = issue.PullRequest.GetLastCommitStatus()
		}
	}

	issueStats, err := models.GetUserIssueStats(models.UserIssueStatsOptions{
		UserID:      ctxUser.ID,
		RepoID:      repoID,
		UserRepoIDs: userRepoIDs,
		FilterMode:  filterMode,
		IsPull:      isPullList,
		IsClosed:    isShowClosed,
	})
	if err != nil {
		ctx.ServerError("GetUserIssueStats", err)
		return
	}

	var total int
	if !isShowClosed {
		total = int(issueStats.OpenCount)
	} else {
		total = int(issueStats.ClosedCount)
	}

	ctx.Data["Issues"] = issues
	ctx.Data["CommitStatus"] = commitStatus
	ctx.Data["Repos"] = showRepos
	ctx.Data["Counts"] = counts
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

	pager := context.NewPagination(total, setting.UI.IssuePagingNum, page, 5)
	pager.AddParam(ctx, "type", "ViewType")
	pager.AddParam(ctx, "repo", "RepoID")
	pager.AddParam(ctx, "sort", "SortType")
	pager.AddParam(ctx, "state", "State")
	pager.AddParam(ctx, "labels", "SelectLabels")
	pager.AddParam(ctx, "milestone", "MilestoneID")
	pager.AddParam(ctx, "assignee", "AssigneeID")
	ctx.Data["Page"] = pager

	ctx.HTML(200, tplIssues)
}

// ShowSSHKeys output all the ssh keys of user by uid
func ShowSSHKeys(ctx *context.Context, uid int64) {
	keys, err := models.ListPublicKeys(uid)
	if err != nil {
		ctx.ServerError("ListPublicKeys", err)
		return
	}

	var buf bytes.Buffer
	for i := range keys {
		buf.WriteString(keys[i].OmitEmail())
		buf.WriteString("\n")
	}
	ctx.PlainText(200, buf.Bytes())
}

// ShowGPGKeys output all the public GPG keys of user by uid
func ShowGPGKeys(ctx *context.Context, uid int64) {
	keys, err := models.ListGPGKeys(uid)
	if err != nil {
		ctx.ServerError("ListGPGKeys", err)
		return
	}
	entities := make([]*openpgp.Entity, 0)
	failedEntitiesID := make([]string, 0)
	for _, k := range keys {
		e, err := models.GPGKeyToEntity(k)
		if err != nil {
			if models.IsErrGPGKeyImportNotExist(err) {
				failedEntitiesID = append(failedEntitiesID, k.KeyID)
				continue //Skip previous import without backup of imported armored key
			}
			ctx.ServerError("ShowGPGKeys", err)
			return
		}
		entities = append(entities, e)
	}
	var buf bytes.Buffer

	headers := make(map[string]string)
	if len(failedEntitiesID) > 0 { //If some key need re-import to be exported
		headers["Note"] = fmt.Sprintf("The keys with the following IDs couldn't be exported and need to be reuploaded %s", strings.Join(failedEntitiesID, ", "))
	}
	writer, _ := armor.Encode(&buf, "PGP PUBLIC KEY BLOCK", headers)
	for _, e := range entities {
		err = e.Serialize(writer) //TODO find why key are exported with a different cipherTypeByte as original (should not be blocking but strange)
		if err != nil {
			ctx.ServerError("ShowGPGKeys", err)
			return
		}
	}
	writer.Close()
	ctx.PlainText(200, buf.Bytes())
}

func showOrgProfile(ctx *context.Context) {
	ctx.SetParams(":org", ctx.Params(":username"))
	context.HandleOrgAssignment(ctx)
	if ctx.Written() {
		return
	}

	org := ctx.Org.Organization

	if !models.HasOrgVisible(org, ctx.User) {
		ctx.NotFound("HasOrgVisible", nil)
		return
	}

	ctx.Data["Title"] = org.DisplayName()

	var orderBy models.SearchOrderBy
	ctx.Data["SortType"] = ctx.Query("sort")
	switch ctx.Query("sort") {
	case "newest":
		orderBy = models.SearchOrderByNewest
	case "oldest":
		orderBy = models.SearchOrderByOldest
	case "recentupdate":
		orderBy = models.SearchOrderByRecentUpdated
	case "leastupdate":
		orderBy = models.SearchOrderByLeastUpdated
	case "reversealphabetically":
		orderBy = models.SearchOrderByAlphabeticallyReverse
	case "alphabetically":
		orderBy = models.SearchOrderByAlphabetically
	case "moststars":
		orderBy = models.SearchOrderByStarsReverse
	case "feweststars":
		orderBy = models.SearchOrderByStars
	case "mostforks":
		orderBy = models.SearchOrderByForksReverse
	case "fewestforks":
		orderBy = models.SearchOrderByForks
	default:
		ctx.Data["SortType"] = "recentupdate"
		orderBy = models.SearchOrderByRecentUpdated
	}

	keyword := strings.Trim(ctx.Query("q"), " ")
	ctx.Data["Keyword"] = keyword

	page := ctx.QueryInt("page")
	if page <= 0 {
		page = 1
	}

	var (
		repos []*models.Repository
		count int64
		err   error
	)
	repos, count, err = models.SearchRepositoryByName(&models.SearchRepoOptions{
		Keyword:     keyword,
		OwnerID:     org.ID,
		OrderBy:     orderBy,
		Private:     ctx.IsSigned,
		UserIsAdmin: ctx.IsUserSiteAdmin(),
		UserID:      ctx.Data["SignedUserID"].(int64),
		Page:        page,
		IsProfile:   true,
		PageSize:    setting.UI.User.RepoPagingNum,
	})
	if err != nil {
		ctx.ServerError("SearchRepositoryByName", err)
		return
	}

	if err := org.GetMembers(); err != nil {
		ctx.ServerError("GetMembers", err)
		return
	}

	ctx.Data["Repos"] = repos
	ctx.Data["Total"] = count
	ctx.Data["Members"] = org.Members
	ctx.Data["Teams"] = org.Teams

	pager := context.NewPagination(int(count), setting.UI.User.RepoPagingNum, page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager

	ctx.HTML(200, tplOrgHome)
}

// Email2User show user page via email
func Email2User(ctx *context.Context) {
	u, err := models.GetUserByEmail(ctx.Query("email"))
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.NotFound("GetUserByEmail", err)
		} else {
			ctx.ServerError("GetUserByEmail", err)
		}
		return
	}
	ctx.Redirect(setting.AppSubURL + "/user/" + u.Name)
}
