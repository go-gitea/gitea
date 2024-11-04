// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conversations

import (
	"context"
	"errors"
	"fmt"

	conversation_model "code.gitea.io/gitea/models/conversations"
	"code.gitea.io/gitea/modules/indexer/conversations/internal"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
)

// getConversationIndexerData returns the indexer data of an conversation and a bool value indicating whether the conversation exists.
func getConversationIndexerData(ctx context.Context, conversationID int64) (*internal.IndexerData, bool, error) {
	conversation, err := conversation_model.GetConversationByID(ctx, conversationID)
	if err != nil {
		if conversation_model.IsErrConversationNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	// FIXME: what if users want to search for a review comment of a pull request?
	//        The comment type is CommentTypeCode or CommentTypeReview.
	//        But LoadDiscussComments only loads CommentTypeComment.
	if err := conversation.LoadDiscussComments(ctx); err != nil {
		return nil, false, err
	}

	comments := make([]string, 0, len(conversation.Comments))
	for _, comment := range conversation.Comments {
		if comment.Content != "" {
			// what ever the comment type is, index the content if it is not empty.
			comments = append(comments, comment.Content)
		}
	}

	if err := conversation.LoadAttributes(ctx); err != nil {
		return nil, false, err
	}

	return &internal.IndexerData{
		ID:           conversation.ID,
		RepoID:       conversation.RepoID,
		IsPublic:     !conversation.Repo.IsPrivate,
		Comments:     comments,
		UpdatedUnix:  conversation.UpdatedUnix,
		CreatedUnix:  conversation.CreatedUnix,
		CommentCount: int64(len(conversation.Comments)),
	}, true, nil
}

func updateRepoIndexer(ctx context.Context, repoID int64) error {
	ids, err := conversation_model.GetConversationIDsByRepoID(ctx, repoID)
	if err != nil {
		return fmt.Errorf("conversation_model.GetConversationIDsByRepoID: %w", err)
	}
	for _, id := range ids {
		if err := updateConversationIndexer(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func updateConversationIndexer(ctx context.Context, conversationID int64) error {
	return pushConversationIndexerQueue(ctx, &IndexerMetadata{ID: conversationID})
}

func deleteRepoConversationIndexer(ctx context.Context, repoID int64) error {
	var ids []int64
	ids, err := conversation_model.GetConversationIDsByRepoID(ctx, repoID)
	if err != nil {
		return fmt.Errorf("conversation_model.GetConversationIDsByRepoID: %w", err)
	}

	if len(ids) == 0 {
		return nil
	}
	return pushConversationIndexerQueue(ctx, &IndexerMetadata{
		IDs:      ids,
		IsDelete: true,
	})
}

type keepRetryKey struct{}

// contextWithKeepRetry returns a context with a key indicating that the indexer should keep retrying.
// Please note that it's for background tasks only, and it should not be used for user requests, or it may cause blocking.
func contextWithKeepRetry(ctx context.Context) context.Context {
	return context.WithValue(ctx, keepRetryKey{}, true)
}

func pushConversationIndexerQueue(ctx context.Context, data *IndexerMetadata) error {
	if conversationIndexerQueue == nil {
		// Some unit tests will trigger indexing, but the queue is not initialized.
		// It's OK to ignore it, but log a warning message in case it's not a unit test.
		log.Warn("Trying to push %+v to conversation indexer queue, but the queue is not initialized, it's OK if it's a unit test", data)
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		err := conversationIndexerQueue.Push(data)
		if errors.Is(err, queue.ErrAlreadyInQueue) {
			return nil
		}
		if errors.Is(err, context.DeadlineExceeded) { // the queue is full
			log.Warn("It seems that conversation indexer is slow and the queue is full. Please check the conversation indexer or increase the queue size.")
			if ctx.Value(keepRetryKey{}) == nil {
				return err
			}
			// It will be better to increase the queue size instead of retrying, but users may ignore the previous warning message.
			// However, even it retries, it may still cause index loss when there's a deadline in the context.
			log.Debug("Retry to push %+v to conversation indexer queue", data)
			continue
		}
		return err
	}
}
