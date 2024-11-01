package repo

import (
	"errors"

	conversations_model "code.gitea.io/gitea/models/conversations"
	issues_model "code.gitea.io/gitea/models/issues"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/services/context"
	conversation_service "code.gitea.io/gitea/services/conversation"
	"code.gitea.io/gitea/services/forms"
	issue_service "code.gitea.io/gitea/services/issue"
)

func AddReaction(ctx *context.Context, form *forms.ReactionForm, comment *conversations_model.ConversationComment, issue *issues_model.Issue) error {
	if issue != nil {
		reaction, err := issue_service.CreateIssueReaction(ctx, ctx.Doer, issue, form.Content)
		if err != nil {
			if issues_model.IsErrForbiddenIssueReaction(err) || errors.Is(err, user_model.ErrBlockedUser) {
				ctx.ServerError("ChangeIssueReaction", err)
				return err
			}
			log.Info("CreateIssueReaction: %s", err)
			return err
		}
		// Reload new reactions
		issue.Reactions = nil
		if err = issue.LoadAttributes(ctx); err != nil {
			log.Info("issue.LoadAttributes: %s", err)
			return err
		}

		log.Trace("Reaction for issue created: %d/%d/%d", ctx.Repo.Repository.ID, issue.ID, reaction.ID)
	} else if comment != nil {

		reaction, err := conversation_service.CreateCommentReaction(ctx, ctx.Doer, comment, form.Content)
		if err != nil {
			if conversations_model.IsErrForbiddenConversationReaction(err) || errors.Is(err, user_model.ErrBlockedUser) {
				ctx.ServerError("ChangeConversationReaction", err)
				return err
			}
			log.Info("CreateConversationCommentReaction: %s", err)
			return err
		}
		// Reload new reactions
		comment.Reactions = nil
		if err = comment.LoadReactions(ctx, ctx.Repo.Repository); err != nil {
			log.Info("comment.LoadReactions: %s", err)
			return err
		}

		log.Trace("Reaction for comment created: %d/%d/%d/%d", ctx.Repo.Repository.ID, comment.Conversation.ID, comment.ID, reaction.ID)
	}

	return nil
}

func RemoveReaction(ctx *context.Context, form *forms.ReactionForm, comment *conversations_model.ConversationComment, issue *issues_model.Issue) error {
	if issue != nil {
		if err := issues_model.DeleteIssueReaction(ctx, ctx.Doer.ID, issue.ID, form.Content); err != nil {
			ctx.ServerError("DeleteIssueReaction", err)
			return err
		}

		// Reload new reactions
		issue.Reactions = nil
		if err := issue.LoadAttributes(ctx); err != nil {
			log.Info("issue.LoadAttributes: %s", err)
			return err
		}

		log.Trace("Reaction for issue removed: %d/%d", ctx.Repo.Repository.ID, issue.ID)
	} else if comment != nil {
		if err := conversations_model.DeleteCommentReaction(ctx, ctx.Doer.ID, comment.Conversation.ID, comment.ID, form.Content); err != nil {
			ctx.ServerError("DeleteConversationCommentReaction", err)
			return err
		}

		// Reload new reactions
		comment.Reactions = nil
		if err := comment.LoadReactions(ctx, ctx.Repo.Repository); err != nil {
			log.Info("comment.LoadReactions: %s", err)
			return err
		}

		log.Trace("Reaction for conversation comment removed: %d/%d/%d", ctx.Repo.Repository.ID, comment.Conversation.ID, comment.ID)
	}
	return nil
}
