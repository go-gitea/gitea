// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

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
	"code.gitea.io/gitea/modules/indexer"
	"code.gitea.io/gitea/modules/indexer/issues/bleve"
	"code.gitea.io/gitea/modules/indexer/issues/db"
	"code.gitea.io/gitea/modules/indexer/issues/elasticsearch"
	"code.gitea.io/gitea/modules/indexer/issues/internal"
	"code.gitea.io/gitea/modules/indexer/issues/meilisearch"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
)

// IndexerMetadata is used to send data to the queue, so it contains only the ids.
// It may look weired, because it has to be compatible with the old queue data format.
// If the IsDelete flag is true, the IDs specify the issues to delete from the index without querying the database.
// If the IsDelete flag is false, the ID specify the issue to index, so Indexer will query the database to get the issue data.
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
	// issueIndexerQueue queue of issue ids to be updated
	issueIndexerQueue *queue.WorkerPoolQueue[*IndexerMetadata]
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

// InitIssueIndexer initialize issue indexer, syncReindex is true then reindex until
// all issue index done.
func InitIssueIndexer(syncReindex bool) {
	ctx, _, finished := process.GetManager().AddTypedContext(context.Background(), "Service: IssueIndexer", process.SystemProcessType, false)

	indexerInitWaitChannel := make(chan time.Duration, 1)

	// Create the Queue
	issueIndexerQueue = queue.CreateUniqueQueue(ctx, "issue_indexer", getIssueIndexerQueueHandler(ctx))

	graceful.GetManager().RunAtTerminate(finished)

	// Create the Indexer
	go func() {
		pprof.SetGoroutineLabels(ctx)
		start := time.Now()
		log.Info("PID %d: Initializing Issue Indexer: %s", os.Getpid(), setting.Indexer.IssueType)
		var (
			issueIndexer internal.Indexer
			existed      bool
			err          error
		)
		switch setting.Indexer.IssueType {
		case "bleve":
			defer func() {
				if err := recover(); err != nil {
					log.Error("PANIC whilst initializing issue indexer: %v\nStacktrace: %s", err, log.Stack(2))
					log.Error("The indexer files are likely corrupted and may need to be deleted")
					log.Error("You can completely remove the %q directory to make Gitea recreate the indexes", setting.Indexer.IssuePath)
					globalIndexer.Store(dummyIndexer)
					log.Fatal("PID: %d Unable to initialize the Bleve Issue Indexer at path: %s Error: %v", os.Getpid(), setting.Indexer.IssuePath, err)
				}
			}()
			issueIndexer = bleve.NewIndexer(setting.Indexer.IssuePath)
			existed, err = issueIndexer.Init(ctx)
			if err != nil {
				log.Fatal("Unable to initialize Bleve Issue Indexer at path: %s Error: %v", setting.Indexer.IssuePath, err)
			}
		case "elasticsearch":
			issueIndexer = elasticsearch.NewIndexer(setting.Indexer.IssueConnStr, setting.Indexer.IssueIndexerName)
			existed, err = issueIndexer.Init(ctx)
			if err != nil {
				log.Fatal("Unable to issueIndexer.Init with connection %s Error: %v", setting.Indexer.IssueConnStr, err)
			}
		case "db":
			issueIndexer = db.GetIndexer()
		case "meilisearch":
			issueIndexer = meilisearch.NewIndexer(setting.Indexer.IssueConnStr, setting.Indexer.IssueConnAuth, setting.Indexer.IssueIndexerName)
			existed, err = issueIndexer.Init(ctx)
			if err != nil {
				log.Fatal("Unable to issueIndexer.Init with connection %s Error: %v", setting.Indexer.IssueConnStr, err)
			}
		default:
			log.Fatal("Unknown issue indexer type: %s", setting.Indexer.IssueType)
		}
		globalIndexer.Store(&issueIndexer)

		graceful.GetManager().RunAtTerminate(func() {
			log.Debug("Closing issue indexer")
			(*globalIndexer.Load()).Close()
			log.Info("PID: %d Issue Indexer closed", os.Getpid())
		})

		// Start processing the queue
		go graceful.GetManager().RunWithCancel(issueIndexerQueue)

		// Populate the index
		if !existed {
			if syncReindex {
				graceful.GetManager().RunWithShutdownContext(populateIssueIndexer)
			} else {
				go graceful.GetManager().RunWithShutdownContext(populateIssueIndexer)
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
				log.Info("Issue Indexer Initialization took %v", duration)
			case <-graceful.GetManager().IsShutdown():
				log.Warn("Shutdown occurred before issue index initialisation was complete")
			case <-time.After(timeout):
				issueIndexerQueue.ShutdownWait(5 * time.Second)
				log.Fatal("Issue Indexer Initialization timed-out after: %v", timeout)
			}
		}()
	}
}

func getIssueIndexerQueueHandler(ctx context.Context) func(items ...*IndexerMetadata) []*IndexerMetadata {
	return func(items ...*IndexerMetadata) []*IndexerMetadata {
		var unhandled []*IndexerMetadata

		indexer := *globalIndexer.Load()
		for _, item := range items {
			log.Trace("IndexerMetadata Process: %d %v %t", item.ID, item.IDs, item.IsDelete)
			if item.IsDelete {
				if err := indexer.Delete(ctx, item.IDs...); err != nil {
					log.Error("Issue indexer handler: failed to from index: %v Error: %v", item.IDs, err)
					unhandled = append(unhandled, item)
				}
				continue
			}
			data, existed, err := getIssueIndexerData(ctx, item.ID)
			if err != nil {
				log.Error("Issue indexer handler: failed to get issue data of %d: %v", item.ID, err)
				unhandled = append(unhandled, item)
				continue
			}
			if !existed {
				if err := indexer.Delete(ctx, item.ID); err != nil {
					log.Error("Issue indexer handler: failed to delete issue %d from index: %v", item.ID, err)
					unhandled = append(unhandled, item)
				}
				continue
			}
			if err := indexer.Index(ctx, data); err != nil {
				log.Error("Issue indexer handler: failed to index issue %d: %v", item.ID, err)
				unhandled = append(unhandled, item)
				continue
			}
		}

		return unhandled
	}
}

// populateIssueIndexer populate the issue indexer with issue data
func populateIssueIndexer(ctx context.Context) {
	ctx, _, finished := process.GetManager().AddTypedContext(ctx, "Service: PopulateIssueIndexer", process.SystemProcessType, true)
	defer finished()
	ctx = contextWithKeepRetry(ctx) // keep retrying since it's a background task
	if err := PopulateIssueIndexer(ctx); err != nil {
		log.Error("Issue indexer population failed: %v", err)
	}
}

func PopulateIssueIndexer(ctx context.Context) error {
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
			log.Debug("Issue Indexer population complete")
			return nil
		}

		for _, repo := range repos {
			if err := updateRepoIndexer(ctx, repo.ID); err != nil {
				return fmt.Errorf("populate issue indexer for repo %d: %v", repo.ID, err)
			}
		}
	}
}

// UpdateRepoIndexer add/update all issues of the repositories
func UpdateRepoIndexer(ctx context.Context, repoID int64) {
	if err := updateRepoIndexer(ctx, repoID); err != nil {
		log.Error("Unable to push repo %d to issue indexer: %v", repoID, err)
	}
}

// UpdateIssueIndexer add/update an issue to the issue indexer
func UpdateIssueIndexer(ctx context.Context, issueID int64) {
	if err := updateIssueIndexer(ctx, issueID); err != nil {
		log.Error("Unable to push issue %d to issue indexer: %v", issueID, err)
	}
}

// DeleteRepoIssueIndexer deletes repo's all issues indexes
func DeleteRepoIssueIndexer(ctx context.Context, repoID int64) {
	if err := deleteRepoIssueIndexer(ctx, repoID); err != nil {
		log.Error("Unable to push deleted repo %d to issue indexer: %v", repoID, err)
	}
}

// IsAvailable checks if issue indexer is available
func IsAvailable(ctx context.Context) bool {
	return (*globalIndexer.Load()).Ping(ctx) == nil
}

// SearchOptions indicates the options for searching issues
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

// SearchIssues search issues by options.
func SearchIssues(ctx context.Context, opts *SearchOptions) ([]int64, int64, error) {
	ix := *globalIndexer.Load()

	if opts.Keyword == "" || opts.IsKeywordNumeric() {
		// This is a conservative shortcut.
		// If the keyword is empty or an integer, db has better (at least not worse) performance to filter issues.
		// When the keyword is empty, it tends to listing rather than searching issues.
		// So if the user creates an issue and list issues immediately, the issue may not be listed because the indexer needs time to index the issue.
		// Even worse, the external indexer like elastic search may not be available for a while,
		// and the user may not be able to list issues completely until it is available again.
		ix = db.GetIndexer()
	}

	result, err := ix.Search(ctx, opts)
	if err != nil {
		return nil, 0, err
	}
	return SearchResultToIDSlice(result), result.Total, nil
}

func SearchResultToIDSlice(result *internal.SearchResult) []int64 {
	ret := make([]int64, 0, len(result.Hits))
	for _, hit := range result.Hits {
		ret = append(ret, hit.ID)
	}
	return ret
}

// CountIssues counts issues by options. It is a shortcut of SearchIssues(ctx, opts) but only returns the total count.
func CountIssues(ctx context.Context, opts *SearchOptions) (int64, error) {
	opts = opts.Copy(func(options *SearchOptions) { options.Paginator = &db_model.ListOptions{PageSize: 0} })

	_, total, err := SearchIssues(ctx, opts)
	return total, err
}

func SupportedSearchModes() []indexer.SearchMode {
	gi := globalIndexer.Load()
	if gi == nil {
		return nil
	}
	return (*gi).SupportedSearchModes()
}
