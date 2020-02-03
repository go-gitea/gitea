// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/keybase/go-crypto/openpgp"
	"github.com/keybase/go-crypto/openpgp/armor"
	"github.com/unknwon/com"
)

const (
	tplDashboard  base.TplName = "user/dashboard/dashboard"
	tplIssues     base.TplName = "user/dashboard/issues"
	tplMilestones base.TplName = "user/dashboard/milestones"
	tplProfile    base.TplName = "user/profile"
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
	}

	for _, act := range actions {
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

	requestingUserID := int64(0)
	if ctx.User != nil {
		requestingUserID = ctx.User.ID
	}

	retrieveFeeds(ctx, models.GetFeedsOptions{
		RequestedUser:    ctxUser,
		RequestingUserID: requestingUserID,
		IncludePrivate:   true,
		OnlyPerformedBy:  false,
		IncludeDeleted:   false,
	})

	if ctx.Written() {
		return
	}
	ctx.HTML(200, tplDashboard)
}

// Milestones render the user milestones page
func Milestones(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("milestones")
	ctx.Data["PageIsMilestonesDashboard"] = true

	ctxUser := getDashboardContextUser(ctx)
	if ctx.Written() {
		return
	}

	sortType := ctx.Query("sort")
	page := ctx.QueryInt("page")
	if page <= 1 {
		page = 1
	}

	reposQuery := ctx.Query("repos")
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
		userRepoIDs, err = models.FilterOutRepoIdsWithoutUnitAccess(ctx.User, userRepoIDs, models.UnitTypeIssues, models.UnitTypePullRequests)
		if err != nil {
			ctx.ServerError("FilterOutRepoIdsWithoutUnitAccess", err)
			return
		}
	} else {
		userRepoIDs, err = ctxUser.GetAccessRepoIDs(models.UnitTypeIssues, models.UnitTypePullRequests)
		if err != nil {
			ctx.ServerError("ctxUser.GetAccessRepoIDs", err)
			return
		}
	}
	if len(userRepoIDs) == 0 {
		userRepoIDs = []int64{-1}
	}

	var repoIDs []int64
	if len(reposQuery) != 0 {
		if issueReposQueryPattern.MatchString(reposQuery) {
			// remove "[" and "]" from string
			reposQuery = reposQuery[1 : len(reposQuery)-1]
			//for each ID (delimiter ",") add to int to repoIDs
			reposSet := false
			for _, rID := range strings.Split(reposQuery, ",") {
				// Ensure nonempty string entries
				if rID != "" && rID != "0" {
					reposSet = true
					rIDint64, err := strconv.ParseInt(rID, 10, 64)
					// If the repo id specified by query is not parseable or not accessible by user, just ignore it.
					if err == nil && com.IsSliceContainsInt64(userRepoIDs, rIDint64) {
						repoIDs = append(repoIDs, rIDint64)
					}
				}
			}
			if reposSet && len(repoIDs) == 0 {
				// force an empty result
				repoIDs = []int64{-1}
			}
		} else {
			log.Warn("issueReposQueryPattern not match with query")
		}
	}

	if len(repoIDs) == 0 {
		repoIDs = userRepoIDs
	}

	counts, err := models.CountMilestonesByRepoIDs(userRepoIDs, isShowClosed)
	if err != nil {
		ctx.ServerError("CountMilestonesByRepoIDs", err)
		return
	}

	milestones, err := models.GetMilestonesByRepoIDs(repoIDs, page, isShowClosed, sortType)
	if err != nil {
		ctx.ServerError("GetMilestonesByRepoIDs", err)
		return
	}

	showReposMap := make(map[int64]*models.Repository, len(counts))
	for rID := range counts {
		if rID == -1 {
			break
		}
		repo, err := models.GetRepositoryByID(rID)
		if err != nil {
			if models.IsErrRepoNotExist(err) {
				ctx.NotFound("GetRepositoryByID", err)
				return
			} else if err != nil {
				ctx.ServerError("GetRepositoryByID", fmt.Errorf("[%d]%v", rID, err))
				return
			}
		}
		showReposMap[rID] = repo
	}

	showRepos := models.RepositoryListOfMap(showReposMap)
	sort.Sort(showRepos)
	if err = showRepos.LoadAttributes(); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}

	for _, m := range milestones {
		m.Repo = showReposMap[m.RepoID]
		m.RenderedContent = string(markdown.Render([]byte(m.Content), m.Repo.Link(), m.Repo.ComposeMetas()))
		if m.Repo.IsTimetrackerEnabled() {
			err := m.LoadTotalTrackedTime()
			if err != nil {
				ctx.ServerError("LoadTotalTrackedTime", err)
				return
			}
		}
	}

	milestoneStats, err := models.GetMilestonesStats(repoIDs)
	if err != nil {
		ctx.ServerError("GetMilestoneStats", err)
		return
	}

	totalMilestoneStats, err := models.GetMilestonesStats(userRepoIDs)
	if err != nil {
		ctx.ServerError("GetMilestoneStats", err)
		return
	}

	var pagerCount int
	if isShowClosed {
		ctx.Data["State"] = "closed"
		ctx.Data["Total"] = totalMilestoneStats.ClosedCount
		pagerCount = int(milestoneStats.ClosedCount)
	} else {
		ctx.Data["State"] = "open"
		ctx.Data["Total"] = totalMilestoneStats.OpenCount
		pagerCount = int(milestoneStats.OpenCount)
	}

	ctx.Data["Milestones"] = milestones
	ctx.Data["Repos"] = showRepos
	ctx.Data["Counts"] = counts
	ctx.Data["MilestoneStats"] = milestoneStats
	ctx.Data["SortType"] = sortType
	if len(repoIDs) != len(userRepoIDs) {
		ctx.Data["RepoIDs"] = repoIDs
	}
	ctx.Data["IsShowClosed"] = isShowClosed

	pager := context.NewPagination(pagerCount, setting.UI.IssuePagingNum, page, 5)
	pager.AddParam(ctx, "repos", "RepoIDs")
	pager.AddParam(ctx, "sort", "SortType")
	pager.AddParam(ctx, "state", "State")
	ctx.Data["Page"] = pager

	ctx.HTML(200, tplMilestones)
}

// Regexp for repos query
var issueReposQueryPattern = regexp.MustCompile(`^\[\d+(,\d+)*,?\]$`)

// Issues render the user issues page
func Issues(ctx *context.Context) {
	isPullList := ctx.Params(":type") == "pulls"
	unitType := models.UnitTypeIssues
	if isPullList {
		ctx.Data["Title"] = ctx.Tr("pull_requests")
		ctx.Data["PageIsPulls"] = true
		unitType = models.UnitTypePullRequests
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
		viewType = "your_repositories"
	} else {
		viewType = ctx.Query("type")
		switch viewType {
		case "assigned":
			filterMode = models.FilterModeAssign
		case "created_by":
			filterMode = models.FilterModeCreate
		case "mentioned":
			filterMode = models.FilterModeMention
		case "your_repositories": // filterMode already set to All
		default:
			viewType = "your_repositories"
		}
	}

	page := ctx.QueryInt("page")
	if page <= 1 {
		page = 1
	}

	reposQuery := ctx.Query("repos")
	var repoIDs []int64
	if len(reposQuery) != 0 {
		if issueReposQueryPattern.MatchString(reposQuery) {
			// remove "[" and "]" from string
			reposQuery = reposQuery[1 : len(reposQuery)-1]
			//for each ID (delimiter ",") add to int to repoIDs
			for _, rID := range strings.Split(reposQuery, ",") {
				// Ensure nonempty string entries
				if rID != "" && rID != "0" {
					rIDint64, err := strconv.ParseInt(rID, 10, 64)
					if err == nil {
						repoIDs = append(repoIDs, rIDint64)
					}
				}
			}
		} else {
			log.Warn("issueReposQueryPattern not match with query")
		}
	}

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
		userRepoIDs, err = models.FilterOutRepoIdsWithoutUnitAccess(ctx.User, userRepoIDs, unitType)
		if err != nil {
			ctx.ServerError("FilterOutRepoIdsWithoutUnitAccess", err)
			return
		}
	} else {
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

	switch filterMode {
	case models.FilterModeAll:
		opts.RepoIDs = userRepoIDs
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

	if len(repoIDs) > 0 {
		opts.RepoIDs = repoIDs
	}

	issues, err := models.Issues(opts)
	if err != nil {
		ctx.ServerError("Issues", err)
		return
	}

	showReposMap := make(map[int64]*models.Repository, len(counts))
	for repoID := range counts {
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
				log.Error("User created Issues in Repository which they no longer have access to: [%d]", repoID)
			}
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

	issueStatsOpts := models.UserIssueStatsOptions{
		UserID:      ctxUser.ID,
		UserRepoIDs: userRepoIDs,
		FilterMode:  filterMode,
		IsPull:      isPullList,
		IsClosed:    isShowClosed,
	}
	if len(repoIDs) > 0 {
		issueStatsOpts.UserRepoIDs = repoIDs
	}
	issueStats, err := models.GetUserIssueStats(issueStatsOpts)
	if err != nil {
		ctx.ServerError("GetUserIssueStats", err)
		return
	}

	allIssueStats, err := models.GetUserIssueStats(models.UserIssueStatsOptions{
		UserID:      ctxUser.ID,
		UserRepoIDs: userRepoIDs,
		FilterMode:  filterMode,
		IsPull:      isPullList,
		IsClosed:    isShowClosed,
	})
	if err != nil {
		ctx.ServerError("GetUserIssueStats All", err)
		return
	}

	var shownIssues int
	var totalIssues int
	if !isShowClosed {
		shownIssues = int(issueStats.OpenCount)
		totalIssues = int(allIssueStats.OpenCount)
	} else {
		shownIssues = int(issueStats.ClosedCount)
		totalIssues = int(allIssueStats.ClosedCount)
	}

	ctx.Data["Issues"] = issues
	ctx.Data["CommitStatus"] = commitStatus
	ctx.Data["Repos"] = showRepos
	ctx.Data["Counts"] = counts
	ctx.Data["IssueStats"] = issueStats
	ctx.Data["ViewType"] = viewType
	ctx.Data["SortType"] = sortType
	ctx.Data["RepoIDs"] = repoIDs
	ctx.Data["IsShowClosed"] = isShowClosed
	ctx.Data["TotalIssueCount"] = totalIssues

	if isShowClosed {
		ctx.Data["State"] = "closed"
	} else {
		ctx.Data["State"] = "open"
	}

	// Convert []int64 to string
	reposParam, _ := json.Marshal(repoIDs)

	ctx.Data["ReposParam"] = string(reposParam)

	pager := context.NewPagination(shownIssues, setting.UI.IssuePagingNum, page, 5)
	pager.AddParam(ctx, "type", "ViewType")
	pager.AddParam(ctx, "repos", "ReposParam")
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
