package rss

import (
	"fmt"
	"html"
	"net/url"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"

	"github.com/gorilla/feeds"
)

func FeedActionsToFeedItems(ctx *context.Context, actions []*models.Action) (items []*feeds.Item) {
	for _, act := range actions {
		act.LoadActUser()

		content, desc, title := "", "", ""

		link := &feeds.Link{Href: act.GetCommentLink()}

		title = tmpTypeName(act.OpType)

		// title
		title = act.ActUser.DisplayName() + " "
		{
			switch act.OpType {
			case models.ActionCreateRepo:
				title += ctx.Tr("action.create_repo", act.GetRepoLink(), act.ShortRepoPath())
			case models.ActionRenameRepo:
				title += ctx.Tr("action.rename_repo", act.GetContent(), act.GetRepoLink(), act.ShortRepoPath())
			case models.ActionCommitRepo:
				branchLink := act.GetBranch()
				if len(act.Content) != 0 {
					title += ctx.Tr("action.commit_repo", act.GetRepoLink(), branchLink, act.GetBranch(), act.ShortRepoPath())
				} else {
					title += ctx.Tr("action.create_branch", act.GetRepoLink(), branchLink, act.GetBranch(), act.ShortRepoPath())
				}
			case models.ActionCreateIssue:
				title += ctx.Tr("action.create_issue", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath())
			case models.ActionCreatePullRequest:
				title += ctx.Tr("action.create_pull_request", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath())
			case models.ActionTransferRepo:
				title += ctx.Tr("action.transfer_repo", act.GetContent(), act.GetRepoLink(), act.ShortRepoPath())
			case models.ActionPushTag:
				title += ctx.Tr("action.push_tag", act.GetRepoLink(), url.QueryEscape(act.GetTag()), act.ShortRepoPath())
			case models.ActionCommentIssue:
				title += ctx.Tr("action.comment_issue", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath())
			case models.ActionMergePullRequest:
				title += ctx.Tr("action.merge_pull_request", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath())
			case models.ActionCloseIssue:
				title += ctx.Tr("action.close_issue", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath())
			case models.ActionReopenIssue:
				title += ctx.Tr("action.reopen_issue", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath())
			case models.ActionClosePullRequest:
				title += ctx.Tr("action.close_pull_request", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath())
			case models.ActionReopenPullRequest:
				title += ctx.Tr("action.reopen_pull_request", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath)
			case models.ActionDeleteTag:
				title += ctx.Tr("action.delete_tag", act.GetRepoLink(), html.EscapeString(act.GetTag()), act.ShortRepoPath())
			case models.ActionDeleteBranch:
				title += ctx.Tr("action.delete_branch", act.GetRepoLink(), html.EscapeString(act.GetBranch()), act.ShortRepoPath())
			case models.ActionMirrorSyncPush:
				title += ctx.Tr("action.mirror_sync_push", act.GetRepoLink(), url.QueryEscape(act.GetBranch()), html.EscapeString(act.GetBranch()), act.ShortRepoPath())
			case models.ActionMirrorSyncCreate:
				title += ctx.Tr("action.mirror_sync_create", act.GetRepoLink(), html.EscapeString(act.GetBranch()), act.ShortRepoPath())
			case models.ActionMirrorSyncDelete:
				title += ctx.Tr("action.mirror_sync_delete", act.GetRepoLink(), html.EscapeString(act.GetBranch()), act.ShortRepoPath())
			case models.ActionApprovePullRequest:
				title += ctx.Tr("action.approve_pull_request", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath())
			case models.ActionRejectPullRequest:
				title += ctx.Tr("action.reject_pull_request", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath())
			case models.ActionCommentPull:
				title += ctx.Tr("action.comment_pull", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath())
			case models.ActionPublishRelease:
				title += ctx.Tr("action.publish_release", act.GetRepoLink(), html.EscapeString(act.GetBranch()), act.ShortRepoPath(), act.Content)
			case models.ActionPullReviewDismissed:
				title += ctx.Tr("action.review_dismissed", act.GetRepoLink(), act.GetIssueInfos()[0], act.ShortRepoPath(), act.GetIssueInfos()[1])
			}
		}

		// description & content
		{
			switch act.OpType {
			case models.ActionCommitRepo, models.ActionMirrorSyncPush:
				desc = `<div class="content">
							<ul>
								{{ $push := ActionContent2Commits .}}
								{{ $repoLink := .GetRepoLink}}
								{{if $push.Commits}}
									{{range $push.Commits}}
										{{ $commitLink := printf "%s/commit/%s" $repoLink .Sha1}}
										<li>
											{{avatarHTML ($push.AvatarLink .AuthorEmail) 16 "mr-2" .AuthorName}}
											<a class="commit-id mr-2" href="{{$commitLink}}">{{ShortSha .Sha1}}</a>
											<span class="text truncate light grey">
												{{RenderCommitMessage .Message $repoLink $.ComposeMetas}}
											</span>
										</li>
									{{end}}
								{{end}}
								{{if and (gt $push.Len 1) $push.CompareURL}}<li><a href="{{AppSubUrl}}/{{$push.CompareURL}}">{{$.i18n.Tr "action.compare_commits" $push.Len}} »</a></li>{{end}}
							</ul>
						</div>`

			case models.ActionCreateIssue, models.ActionCreatePullRequest:
				desc = strings.Join(act.GetIssueInfos(), "#") // index .GetIssueInfos 1 | RenderEmoji
				content = act.GetIssueContent()
			case models.ActionCommentIssue, models.ActionApprovePullRequest, models.ActionRejectPullRequest, models.ActionCommentPull:
				desc = act.GetIssueTitle()        // class="text truncate issue title" | RenderEmoji
				comment := act.GetIssueInfos()[1] // class="text light grey" | RenderEmoji
				if len(comment) != 0 {
					desc += "\n\n" + comment
				}
			case models.ActionMergePullRequest:
				desc = act.GetIssueInfos()[1] // class="text light grey"
			case models.ActionCloseIssue, models.ActionReopenIssue, models.ActionClosePullRequest, models.ActionReopenPullRequest:
				desc = `<span class="text truncate issue title">{{.GetIssueTitle | RenderEmoji}}</span>`
			case models.ActionPullReviewDismissed:
				desc = `<p class="text light grey">{{$.i18n.Tr "action.review_dismissed_reason"}}</p>
		<p class="text light grey">{{index .GetIssueInfos 2 | RenderEmoji}}</p>`
			}
		}
		if len(content) == 0 {
			content = desc
		}

		// img := templates.ActionIcon(act.OpType)

		items = append(items, &feeds.Item{
			Title:       title,
			Link:        link,
			Description: desc,
			Author:      feedsAuthor(act.ActUser),
			Id:          fmt.Sprint(act.ID),
			Created:     time.Now(), // Created:     act.CreatedUnix.AsTime(),
			Content:     content,
		})
	}
	return
}

func feedsAuthor(user *models.User) *feeds.Author {
	return &feeds.Author{
		Name:  user.DisplayName(),
		Email: user.GetEmail(),
	}
}

func tmpTypeName(t models.ActionType) string {
	switch t {
	case models.ActionCreateRepo:
		return "ActionCreateRepo"
	case models.ActionRenameRepo:
		return "ActionRenameRepo"
	case models.ActionStarRepo:
		return "ActionStarRepo"
	case models.ActionWatchRepo:
		return "ActionWatchRepo"
	case models.ActionCommitRepo:
		return "ActionCommitRepo"
	case models.ActionCreateIssue:
		return "ActionCreateIssue"
	case models.ActionCreatePullRequest:
		return "ActionCreatePullRequest"
	case models.ActionTransferRepo:
		return "ActionTransferRepo"
	case models.ActionPushTag:
		return "ActionPushTag"
	case models.ActionCommentIssue:
		return "ActionCommentIssue"
	case models.ActionMergePullRequest:
		return "ActionMergePullRequest"
	case models.ActionCloseIssue:
		return "ActionCloseIssue"
	case models.ActionReopenIssue:
		return "ActionReopenIssue"
	case models.ActionClosePullRequest:
		return "ActionClosePullRequest"
	case models.ActionReopenPullRequest:
		return "ActionReopenPullRequest"
	case models.ActionDeleteTag:
		return "ActionDeleteTag"
	case models.ActionDeleteBranch:
		return "ActionDeleteBranch"
	case models.ActionMirrorSyncPush:
		return "ActionMirrorSyncPush"
	case models.ActionMirrorSyncCreate:
		return "ActionMirrorSyncCreate"
	case models.ActionMirrorSyncDelete:
		return "ActionMirrorSyncDelete"
	case models.ActionApprovePullRequest:
		return "ActionApprovePullRequest"
	case models.ActionRejectPullRequest:
		return "ActionRejectPullRequest"
	case models.ActionCommentPull:
		return "ActionCommentPull"
	case models.ActionPublishRelease:
		return "ActionPublishRelease"
	case models.ActionPullReviewDismissed:
		return "ActionPullReviewDismissed"
	}
	return ""
}

/*

 */
