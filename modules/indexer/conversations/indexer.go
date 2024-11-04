// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package conversations

import (
	"context"
	"fmt"
	"os"
	"runtime/pprof"
	"sync/atomic"
	"time"

	db_model "code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/indexer/conversations/bleve"
	"code.gitea.io/gitea/modules/indexer/conversations/db"
	"code.gitea.io/gitea/modules/indexer/conversations/elasticsearch"
	"code.gitea.io/gitea/modules/indexer/conversations/internal"
	"code.gitea.io/gitea/modules/indexer/conversations/meilisearch"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
)

// IndexerMetadata is used to send data to the queue, so it contains only the ids.
// It may look weired, because it has to be compatible with the old queue data format.
// If the IsDelete flag is true, the IDs specify the conversations to delete from the index without querying the database.
// If the IsDelete flag is false, the ID specify the conversation to index, so Indexer will query the database to get the conversation data.
// It should be noted that if the id is not existing in the database, it's index will be deleted too even if IsDelete is false.
// Valid values:
//   - IsDelete = true, IDs = [1, 2, 3], and ID will be ignored
//   - IsDelete = false, ID = 1, and IDs will be ignored
type IndexerMetadata struct {
	ID int64 `json:"id"`

	IsDelete bool    `json:"is_delete"`
	IDs      []int64 `json:"ids"`
}

var (
	// conversationIndexerQueue queue of conversation ids to be updated
	conversationIndexerQueue *queue.WorkerPoolQueue[*IndexerMetadata]
	// globalIndexer is the global indexer, it cannot be nil.
	// When the real indexer is not ready, it will be a dummy indexer which will return error to explain it's not ready.
	// So it's always safe use it as *globalIndexer.Load() and call its methods.
	globalIndexer atomic.Pointer[internal.Indexer]
	dummyIndexer  *internal.Indexer
)

func init() {
	i := internal.NewDummyIndexer()
	dummyIndexer = &i
	globalIndexer.Store(dummyIndexer)
}

// InitConversationIndexer initialize conversation indexer, syncReindex is true then reindex until
// all conversation index done.
func InitConversationIndexer(syncReindex bool) {
	ctx, _, finished := process.GetManager().AddTypedContext(context.Background(), "Service: ConversationIndexer", process.SystemProcessType, false)

	indexerInitWaitChannel := make(chan time.Duration, 1)

	// Create the Queue
	conversationIndexerQueue = queue.CreateUniqueQueue(ctx, "conversation_indexer", getConversationIndexerQueueHandler(ctx))

	graceful.GetManager().RunAtTerminate(finished)

	// Create the Indexer
	go func() {
		pprof.SetGoroutineLabels(ctx)
		start := time.Now()
		log.Info("PID %d: Initializing Conversation Indexer: %s", os.Getpid(), setting.Indexer.ConversationType)
		var (
			conversationIndexer internal.Indexer
			existed             bool
			err                 error
		)
		switch setting.Indexer.ConversationType {
		case "bleve":
			defer func() {
				if err := recover(); err != nil {
					log.Error("PANIC whilst initializing conversation indexer: %v\nStacktrace: %s", err, log.Stack(2))
					log.Error("The indexer files are likely corrupted and may need to be deleted")
					log.Error("You can completely remove the %q directory to make Gitea recreate the indexes", setting.Indexer.ConversationPath)
					globalIndexer.Store(dummyIndexer)
					log.Fatal("PID: %d Unable to initialize the Bleve Conversation Indexer at path: %s Error: %v", os.Getpid(), setting.Indexer.ConversationPath, err)
				}
			}()
			conversationIndexer = bleve.NewIndexer(setting.Indexer.ConversationPath)
			existed, err = conversationIndexer.Init(ctx)
			if err != nil {
				log.Fatal("Unable to initialize Bleve Conversation Indexer at path: %s Error: %v", setting.Indexer.ConversationPath, err)
			}
		case "elasticsearch":
			conversationIndexer = elasticsearch.NewIndexer(setting.Indexer.ConversationConnStr, setting.Indexer.ConversationIndexerName)
			existed, err = conversationIndexer.Init(ctx)
			if err != nil {
				log.Fatal("Unable to conversationIndexer.Init with connection %s Error: %v", setting.Indexer.ConversationConnStr, err)
			}
		case "db":
			conversationIndexer = db.NewIndexer()
		case "meilisearch":
			conversationIndexer = meilisearch.NewIndexer(setting.Indexer.ConversationConnStr, setting.Indexer.ConversationConnAuth, setting.Indexer.ConversationIndexerName)
			existed, err = conversationIndexer.Init(ctx)
			if err != nil {
				log.Fatal("Unable to conversationIndexer.Init with connection %s Error: %v", setting.Indexer.ConversationConnStr, err)
			}
		default:
			log.Fatal("Unknown conversation indexer type: %s", setting.Indexer.ConversationType)
		}
		globalIndexer.Store(&conversationIndexer)

		graceful.GetManager().RunAtTerminate(func() {
			log.Debug("Closing conversation indexer")
			(*globalIndexer.Load()).Close()
			log.Info("PID: %d Conversation Indexer closed", os.Getpid())
		})

		// Start processing the queue
		go graceful.GetManager().RunWithCancel(conversationIndexerQueue)

		// Populate the index
		if !existed {
			if syncReindex {
				graceful.GetManager().RunWithShutdownContext(populateConversationIndexer)
			} else {
				go graceful.GetManager().RunWithShutdownContext(populateConversationIndexer)
			}
		}

		indexerInitWaitChannel <- time.Since(start)
		close(indexerInitWaitChannel)
	}()

	if syncReindex {
		select {
		case <-indexerInitWaitChannel:
		case <-graceful.GetManager().IsShutdown():
		}
	} else if setting.Indexer.StartupTimeout > 0 {
		go func() {
			pprof.SetGoroutineLabels(ctx)
			timeout := setting.Indexer.StartupTimeout
			if graceful.GetManager().IsChild() && setting.GracefulHammerTime > 0 {
				timeout += setting.GracefulHammerTime
			}
			select {
			case duration := <-indexerInitWaitChannel:
				log.Info("Conversation Indexer Initialization took %v", duration)
			case <-graceful.GetManager().IsShutdown():
				log.Warn("Shutdown occurred before conversation index initialisation was complete")
			case <-time.After(timeout):
				conversationIndexerQueue.ShutdownWait(5 * time.Second)
				log.Fatal("Conversation Indexer Initialization timed-out after: %v", timeout)
			}
		}()
	}
}

func getConversationIndexerQueueHandler(ctx context.Context) func(items ...*IndexerMetadata) []*IndexerMetadata {
	return func(items ...*IndexerMetadata) []*IndexerMetadata {
		var unhandled []*IndexerMetadata

		indexer := *globalIndexer.Load()
		for _, item := range items {
			log.Trace("IndexerMetadata Process: %d %v %t", item.ID, item.IDs, item.IsDelete)
			if item.IsDelete {
				if err := indexer.Delete(ctx, item.IDs...); err != nil {
					log.Error("Conversation indexer handler: failed to from index: %v Error: %v", item.IDs, err)
					unhandled = append(unhandled, item)
				}
				continue
			}
			data, existed, err := getConversationIndexerData(ctx, item.ID)
			if err != nil {
				log.Error("Conversation indexer handler: failed to get conversation data of %d: %v", item.ID, err)
				unhandled = append(unhandled, item)
				continue
			}
			if !existed {
				if err := indexer.Delete(ctx, item.ID); err != nil {
					log.Error("Conversation indexer handler: failed to delete conversation %d from index: %v", item.ID, err)
					unhandled = append(unhandled, item)
				}
				continue
			}
			if err := indexer.Index(ctx, data); err != nil {
				log.Error("Conversation indexer handler: failed to index conversation %d: %v", item.ID, err)
				unhandled = append(unhandled, item)
				continue
			}
		}

		return unhandled
	}
}

// populateConversationIndexer populate the conversation indexer with conversation data
func populateConversationIndexer(ctx context.Context) {
	ctx, _, finished := process.GetManager().AddTypedContext(ctx, "Service: PopulateConversationIndexer", process.SystemProcessType, true)
	defer finished()
	ctx = contextWithKeepRetry(ctx) // keep retrying since it's a background task
	if err := PopulateConversationIndexer(ctx); err != nil {
		log.Error("Conversation indexer population failed: %v", err)
	}
}

func PopulateConversationIndexer(ctx context.Context) error {
	for page := 1; ; page++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("shutdown before completion: %w", ctx.Err())
		default:
		}
		repos, _, err := repo_model.SearchRepositoryByName(ctx, &repo_model.SearchRepoOptions{
			ListOptions: db_model.ListOptions{Page: page, PageSize: repo_model.RepositoryListDefaultPageSize},
			OrderBy:     db_model.SearchOrderByID,
			Private:     true,
			Collaborate: optional.Some(false),
		})
		if err != nil {
			log.Error("SearchRepositoryByName: %v", err)
			continue
		}
		if len(repos) == 0 {
			log.Debug("Conversation Indexer population complete")
			return nil
		}

		for _, repo := range repos {
			if err := updateRepoIndexer(ctx, repo.ID); err != nil {
				return fmt.Errorf("populate conversation indexer for repo %d: %v", repo.ID, err)
			}
		}
	}
}

// UpdateRepoIndexer add/update all conversations of the repositories
func UpdateRepoIndexer(ctx context.Context, repoID int64) {
	if err := updateRepoIndexer(ctx, repoID); err != nil {
		log.Error("Unable to push repo %d to conversation indexer: %v", repoID, err)
	}
}

// UpdateConversationIndexer add/update an conversation to the conversation indexer
func UpdateConversationIndexer(ctx context.Context, conversationID int64) {
	if err := updateConversationIndexer(ctx, conversationID); err != nil {
		log.Error("Unable to push conversation %d to conversation indexer: %v", conversationID, err)
	}
}

// DeleteRepoConversationIndexer deletes repo's all conversations indexes
func DeleteRepoConversationIndexer(ctx context.Context, repoID int64) {
	if err := deleteRepoConversationIndexer(ctx, repoID); err != nil {
		log.Error("Unable to push deleted repo %d to conversation indexer: %v", repoID, err)
	}
}

// IsAvailable checks if conversation indexer is available
func IsAvailable(ctx context.Context) bool {
	return (*globalIndexer.Load()).Ping(ctx) == nil
}

// SearchOptions indicates the options for searching conversations
type SearchOptions = internal.SearchOptions

const (
	SortByCreatedDesc  = internal.SortByCreatedDesc
	SortByUpdatedDesc  = internal.SortByUpdatedDesc
	SortByCommentsDesc = internal.SortByCommentsDesc
	SortByDeadlineDesc = internal.SortByDeadlineDesc
	SortByCreatedAsc   = internal.SortByCreatedAsc
	SortByUpdatedAsc   = internal.SortByUpdatedAsc
	SortByCommentsAsc  = internal.SortByCommentsAsc
	SortByDeadlineAsc  = internal.SortByDeadlineAsc
)

// SearchConversations search conversations by options.
func SearchConversations(ctx context.Context, opts *SearchOptions) ([]int64, int64, error) {
	indexer := *globalIndexer.Load()

	if opts.Keyword == "" || opts.IsKeywordNumeric() {
		// This is a conservative shortcut.
		// If the keyword is empty or an integer, db has better (at least not worse) performance to filter conversations.
		// When the keyword is empty, it tends to listing rather than searching conversations.
		// So if the user creates an conversation and list conversations immediately, the conversation may not be listed because the indexer needs time to index the conversation.
		// Even worse, the external indexer like elastic search may not be available for a while,
		// and the user may not be able to list conversations completely until it is available again.
		indexer = db.NewIndexer()
	}

	result, err := indexer.Search(ctx, opts)
	if err != nil {
		return nil, 0, err
	}

	ret := make([]int64, 0, len(result.Hits))
	for _, hit := range result.Hits {
		ret = append(ret, hit.ID)
	}

	return ret, result.Total, nil
}

// CountConversations counts conversations by options. It is a shortcut of SearchConversations(ctx, opts) but only returns the total count.
func CountConversations(ctx context.Context, opts *SearchOptions) (int64, error) {
	opts = opts.Copy(func(options *SearchOptions) { options.Paginator = &db_model.ListOptions{PageSize: 0} })

	_, total, err := SearchConversations(ctx, opts)
	return total, err
}
