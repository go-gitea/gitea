// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"
	"os"
	"runtime/pprof"
	"sync"
	"time"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// IndexerData data stored in the issue indexer
type IndexerData struct {
	ID       int64    `json:"id"`
	RepoID   int64    `json:"repo_id"`
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Comments []string `json:"comments"`
	IsDelete bool     `json:"is_delete"`
	IDs      []int64  `json:"ids"`
}

// Match represents on search result
type Match struct {
	ID    int64   `json:"id"`
	Score float64 `json:"score"`
}

// SearchResult represents search results
type SearchResult struct {
	Total int64
	Hits  []Match
}

// Indexer defines an interface to indexer issues contents
type Indexer interface {
	Init() (bool, error)
	Ping() bool
	Index(issue []*IndexerData) error
	Delete(ids ...int64) error
	Search(ctx context.Context, kw string, repoIDs []int64, limit, start int) (*SearchResult, error)
	Close()
}

type indexerHolder struct {
	indexer   Indexer
	mutex     sync.RWMutex
	cond      *sync.Cond
	cancelled bool
}

func newIndexerHolder() *indexerHolder {
	h := &indexerHolder{}
	h.cond = sync.NewCond(h.mutex.RLocker())
	return h
}

func (h *indexerHolder) cancel() {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.cancelled = true
	h.cond.Broadcast()
}

func (h *indexerHolder) set(indexer Indexer) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.indexer = indexer
	h.cond.Broadcast()
}

func (h *indexerHolder) get() Indexer {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	if h.indexer == nil && !h.cancelled {
		h.cond.Wait()
	}
	return h.indexer
}

var (
	// issueIndexerQueue queue of issue ids to be updated
	issueIndexerQueue *queue.WorkerPoolQueue[*IndexerData]
	holder            = newIndexerHolder()
)

// InitIssueIndexer initialize issue indexer, syncReindex is true then reindex until
// all issue index done.
func InitIssueIndexer(syncReindex bool) {
	ctx, _, finished := process.GetManager().AddTypedContext(context.Background(), "Service: IssueIndexer", process.SystemProcessType, false)

	indexerInitWaitChannel := make(chan time.Duration, 1)

	// Create the Queue
	switch setting.Indexer.IssueType {
	case "bleve", "elasticsearch", "meilisearch":
		handler := func(items ...*IndexerData) (unhandled []*IndexerData) {
			indexer := holder.get()
			if indexer == nil {
				log.Warn("Issue indexer handler: indexer is not ready, retry later.")
				return items
			}
			toIndex := make([]*IndexerData, 0, len(items))
			for _, indexerData := range items {
				log.Trace("IndexerData Process: %d %v %t", indexerData.ID, indexerData.IDs, indexerData.IsDelete)
				if indexerData.IsDelete {
					if err := indexer.Delete(indexerData.IDs...); err != nil {
						log.Error("Issue indexer handler: failed to from index: %v Error: %v", indexerData.IDs, err)
						if !indexer.Ping() {
							log.Error("Issue indexer handler: indexer is unavailable when deleting")
							unhandled = append(unhandled, indexerData)
						}
					}
					continue
				}
				toIndex = append(toIndex, indexerData)
			}
			if err := indexer.Index(toIndex); err != nil {
				log.Error("Error whilst indexing: %v Error: %v", toIndex, err)
				if !indexer.Ping() {
					log.Error("Issue indexer handler: indexer is unavailable when indexing")
					unhandled = append(unhandled, toIndex...)
				}
			}
			return unhandled
		}

		issueIndexerQueue = queue.CreateSimpleQueue(ctx, "issue_indexer", handler)

		if issueIndexerQueue == nil {
			log.Fatal("Unable to create issue indexer queue")
		}
	default:
		issueIndexerQueue = queue.CreateSimpleQueue[*IndexerData](ctx, "issue_indexer", nil)
	}

	graceful.GetManager().RunAtTerminate(finished)

	// Create the Indexer
	go func() {
		pprof.SetGoroutineLabels(ctx)
		start := time.Now()
		log.Info("PID %d: Initializing Issue Indexer: %s", os.Getpid(), setting.Indexer.IssueType)
		var populate bool
		switch setting.Indexer.IssueType {
		case "bleve":
			defer func() {
				if err := recover(); err != nil {
					log.Error("PANIC whilst initializing issue indexer: %v\nStacktrace: %s", err, log.Stack(2))
					log.Error("The indexer files are likely corrupted and may need to be deleted")
					log.Error("You can completely remove the %q directory to make Gitea recreate the indexes", setting.Indexer.IssuePath)
					holder.cancel()
					log.Fatal("PID: %d Unable to initialize the Bleve Issue Indexer at path: %s Error: %v", os.Getpid(), setting.Indexer.IssuePath, err)
				}
			}()
			issueIndexer := NewBleveIndexer(setting.Indexer.IssuePath)
			exist, err := issueIndexer.Init()
			if err != nil {
				holder.cancel()
				log.Fatal("Unable to initialize Bleve Issue Indexer at path: %s Error: %v", setting.Indexer.IssuePath, err)
			}
			populate = !exist
			holder.set(issueIndexer)
			graceful.GetManager().RunAtTerminate(func() {
				log.Debug("Closing issue indexer")
				issueIndexer := holder.get()
				if issueIndexer != nil {
					issueIndexer.Close()
				}
				log.Info("PID: %d Issue Indexer closed", os.Getpid())
			})
			log.Debug("Created Bleve Indexer")
		case "elasticsearch":
			issueIndexer, err := NewElasticSearchIndexer(setting.Indexer.IssueConnStr, setting.Indexer.IssueIndexerName)
			if err != nil {
				log.Fatal("Unable to initialize Elastic Search Issue Indexer at connection: %s Error: %v", setting.Indexer.IssueConnStr, err)
			}
			exist, err := issueIndexer.Init()
			if err != nil {
				log.Fatal("Unable to issueIndexer.Init with connection %s Error: %v", setting.Indexer.IssueConnStr, err)
			}
			populate = !exist
			holder.set(issueIndexer)
		case "db":
			issueIndexer := &DBIndexer{}
			holder.set(issueIndexer)
		case "meilisearch":
			issueIndexer, err := NewMeilisearchIndexer(setting.Indexer.IssueConnStr, setting.Indexer.IssueConnAuth, setting.Indexer.IssueIndexerName)
			if err != nil {
				log.Fatal("Unable to initialize Meilisearch Issue Indexer at connection: %s Error: %v", setting.Indexer.IssueConnStr, err)
			}
			exist, err := issueIndexer.Init()
			if err != nil {
				log.Fatal("Unable to issueIndexer.Init with connection %s Error: %v", setting.Indexer.IssueConnStr, err)
			}
			populate = !exist
			holder.set(issueIndexer)
		default:
			holder.cancel()
			log.Fatal("Unknown issue indexer type: %s", setting.Indexer.IssueType)
		}

		// Start processing the queue
		go graceful.GetManager().RunWithCancel(issueIndexerQueue)

		// Populate the index
		if populate {
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

// populateIssueIndexer populate the issue indexer with issue data
func populateIssueIndexer(ctx context.Context) {
	ctx, _, finished := process.GetManager().AddTypedContext(ctx, "Service: PopulateIssueIndexer", process.SystemProcessType, true)
	defer finished()
	for page := 1; ; page++ {
		select {
		case <-ctx.Done():
			log.Warn("Issue Indexer population shutdown before completion")
			return
		default:
		}
		repos, _, err := repo_model.SearchRepositoryByName(ctx, &repo_model.SearchRepoOptions{
			ListOptions: db.ListOptions{Page: page, PageSize: repo_model.RepositoryListDefaultPageSize},
			OrderBy:     db.SearchOrderByID,
			Private:     true,
			Collaborate: util.OptionalBoolFalse,
		})
		if err != nil {
			log.Error("SearchRepositoryByName: %v", err)
			continue
		}
		if len(repos) == 0 {
			log.Debug("Issue Indexer population complete")
			return
		}

		for _, repo := range repos {
			select {
			case <-ctx.Done():
				log.Info("Issue Indexer population shutdown before completion")
				return
			default:
			}
			UpdateRepoIndexer(ctx, repo)
		}
	}
}

// UpdateRepoIndexer add/update all issues of the repositories
func UpdateRepoIndexer(ctx context.Context, repo *repo_model.Repository) {
	is, err := issues_model.Issues(ctx, &issues_model.IssuesOptions{
		RepoIDs:  []int64{repo.ID},
		IsClosed: util.OptionalBoolNone,
		IsPull:   util.OptionalBoolNone,
	})
	if err != nil {
		log.Error("Issues: %v", err)
		return
	}
	if err = issues_model.IssueList(is).LoadDiscussComments(ctx); err != nil {
		log.Error("LoadDiscussComments: %v", err)
		return
	}
	for _, issue := range is {
		UpdateIssueIndexer(issue)
	}
}

// UpdateIssueIndexer add/update an issue to the issue indexer
func UpdateIssueIndexer(issue *issues_model.Issue) {
	var comments []string
	for _, comment := range issue.Comments {
		if comment.Type == issues_model.CommentTypeComment {
			comments = append(comments, comment.Content)
		}
	}
	indexerData := &IndexerData{
		ID:       issue.ID,
		RepoID:   issue.RepoID,
		Title:    issue.Title,
		Content:  issue.Content,
		Comments: comments,
	}
	log.Debug("Adding to channel: %v", indexerData)
	if err := issueIndexerQueue.Push(indexerData); err != nil {
		log.Error("Unable to push to issue indexer: %v: Error: %v", indexerData, err)
	}
}

// DeleteRepoIssueIndexer deletes repo's all issues indexes
func DeleteRepoIssueIndexer(ctx context.Context, repo *repo_model.Repository) {
	var ids []int64
	ids, err := issues_model.GetIssueIDsByRepoID(ctx, repo.ID)
	if err != nil {
		log.Error("GetIssueIDsByRepoID failed: %v", err)
		return
	}

	if len(ids) == 0 {
		return
	}
	indexerData := &IndexerData{
		IDs:      ids,
		IsDelete: true,
	}
	if err := issueIndexerQueue.Push(indexerData); err != nil {
		log.Error("Unable to push to issue indexer: %v: Error: %v", indexerData, err)
	}
}

// SearchIssuesByKeyword search issue ids by keywords and repo id
// WARNNING: You have to ensure user have permission to visit repoIDs' issues
func SearchIssuesByKeyword(ctx context.Context, repoIDs []int64, keyword string) ([]int64, error) {
	var issueIDs []int64
	indexer := holder.get()

	if indexer == nil {
		log.Error("SearchIssuesByKeyword(): unable to get indexer!")
		return nil, fmt.Errorf("unable to get issue indexer")
	}
	res, err := indexer.Search(ctx, keyword, repoIDs, 50, 0)
	if err != nil {
		return nil, err
	}
	for _, r := range res.Hits {
		issueIDs = append(issueIDs, r.ID)
	}
	return issueIDs, nil
}

// IsAvailable checks if issue indexer is available
func IsAvailable() bool {
	indexer := holder.get()
	if indexer == nil {
		log.Error("IsAvailable(): unable to get indexer!")
		return false
	}

	return indexer.Ping()
}
