// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"html/template"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	activities_model "code.gitea.io/gitea/models/activities"
	conversations_model "code.gitea.io/gitea/models/conversations"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	conversation_indexer "code.gitea.io/gitea/modules/indexer/conversations"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/templates/vars"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/context/upload"
	conversation_service "code.gitea.io/gitea/services/conversation"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/services/forms"
	user_service "code.gitea.io/gitea/services/user"
)

const (
	tplConversations      base.TplName = "repo/conversation/list"
	tplConversationNew    base.TplName = "repo/conversation/new"
	tplConversationChoose base.TplName = "repo/conversation/choose"
	tplConversationView   base.TplName = "repo/conversation/view"
)

// MustAllowUserComment checks to make sure if an conversation is locked.
// If locked and user has permissions to write to the repository,
// then the comment is allowed, else it is blocked
func ConversationMustAllowUserComment(ctx *context.Context) {
	conversation := GetActionConversation(ctx)
	if ctx.Written() {
		return
	}

	if conversation.IsLocked && !ctx.Doer.IsAdmin {
		ctx.Flash.Error(ctx.Tr("repo.conversations.comment_on_locked"))
		ctx.Redirect(conversation.Link())
		return
	}
}

// MustEnableConversations check if repository enable internal conversations
func MustEnableConversations(ctx *context.Context) {
	if !ctx.Repo.CanRead(unit.TypeConversations) &&
		!ctx.Repo.CanRead(unit.TypeExternalTracker) {
		ctx.NotFound("MustEnableConversations", nil)
		return
	}

	unit, err := ctx.Repo.Repository.GetUnit(ctx, unit.TypeExternalTracker)
	if err == nil {
		ctx.Redirect(unit.ExternalTrackerConfig().ExternalTrackerURL)
		return
	}
}

// MustAllowPulls check if repository enable pull requests and user have right to do that
func ConversationMustAllowPulls(ctx *context.Context) {
	if !ctx.Repo.Repository.CanEnablePulls() || !ctx.Repo.CanRead(unit.TypePullRequests) {
		ctx.NotFound("MustAllowPulls", nil)
		return
	}

	// User can send pull request if owns a forked repository.
	if ctx.IsSigned && repo_model.HasForkedRepo(ctx, ctx.Doer.ID, ctx.Repo.Repository.ID) {
		ctx.Repo.PullRequest.Allowed = true
		ctx.Repo.PullRequest.HeadInfoSubURL = url.PathEscape(ctx.Doer.Name) + ":" + util.PathEscapeSegments(ctx.Repo.BranchName)
	}
}

// Conversations render conversations page
func Conversations(ctx *context.Context) {
	if ctx.Written() {
		return
	}

	ctx.Data["CanWriteConversations"] = ctx.Repo.CanWriteConversations()

	ctx.HTML(http.StatusOK, tplConversations)
}

// NewConversation render creating conversation page
func NewConversation(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.conversations.new")
	ctx.Data["PageIsConversationList"] = true
	ctx.Data["NewConversationChooseTemplate"] = false
	ctx.Data["PullRequestWorkInProgressPrefixes"] = setting.Repository.PullRequest.WorkInProgressPrefixes
	title := ctx.FormString("title")
	ctx.Data["TitleQuery"] = title
	body := ctx.FormString("body")
	ctx.Data["BodyQuery"] = body

	isProjectsEnabled := ctx.Repo.CanRead(unit.TypeProjects)
	ctx.Data["IsProjectsEnabled"] = isProjectsEnabled
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "comment")

	projectID := ctx.FormInt64("project")
	if projectID > 0 && isProjectsEnabled {
		project, err := project_model.GetProjectByID(ctx, projectID)
		if err != nil {
			log.Error("GetProjectByID: %d: %v", projectID, err)
		} else if project.RepoID != ctx.Repo.Repository.ID {
			log.Error("GetProjectByID: %d: %v", projectID, fmt.Errorf("project[%d] not in repo [%d]", project.ID, ctx.Repo.Repository.ID))
		} else {
			ctx.Data["project_id"] = projectID
			ctx.Data["Project"] = project
		}

		if len(ctx.Req.URL.Query().Get("project")) > 0 {
			ctx.Data["redirect_after_creation"] = "project"
		}
	}

	RetrieveRepoMetas(ctx, ctx.Repo.Repository, false)

	tags, err := repo_model.GetTagNamesByRepoID(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("GetTagNamesByRepoID", err)
		return
	}
	ctx.Data["Tags"] = tags

	ctx.Data["HasConversationsWritePermission"] = ctx.Repo.CanWrite(unit.TypeConversations)

	ctx.HTML(http.StatusOK, tplConversationNew)
}

// DeleteConversation deletes an conversation
func DeleteConversation(ctx *context.Context) {
	conversation := GetActionConversation(ctx)
	if ctx.Written() {
		return
	}

	if err := conversation_service.DeleteConversation(ctx, ctx.Doer, ctx.Repo.GitRepo, conversation); err != nil {
		ctx.ServerError("DeleteConversationByID", err)
		return
	}

	ctx.Redirect(fmt.Sprintf("%s/conversations", ctx.Repo.Repository.Link()), http.StatusSeeOther)
}

// ViewConversation render conversation view page
func ViewConversation(ctx *context.Context) {
	if ctx.PathParam(":type") == "conversations" {
		// If conversation was requested we check if repo has external tracker and redirect
		extConversationUnit, err := ctx.Repo.Repository.GetUnit(ctx, unit.TypeExternalTracker)
		if err == nil && extConversationUnit != nil {
			if extConversationUnit.ExternalTrackerConfig().ExternalTrackerStyle == markup.IssueNameStyleNumeric || extConversationUnit.ExternalTrackerConfig().ExternalTrackerStyle == "" {
				metas := ctx.Repo.Repository.ComposeMetas(ctx)
				metas["index"] = ctx.PathParam(":index")
				res, err := vars.Expand(extConversationUnit.ExternalTrackerConfig().ExternalTrackerFormat, metas)
				if err != nil {
					log.Error("unable to expand template vars for conversation url. conversation: %s, err: %v", metas["index"], err)
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

	conversation, err := conversations_model.GetConversationByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64(":index"))
	if err != nil {
		if conversations_model.IsErrConversationNotExist(err) {
			ctx.NotFound("GetConversationByIndex", err)
		} else {
			ctx.ServerError("GetConversationByIndex", err)
		}
		return
	}
	if conversation.Repo == nil {
		conversation.Repo = ctx.Repo.Repository
	}

	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "comment")

	if err = conversation.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}

	repo := ctx.Repo.Repository

	if ctx.IsSigned {
		// Update conversation-user.
		if err = activities_model.SetConversationReadBy(ctx, conversation.ID, ctx.Doer.ID); err != nil {
			ctx.ServerError("ReadBy", err)
			return
		}
	}

	var (
		role                 conversations_model.RoleDescriptor
		ok                   bool
		marked               = make(map[int64]conversations_model.RoleDescriptor)
		comment              *conversations_model.ConversationComment
		participants         = make([]*user_model.User, 1, 10)
		latestCloseCommentID int64
	)

	// Check if the user can use the dependencies
	// ctx.Data["CanCreateConversationDependencies"] = ctx.Repo.CanCreateConversationDependencies(ctx, ctx.Doer, conversation.IsPull)

	// check if dependencies can be created across repositories
	ctx.Data["AllowCrossRepositoryDependencies"] = setting.Service.AllowCrossRepositoryDependencies

	if err := conversation.Comments.LoadAttachmentsByConversation(ctx); err != nil {
		ctx.ServerError("LoadAttachmentsByConversation", err)
		return
	}
	if err := conversation.Comments.LoadPosters(ctx); err != nil {
		ctx.ServerError("LoadPosters", err)
		return
	}

	for _, comment = range conversation.Comments {
		comment.Conversation = conversation

		if comment.Type == conversations_model.CommentTypeComment {
			comment.RenderedContent, err = markdown.RenderString(&markup.RenderContext{
				Links: markup.Links{
					Base: ctx.Repo.RepoLink,
				},
				Metas:   ctx.Repo.Repository.ComposeMetas(ctx),
				GitRepo: ctx.Repo.GitRepo,
				Repo:    ctx.Repo.Repository,
				Ctx:     ctx,
			}, comment.Content)
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

			comment.ShowRole, err = conversationRoleDescriptor(ctx, repo, comment.Poster, comment.HasOriginalAuthor())
			if err != nil {
				ctx.ServerError("roleDescriptor", err)
				return
			}
			marked[comment.PosterID] = comment.ShowRole
			participants = addParticipant(comment.Poster, participants)
		}
	}

	ctx.Data["LatestCloseCommentID"] = latestCloseCommentID

	ctx.Data["Participants"] = participants
	ctx.Data["NumParticipants"] = len(participants)
	ctx.Data["Conversation"] = conversation
	ctx.Data["IsConversation"] = true
	ctx.Data["Comments"] = conversation.Comments
	ctx.Data["SignInLink"] = setting.AppSubURL + "/user/login?redirect_to=" + url.QueryEscape(ctx.Data["Link"].(string))
	ctx.Data["HasConversationsOrPullsWritePermission"] = ctx.Repo.CanWriteConversations()
	ctx.Data["HasProjectsWritePermission"] = ctx.Repo.CanWrite(unit.TypeProjects)
	ctx.Data["IsRepoAdmin"] = ctx.IsSigned && (ctx.Repo.IsAdmin() || ctx.Doer.IsAdmin)
	ctx.Data["LockReasons"] = setting.Repository.Conversation.LockReasons

	var hiddenCommentTypes *big.Int
	if ctx.IsSigned {
		val, err := user_model.GetUserSetting(ctx, ctx.Doer.ID, user_model.SettingsKeyHiddenCommentTypes)
		if err != nil {
			ctx.ServerError("GetUserSetting", err)
			return
		}
		hiddenCommentTypes, _ = new(big.Int).SetString(val, 10) // we can safely ignore the failed conversion here
	}
	ctx.Data["ShouldShowCommentType"] = func(commentType conversations_model.CommentType) bool {
		return hiddenCommentTypes == nil || hiddenCommentTypes.Bit(int(commentType)) == 0
	}
	// For sidebar
	PrepareBranchList(ctx)

	if ctx.Written() {
		return
	}

	tags, err := repo_model.GetTagNamesByRepoID(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("GetTagNamesByRepoID", err)
		return
	}
	ctx.Data["Tags"] = tags

	ctx.Data["CanBlockUser"] = func(blocker, blockee *user_model.User) bool {
		return user_service.CanBlockUser(ctx, ctx.Doer, blocker, blockee)
	}

	ctx.HTML(http.StatusOK, tplConversationView)
}

// GetActionConversation will return the conversation which is used in the context.
func GetActionConversation(ctx *context.Context) *conversations_model.Conversation {
	conversation, err := conversations_model.GetConversationByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64(":index"))
	if err != nil {
		ctx.NotFoundOrServerError("GetConversationByIndex", conversations_model.IsErrConversationNotExist, err)
		return nil
	}
	conversation.Repo = ctx.Repo.Repository
	checkConversationRights(ctx)
	if ctx.Written() {
		return nil
	}
	if err = conversation.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return nil
	}
	return conversation
}

func checkConversationRights(ctx *context.Context) {
	if !ctx.Repo.CanRead(unit.TypeConversations) {
		ctx.NotFound("ConversationUnitNotAllowed", nil)
	}
}

func getActionConversations(ctx *context.Context) conversations_model.ConversationList {
	commaSeparatedConversationIDs := ctx.FormString("conversation_ids")
	if len(commaSeparatedConversationIDs) == 0 {
		return nil
	}
	conversationIDs := make([]int64, 0, 10)
	for _, stringConversationID := range strings.Split(commaSeparatedConversationIDs, ",") {
		conversationID, err := strconv.ParseInt(stringConversationID, 10, 64)
		if err != nil {
			ctx.ServerError("ParseInt", err)
			return nil
		}
		conversationIDs = append(conversationIDs, conversationID)
	}
	conversations, err := conversations_model.GetConversationsByIDs(ctx, conversationIDs)
	if err != nil {
		ctx.ServerError("GetConversationsByIDs", err)
		return nil
	}
	// Check access rights for all conversations
	conversationUnitEnabled := ctx.Repo.CanRead(unit.TypeConversations)
	for _, conversation := range conversations {
		if conversation.RepoID != ctx.Repo.Repository.ID {
			ctx.NotFound("some conversation's RepoID is incorrect", errors.New("some conversation's RepoID is incorrect"))
			return nil
		}
		if !conversationUnitEnabled {
			ctx.NotFound("ConversationUnitNotAllowed", nil)
			return nil
		}
		if err = conversation.LoadAttributes(ctx); err != nil {
			ctx.ServerError("LoadAttributes", err)
			return nil
		}
	}
	return conversations
}

// GetConversationInfo get an conversation of a repository
func GetConversationInfo(ctx *context.Context) {
	conversation, err := conversations_model.GetConversationWithAttrsByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64(":index"))
	if err != nil {
		if conversations_model.IsErrConversationNotExist(err) {
			ctx.Error(http.StatusNotFound)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetConversationByIndex", err.Error())
		}
		return
	}

	// Need to check if Conversations are enabled and we can read Conversations
	if !ctx.Repo.CanRead(unit.TypeConversations) {
		ctx.Error(http.StatusNotFound)
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"convertedConversation": convert.ToConversation(ctx, conversation),
	})
}

// SearchConversations searches for conversations across the repositories that the user has access to
func SearchConversations(ctx *context.Context) {
	before, since, err := context.GetQueryBeforeSince(ctx.Base)
	if err != nil {
		ctx.Error(http.StatusUnprocessableEntity, err.Error())
		return
	}

	var isClosed optional.Option[bool]
	switch ctx.FormString("state") {
	case "closed":
		isClosed = optional.Some(true)
	case "all":
		isClosed = optional.None[bool]()
	default:
		isClosed = optional.Some(false)
	}

	repoIDs, allPublic := GetUserAccessibleRepo(ctx)

	keyword := ctx.FormTrim("q")
	if strings.IndexByte(keyword, 0) >= 0 {
		keyword = ""
	}

	// this api is also used in UI,
	// so the default limit is set to fit UI needs
	limit := ctx.FormInt("limit")
	if limit == 0 {
		limit = setting.UI.ConversationPagingNum
	} else if limit > setting.API.MaxResponseItems {
		limit = setting.API.MaxResponseItems
	}

	searchOpt := &conversation_indexer.SearchOptions{
		Paginator: &db.ListOptions{
			Page:     ctx.FormInt("page"),
			PageSize: limit,
		},
		Keyword:   keyword,
		RepoIDs:   repoIDs,
		AllPublic: allPublic,
		IsClosed:  isClosed,
		SortBy:    conversation_indexer.SortByCreatedDesc,
	}

	if since != 0 {
		searchOpt.UpdatedAfterUnix = optional.Some(since)
	}
	if before != 0 {
		searchOpt.UpdatedBeforeUnix = optional.Some(before)
	}

	if ctx.IsSigned {
		ctxUserID := ctx.Doer.ID
		if ctx.FormBool("created") {
			searchOpt.PosterID = optional.Some(ctxUserID)
		}
		if ctx.FormBool("assigned") {
			searchOpt.AssigneeID = optional.Some(ctxUserID)
		}
		if ctx.FormBool("mentioned") {
			searchOpt.MentionID = optional.Some(ctxUserID)
		}
		if ctx.FormBool("review_requested") {
			searchOpt.ReviewRequestedID = optional.Some(ctxUserID)
		}
		if ctx.FormBool("reviewed") {
			searchOpt.ReviewedID = optional.Some(ctxUserID)
		}
	}

	// FIXME: It's unsupported to sort by priority repo when searching by indexer,
	//        it's indeed an regression, but I think it is worth to support filtering by indexer first.
	_ = ctx.FormInt64("priority_repo_id")

	ids, total, err := conversation_indexer.SearchConversations(ctx, searchOpt)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "SearchConversations", err.Error())
		return
	}
	conversations, err := conversations_model.GetConversationsByIDs(ctx, ids, true)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindConversationsByIDs", err.Error())
		return
	}

	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, convert.ToConversationList(ctx, ctx.Doer, conversations))
}

func GetUserAccessibleRepo(ctx *context.Context) ([]int64, bool) {
	var (
		repoIDs   []int64
		allPublic bool
	)
	// find repos user can access (for conversation search)
	opts := &repo_model.SearchRepoOptions{
		Private:     false,
		AllPublic:   true,
		TopicOnly:   false,
		Collaborate: optional.None[bool](),
		// This needs to be a column that is not nil in fixtures or
		// MySQL will return different results when sorting by null in some cases
		OrderBy: db.SearchOrderByAlphabetically,
		Actor:   ctx.Doer,
	}
	if ctx.IsSigned {
		opts.Private = true
		opts.AllLimited = true
	}
	if ctx.FormString("owner") != "" {
		owner, err := user_model.GetUserByName(ctx, ctx.FormString("owner"))
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				ctx.Error(http.StatusBadRequest, "Owner not found", err.Error())
			} else {
				ctx.Error(http.StatusInternalServerError, "GetUserByName", err.Error())
			}
			return nil, false
		}
		opts.OwnerID = owner.ID
		opts.AllLimited = false
		opts.AllPublic = false
		opts.Collaborate = optional.Some(false)
	}
	if ctx.FormString("team") != "" {
		if ctx.FormString("owner") == "" {
			ctx.Error(http.StatusBadRequest, "", "Owner organisation is required for filtering on team")
			return nil, false
		}
		team, err := organization.GetTeam(ctx, opts.OwnerID, ctx.FormString("team"))
		if err != nil {
			if organization.IsErrTeamNotExist(err) {
				ctx.Error(http.StatusBadRequest, "Team not found", err.Error())
			} else {
				ctx.Error(http.StatusInternalServerError, "GetUserByName", err.Error())
			}
			return nil, false
		}
		opts.TeamID = team.ID
	}

	if opts.AllPublic {
		allPublic = true
		opts.AllPublic = false // set it false to avoid returning too many repos, we could filter by indexer
	}
	repoIDs, _, err := repo_model.SearchRepositoryIDs(ctx, opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "SearchRepositoryIDs", err.Error())
		return nil, false
	}
	if len(repoIDs) == 0 {
		// no repos found, don't let the indexer return all repos
		repoIDs = []int64{0}
	}
	return repoIDs, allPublic
}

// ListConversations list the conversations of a repository
func ListConversations(ctx *context.Context) {
	before, since, err := context.GetQueryBeforeSince(ctx.Base)
	if err != nil {
		ctx.Error(http.StatusUnprocessableEntity, err.Error())
		return
	}

	var isClosed optional.Option[bool]
	switch ctx.FormString("state") {
	case "closed":
		isClosed = optional.Some(true)
	case "all":
		isClosed = optional.None[bool]()
	default:
		isClosed = optional.Some(false)
	}

	keyword := ctx.FormTrim("q")
	if strings.IndexByte(keyword, 0) >= 0 {
		keyword = ""
	}

	projectID := optional.None[int64]()
	if v := ctx.FormInt64("project"); v > 0 {
		projectID = optional.Some(v)
	}

	isPull := optional.None[bool]()
	switch ctx.FormString("type") {
	case "conversations":
		isPull = optional.Some(false)
	}

	// FIXME: we should be more efficient here
	createdByID := getUserIDForFilter(ctx, "created_by")
	if ctx.Written() {
		return
	}
	assignedByID := getUserIDForFilter(ctx, "assigned_by")
	if ctx.Written() {
		return
	}
	mentionedByID := getUserIDForFilter(ctx, "mentioned_by")
	if ctx.Written() {
		return
	}

	searchOpt := &conversation_indexer.SearchOptions{
		Paginator: &db.ListOptions{
			Page:     ctx.FormInt("page"),
			PageSize: convert.ToCorrectPageSize(ctx.FormInt("limit")),
		},
		Keyword:   keyword,
		RepoIDs:   []int64{ctx.Repo.Repository.ID},
		IsPull:    isPull,
		IsClosed:  isClosed,
		ProjectID: projectID,
		SortBy:    conversation_indexer.SortByCreatedDesc,
	}
	if since != 0 {
		searchOpt.UpdatedAfterUnix = optional.Some(since)
	}
	if before != 0 {
		searchOpt.UpdatedBeforeUnix = optional.Some(before)
	}
	if createdByID > 0 {
		searchOpt.PosterID = optional.Some(createdByID)
	}
	if assignedByID > 0 {
		searchOpt.AssigneeID = optional.Some(assignedByID)
	}
	if mentionedByID > 0 {
		searchOpt.MentionID = optional.Some(mentionedByID)
	}

	ids, total, err := conversation_indexer.SearchConversations(ctx, searchOpt)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "SearchConversations", err.Error())
		return
	}
	conversations, err := conversations_model.GetConversationsByIDs(ctx, ids, true)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindConversationsByIDs", err.Error())
		return
	}

	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, convert.ToConversationList(ctx, ctx.Doer, conversations))
}

func BatchDeleteConversations(ctx *context.Context) {
	conversations := getActionConversations(ctx)
	if ctx.Written() {
		return
	}
	for _, conversation := range conversations {
		if err := conversation_service.DeleteConversation(ctx, ctx.Doer, ctx.Repo.GitRepo, conversation); err != nil {
			ctx.ServerError("DeleteConversation", err)
			return
		}
	}
	ctx.JSONOK()
}

// NewComment create a comment for conversation
func NewConversationComment(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateConversationCommentForm)
	conversation := GetActionConversation(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (!ctx.Repo.CanReadConversations()) {
		if log.IsTrace() {
			if ctx.IsSigned {
				conversationType := "conversations"
				log.Trace("Permission Denied: User %-v not the Poster (ID: %d) and cannot read %s in Repo %-v.\n"+
					"User in Repo has Permissions: %-+v",
					ctx.Doer,
					conversationType,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			} else {
				log.Trace("Permission Denied: Not logged in")
			}
		}

		ctx.Error(http.StatusForbidden)
		return
	}

	if conversation.IsLocked && !ctx.Repo.CanWriteConversations() && !ctx.Doer.IsAdmin {
		ctx.JSONError(ctx.Tr("repo.conversations.comment_on_locked"))
		return
	}

	var attachments []string
	if setting.Attachment.Enabled {
		attachments = form.Files
	}

	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	var comment *conversations_model.ConversationComment
	defer func() {
		// Redirect to comment hashtag if there is any actual content.
		typeName := "commit"
		if comment != nil {
			ctx.JSONRedirect(fmt.Sprintf("%s/%s/%s#%s", ctx.Repo.RepoLink, typeName, conversation.CommitSha, comment.HashTag()))
		} else {
			ctx.JSONRedirect(fmt.Sprintf("%s/%s/%s", ctx.Repo.RepoLink, typeName, conversation.CommitSha))
		}
	}()

	// Fix #321: Allow empty comments, as long as we have attachments.
	if len(form.Content) == 0 && len(attachments) == 0 {
		return
	}

	comment, err := conversation_service.CreateConversationComment(ctx, ctx.Doer, ctx.Repo.Repository, conversation, form.Content, attachments)
	if err != nil {
		if errors.Is(err, user_model.ErrBlockedUser) {
			ctx.JSONError(ctx.Tr("repo.conversations.comment.blocked_user"))
		} else {
			ctx.ServerError("CreateConversationComment", err)
		}
		return
	}

	log.Trace("Comment created: %d/%d/%d", ctx.Repo.Repository.ID, conversation.ID, comment.ID)
}

// UpdateCommentContent change comment of conversation's content
func UpdateConversationCommentContent(ctx *context.Context) {
	comment, err := conversations_model.GetCommentByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommentByID", conversations_model.IsErrCommentNotExist, err)
		return
	}

	if err := comment.LoadConversation(ctx); err != nil {
		ctx.NotFoundOrServerError("LoadConversation", conversations_model.IsErrConversationNotExist, err)
		return
	}

	if comment.Conversation.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound("CompareRepoID", conversations_model.ErrCommentNotExist{})
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != comment.PosterID && !ctx.Repo.CanWriteConversations()) {
		ctx.Error(http.StatusForbidden)
		return
	}

	if !comment.Type.HasContentSupport() {
		ctx.Error(http.StatusNoContent)
		return
	}

	oldContent := comment.Content
	newContent := ctx.FormString("content")
	contentVersion := ctx.FormInt("content_version")

	// allow to save empty content
	comment.Content = newContent
	if err = conversation_service.UpdateComment(ctx, comment, contentVersion, ctx.Doer, oldContent); err != nil {
		if errors.Is(err, user_model.ErrBlockedUser) {
			ctx.JSONError(ctx.Tr("repo.conversations.comment.blocked_user"))
		} else if errors.Is(err, conversations_model.ErrCommentAlreadyChanged) {
			ctx.JSONError(ctx.Tr("repo.comments.edit.already_changed"))
		} else {
			ctx.ServerError("UpdateComment", err)
		}
		return
	}

	if err := comment.LoadAttachments(ctx); err != nil {
		ctx.ServerError("LoadAttachments", err)
		return
	}

	// when the update request doesn't intend to update attachments (eg: change checkbox state), ignore attachment updates
	if !ctx.FormBool("ignore_attachments") {
		if err := updateConversationAttachments(ctx, comment, ctx.FormStrings("files[]")); err != nil {
			ctx.ServerError("UpdateAttachments", err)
			return
		}
	}

	var renderedContent template.HTML
	if comment.Content != "" {
		renderedContent, err = markdown.RenderString(&markup.RenderContext{
			Links: markup.Links{
				Base: ctx.FormString("context"), // FIXME: <- IS THIS SAFE ?
			},
			Metas:   ctx.Repo.Repository.ComposeMetas(ctx),
			GitRepo: ctx.Repo.GitRepo,
			Repo:    ctx.Repo.Repository,
			Ctx:     ctx,
		}, comment.Content)
		if err != nil {
			ctx.ServerError("RenderString", err)
			return
		}
	} else {
		contentEmpty := fmt.Sprintf(`<span class="no-content">%s</span>`, ctx.Tr("repo.conversations.no_content"))
		renderedContent = template.HTML(contentEmpty)
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"content":        renderedContent,
		"contentVersion": comment.ContentVersion,
		"attachments":    attachmentsHTML(ctx, comment.Attachments, comment.Content),
	})
}

// DeleteComment delete comment of conversation
func DeleteConversationComment(ctx *context.Context) {
	comment, err := conversations_model.GetCommentByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommentByID", conversations_model.IsErrCommentNotExist, err)
		return
	}

	if err := comment.LoadConversation(ctx); err != nil {
		ctx.NotFoundOrServerError("LoadConversation", conversations_model.IsErrConversationNotExist, err)
		return
	}

	if comment.Conversation.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound("CompareRepoID", conversations_model.ErrCommentNotExist{})
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != comment.PosterID && !ctx.Repo.CanWriteConversations()) {
		ctx.Error(http.StatusForbidden)
		return
	} else if !comment.Type.HasContentSupport() {
		ctx.Error(http.StatusNoContent)
		return
	}

	if err = conversation_service.DeleteComment(ctx, ctx.Doer, comment); err != nil {
		ctx.ServerError("DeleteComment", err)
		return
	}

	ctx.Status(http.StatusOK)
}

// ChangeCommentReaction create a reaction for comment
func ChangeConversationCommentReaction(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.ReactionForm)
	conversation_comment, err := conversations_model.GetCommentByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetConversationCommentByID", conversations_model.IsErrCommentNotExist, err)
		return
	}

	if err := conversation_comment.LoadConversation(ctx); err != nil {
		ctx.NotFoundOrServerError("LoadConversation", conversations_model.IsErrConversationNotExist, err)
		return
	}

	if conversation_comment.Conversation.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound("CompareRepoID", conversations_model.ErrCommentNotExist{})
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != conversation_comment.PosterID && !ctx.Repo.CanReadConversations()) {
		if log.IsTrace() {
			if ctx.IsSigned {
				conversationType := "conversations"
				log.Trace("Permission Denied: User %-v cannot read %s in Repo %-v.\n"+
					"User in Repo has Permissions: %-+v",
					ctx.Doer,
					conversationType,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			} else {
				log.Trace("Permission Denied: Not logged in")
			}
		}

		ctx.Error(http.StatusForbidden)
		return
	}

	if !conversation_comment.Type.HasContentSupport() {
		ctx.Error(http.StatusNoContent)
		return
	}

	switch ctx.PathParam(":action") {
	case "react":
		reaction, err := conversation_service.CreateCommentReaction(ctx, ctx.Doer, conversation_comment, form.Content)
		if err != nil {
			if conversations_model.IsErrForbiddenConversationReaction(err) || errors.Is(err, user_model.ErrBlockedUser) {
				ctx.ServerError("ChangeConversationReaction", err)
				return
			}
			log.Info("CreateConversationCommentReaction: %s", err)
			break
		}
		// Reload new reactions
		conversation_comment.Reactions = nil
		if err = conversation_comment.LoadReactions(ctx, ctx.Repo.Repository); err != nil {
			log.Info("conversation_comment.LoadReactions: %s", err)
			break
		}

		log.Trace("Reaction for comment created: %d/%d/%d/%d", ctx.Repo.Repository.ID, conversation_comment.Conversation.ID, conversation_comment.ID, reaction.ID)
	case "unreact":
		if err := conversations_model.DeleteCommentReaction(ctx, ctx.Doer.ID, conversation_comment.Conversation.ID, conversation_comment.ID, form.Content); err != nil {
			ctx.ServerError("DeleteConversationCommentReaction", err)
			return
		}

		// Reload new reactions
		conversation_comment.Reactions = nil
		if err = conversation_comment.LoadReactions(ctx, ctx.Repo.Repository); err != nil {
			log.Info("comment.LoadReactions: %s", err)
			break
		}

		log.Trace("Reaction for conversation comment removed: %d/%d/%d", ctx.Repo.Repository.ID, conversation_comment.Conversation.ID, conversation_comment.ID)
	default:
		ctx.NotFound(fmt.Sprintf("Unknown action %s", ctx.PathParam(":action")), nil)
		return
	}

	if len(conversation_comment.Reactions) == 0 {
		ctx.JSON(http.StatusOK, map[string]any{
			"empty": true,
			"html":  "",
		})
		return
	}

	html, err := ctx.RenderToHTML(tplReactions, map[string]any{
		"ActionURL": fmt.Sprintf("%s/conversations/comments/%d/reactions", ctx.Repo.RepoLink, conversation_comment.ID),
		"Reactions": conversation_comment.Reactions.GroupByType(),
	})
	if err != nil {
		ctx.ServerError("ChangeCommentReaction.HTMLString", err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{
		"html": html,
	})
}

// GetCommentAttachments returns attachments for the comment
func GetConversationCommentAttachments(ctx *context.Context) {
	comment, err := conversations_model.GetCommentByID(ctx, ctx.PathParamInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommentByID", conversations_model.IsErrCommentNotExist, err)
		return
	}

	if err := comment.LoadConversation(ctx); err != nil {
		ctx.NotFoundOrServerError("LoadConversation", conversations_model.IsErrConversationNotExist, err)
		return
	}

	if comment.Conversation.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound("CompareRepoID", conversations_model.ErrCommentNotExist{})
		return
	}

	if !ctx.Repo.Permission.CanReadConversations() {
		ctx.NotFound("CanReadConversationsOrPulls", conversations_model.ErrCommentNotExist{})
		return
	}

	if !comment.Type.HasAttachmentSupport() {
		ctx.ServerError("GetCommentAttachments", fmt.Errorf("comment type %v does not support attachments", comment.Type))
		return
	}

	attachments := make([]*api.Attachment, 0)
	if err := comment.LoadAttachments(ctx); err != nil {
		ctx.ServerError("LoadAttachments", err)
		return
	}
	for i := 0; i < len(comment.Attachments); i++ {
		attachments = append(attachments, convert.ToAttachment(ctx.Repo.Repository, comment.Attachments[i]))
	}
	ctx.JSON(http.StatusOK, attachments)
}

func updateConversationAttachments(ctx *context.Context, item any, files []string) error {
	var attachments []*repo_model.Attachment
	switch content := item.(type) {
	case *conversations_model.ConversationComment:
		attachments = content.Attachments
	default:
		return fmt.Errorf("unknown Type: %T", content)
	}
	for i := 0; i < len(attachments); i++ {
		if util.SliceContainsString(files, attachments[i].UUID) {
			continue
		}
		if err := repo_model.DeleteAttachment(ctx, attachments[i], true); err != nil {
			return err
		}
	}
	var err error
	if len(files) > 0 {
		switch content := item.(type) {
		case *conversations_model.Conversation:
			err = conversations_model.UpdateConversationAttachments(ctx, content.ID, files)
		case *conversations_model.ConversationComment:
			err = content.UpdateAttachments(ctx, files)
		default:
			return fmt.Errorf("unknown Type: %T", content)
		}
		if err != nil {
			return err
		}
	}
	switch content := item.(type) {
	case *conversations_model.ConversationComment:
		content.Attachments, err = repo_model.GetAttachmentsByCommentID(ctx, content.ID)
	default:
		return fmt.Errorf("unknown Type: %T", content)
	}
	return err
}

// roleDescriptor returns the role descriptor for a comment in/with the given repo, poster and conversation
func conversationRoleDescriptor(ctx *context.Context, repo *repo_model.Repository, poster *user_model.User, hasOriginalAuthor bool) (conversations_model.RoleDescriptor, error) {
	roleDescriptor := conversations_model.RoleDescriptor{}

	if hasOriginalAuthor {
		return roleDescriptor, nil
	}

	perm, err := access_model.GetUserRepoPermission(ctx, repo, poster)
	if err != nil {
		return roleDescriptor, err
	}

	// If the poster is the actual poster of the conversation, enable Poster role.
	roleDescriptor.IsPoster = false

	// Check if the poster is owner of the repo.
	if perm.IsOwner() {
		// If the poster isn't an admin, enable the owner role.
		if !poster.IsAdmin {
			roleDescriptor.RoleInRepo = conversations_model.RoleRepoOwner
			return roleDescriptor, nil
		}

		// Otherwise check if poster is the real repo admin.
		ok, err := access_model.IsUserRealRepoAdmin(ctx, repo, poster)
		if err != nil {
			return roleDescriptor, err
		}
		if ok {
			roleDescriptor.RoleInRepo = conversations_model.RoleRepoOwner
			return roleDescriptor, nil
		}
	}

	// If repo is organization, check Member role
	if err := repo.LoadOwner(ctx); err != nil {
		return roleDescriptor, err
	}
	if repo.Owner.IsOrganization() {
		if isMember, err := organization.IsOrganizationMember(ctx, repo.Owner.ID, poster.ID); err != nil {
			return roleDescriptor, err
		} else if isMember {
			roleDescriptor.RoleInRepo = conversations_model.RoleRepoMember
			return roleDescriptor, nil
		}
	}

	// If the poster is the collaborator of the repo
	if isCollaborator, err := repo_model.IsCollaborator(ctx, repo.ID, poster.ID); err != nil {
		return roleDescriptor, err
	} else if isCollaborator {
		roleDescriptor.RoleInRepo = conversations_model.RoleRepoCollaborator
		return roleDescriptor, nil
	}

	return roleDescriptor, nil
}
