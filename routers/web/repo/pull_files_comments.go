// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"cmp"
	"html/template"
	"net/http"
	"slices"
	"strings"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
	user_service "gitea.dev/services/user"
)

type diffViewerUser struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	HomeLink    string `json:"homeLink,omitempty"`
	Avatar      string `json:"avatar"`
}

type diffViewerReaction struct {
	Content    string        `json:"content"`
	EmojiHTML  template.HTML `json:"emojiHtml"`
	Count      int           `json:"count"`
	Users      string        `json:"users"`
	MoreCount  int           `json:"moreCount"`
	HasReacted bool          `json:"hasReacted"`
}

type diffViewerAttachment struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"downloadUrl"`
	IsImage     bool   `json:"isImage"`
	InContent   bool   `json:"inContent"` // the image is already shown by the rendered content
}

type diffViewerComment struct {
	ID             int64                  `json:"id"`
	Path           string                 `json:"path"`
	Side           string                 `json:"side"` // "new" or "old"
	Line           int                    `json:"line"`
	Poster         diffViewerUser         `json:"poster"`
	OriginalAuthor string                 `json:"originalAuthor,omitempty"`
	Body           string                 `json:"body"`
	BodyHTML       string                 `json:"bodyHtml"`
	ContentVersion int                    `json:"contentVersion"`
	Created        int64                  `json:"created"`
	Outdated       bool                   `json:"outdated"`
	Pending        bool                   `json:"pending"`
	ReviewID       int64                  `json:"reviewId"`
	Resolved       bool                   `json:"resolved"`
	ResolvedBy     string                 `json:"resolvedBy"`
	CanEdit        bool                   `json:"canEdit"`
	CanBlockUser   bool                   `json:"canBlockUser"`
	CanBlockOrg    bool                   `json:"canBlockOrg"`
	Reactions      []diffViewerReaction   `json:"reactions"`
	Attachments    []diffViewerAttachment `json:"attachments"`
}

type diffViewerReactionType struct {
	Content   string        `json:"content"`
	EmojiHTML template.HTML `json:"emojiHtml"`
}

type diffViewerComments struct {
	IsSigned          bool                     `json:"isSigned"`
	CanComment        bool                     `json:"canComment"`
	CanResolve        bool                     `json:"canResolve"`
	CanReferenceIssue bool                     `json:"canReferenceIssue"`
	HasPendingReview  bool                     `json:"hasPendingReview"`
	PendingCount      int64                    `json:"pendingCount"`
	IssueHTMLURL      string                   `json:"issueHtmlUrl"`
	RepoLink          string                   `json:"repoLink"`
	OrgBlockLink      string                   `json:"orgBlockLink,omitempty"`
	OriginalURL       string                   `json:"originalUrl,omitempty"`
	OriginalHostname  string                   `json:"originalHostname,omitempty"`
	MigrationIcon     string                   `json:"migrationIcon,omitempty"`
	Reactions         []diffViewerReactionType `json:"reactions"`
	Comments          []diffViewerComment      `json:"comments"`
}

func diffViewerUserOf(ctx *context.Context, user *user_model.User) diffViewerUser {
	if user == nil {
		user = user_model.NewGhostUser()
	}
	result := diffViewerUser{Name: user.Name, DisplayName: user.GetDisplayName(), Avatar: user.AvatarLinkWithSize(ctx, 40)}
	if user.ID > 0 {
		result.HomeLink = user.HomeLink()
	}
	return result
}

// PullFilesComments serves the code comments of a pull request for the diff viewer
func PullFilesComments(ctx *context.Context) {
	issue, ok := getPullInfo(ctx)
	if !ok {
		return
	}
	repo := ctx.Repo.Repository

	codeComments, err := issues_model.FetchCodeComments(ctx, issue, ctx.Doer, GetShowOutdatedComments(ctx))
	if err != nil {
		ctx.ServerError("FetchCodeComments", err)
		return
	}

	canComment := ctx.IsSigned && !repo.IsArchived
	canWrite := ctx.Repo.Permission.IsAdmin() || ctx.Repo.Permission.CanWriteIssuesOrPulls(true)
	data := diffViewerComments{
		IsSigned:          ctx.IsSigned,
		CanComment:        canComment,
		CanReferenceIssue: canComment && !unit.TypeIssues.UnitGlobalDisabled(),
		IssueHTMLURL:      issue.HTMLURL(ctx),
		RepoLink:          ctx.Repo.RepoLink,
		Comments:          make([]diffViewerComment, 0),
	}
	if repo.OriginalURL != "" {
		data.OriginalURL = repo.OriginalURL
		data.OriginalHostname = repo.GetOriginalURLHostname()
		data.MigrationIcon = templates.MigrationIcon(data.OriginalHostname)
	}
	if err := repo.LoadOwner(ctx); err != nil {
		ctx.ServerError("LoadOwner", err)
		return
	}
	if repo.Owner.IsOrganization() {
		data.OrgBlockLink = repo.Owner.OrganisationLink() + "/settings/blocked_users"
	}
	for _, reaction := range setting.UI.Reactions {
		data.Reactions = append(data.Reactions, diffViewerReactionType{Content: reaction, EmojiHTML: templates.ReactionToEmoji(reaction)})
	}

	type blockState struct{ user, org bool }
	blockStates := map[int64]blockState{} // the checks query the database, and posters repeat
	for path, lineComments := range codeComments {
		for line, list := range lineComments {
			side := util.Iif(line < 0, "old", "new")
			for _, comment := range list {
				c := toDiffViewerComment(ctx, comment, path, side, int(max(line, -line)), canWrite)
				if ctx.IsSigned && comment.Poster != nil {
					state, ok := blockStates[comment.PosterID]
					if !ok {
						state = blockState{
							user: user_service.CanBlockUser(ctx, ctx.Doer, ctx.Doer, comment.Poster),
							org:  repo.Owner.IsOrganization() && user_service.CanBlockUser(ctx, ctx.Doer, repo.Owner, comment.Poster),
						}
						blockStates[comment.PosterID] = state
					}
					c.CanBlockUser, c.CanBlockOrg = state.user, state.org
				}
				data.Comments = append(data.Comments, c)
			}
		}
	}

	if data.CanResolve, err = issues_model.CanMarkConversation(ctx, issue, ctx.Doer); err != nil {
		ctx.ServerError("CanMarkConversation", err)
		return
	}
	if data.HasPendingReview, data.PendingCount, err = getPendingCodeCommentCount(ctx, issue); err != nil {
		ctx.ServerError("getPendingCodeCommentCount", err)
		return
	}
	ctx.JSON(http.StatusOK, data)
}

// getPendingCodeCommentCount returns whether the doer has a pending review and how many code comments it has
func getPendingCodeCommentCount(ctx *context.Context, issue *issues_model.Issue) (hasPendingReview bool, count int64, err error) {
	review, err := issues_model.GetCurrentReview(ctx, ctx.Doer, issue)
	if err != nil && !issues_model.IsErrReviewNotExist(err) {
		return false, 0, err
	}
	if review == nil { // also for anonymous users
		return false, 0, nil
	}
	count, err = issues_model.CountComments(ctx, &issues_model.FindCommentsOptions{Type: issues_model.CommentTypeCode, ReviewID: review.ID, IssueID: issue.ID})
	return true, count, err
}

func toDiffViewerComment(ctx *context.Context, comment *issues_model.Comment, path, side string, line int, canWrite bool) diffViewerComment {
	c := diffViewerComment{
		ID:             comment.ID,
		Path:           path,
		Side:           side,
		Line:           line,
		Poster:         diffViewerUserOf(ctx, comment.Poster),
		OriginalAuthor: comment.OriginalAuthor,
		Body:           comment.Content,
		BodyHTML:       string(comment.RenderedContent),
		ContentVersion: comment.ContentVersion,
		Created:        int64(comment.CreatedUnix),
		Outdated:       comment.Invalidated,
		Pending:        comment.Review != nil && comment.Review.Type == issues_model.ReviewTypePending,
		ReviewID:       comment.ReviewID,
		Resolved:       comment.IsResolved(),
		Reactions:      make([]diffViewerReaction, 0),
		Attachments:    make([]diffViewerAttachment, 0),
	}
	isPoster := ctx.IsSigned && ctx.Doer.ID == comment.PosterID
	c.CanEdit = ctx.IsSigned && !ctx.Repo.Repository.IsArchived && (canWrite || isPoster)
	// the poster, resolver, reactions and attachments are loaded by FetchCodeComments
	if c.Resolved && comment.ResolveDoer != nil {
		c.ResolvedBy = comment.ResolveDoer.Name
	}
	for content, reactions := range comment.Reactions.GroupByType() {
		c.Reactions = append(c.Reactions, diffViewerReaction{
			Content:    content,
			EmojiHTML:  templates.ReactionToEmoji(content),
			Count:      len(reactions),
			Users:      reactions.GetFirstUsers(),
			MoreCount:  reactions.GetMoreUserCount(),
			HasReacted: ctx.IsSigned && reactions.HasUser(ctx.Doer.ID),
		})
	}
	slices.SortFunc(c.Reactions, func(a, b diffViewerReaction) int { return cmp.Compare(a.Content, b.Content) })
	for _, attachment := range comment.Attachments {
		c.Attachments = append(c.Attachments, diffViewerAttachment{
			Name:        attachment.Name,
			Size:        attachment.Size,
			DownloadURL: attachment.DownloadURL(ctx),
			IsImage:     templates.FilenameIsImage(attachment.Name),
			InContent:   strings.Contains(string(comment.RenderedContent), attachment.UUID),
		})
	}
	return c
}
