// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issues

import (
	"sync"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// IndexerData data stored in the issue indexer
type IndexerData struct {
	ID       int64
	RepoID   int64
	Title    string
	Content  string
	Comments []string
	IsDelete bool
	IDs      []int64
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

// Indexer defines an inteface to indexer issues contents
type Indexer interface {
	Init() (bool, error)
	Index(issue []*IndexerData) error
	Delete(ids ...int64) error
	Search(kw string, repoIDs []int64, limit, start int) (*SearchResult, error)
}

type indexerHolder struct {
	indexer Indexer
	mutex   sync.RWMutex
	cond    *sync.Cond
}

func newIndexerHolder() *indexerHolder {
	h := &indexerHolder{}
	h.cond = sync.NewCond(h.mutex.RLocker())
	return h
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
	if h.indexer == nil {
		h.cond.Wait()
	}
	return h.indexer
}

var (
	issueIndexerChannel = make(chan *IndexerData, setting.Indexer.UpdateQueueLength)
	// issueIndexerQueue queue of issue ids to be updated
	issueIndexerQueue Queue
	holder            = newIndexerHolder()
)

// InitIssueIndexer initialize issue indexer, syncReindex is true then reindex until
// all issue index done.
func InitIssueIndexer(syncReindex bool) {
	waitChannel := make(chan time.Duration)
	go func() {
		start := time.Now()
		log.Info("Initializing Issue Indexer")
		var populate bool
		var dummyQueue bool
		switch setting.Indexer.IssueType {
		case "bleve":
			issueIndexer := NewBleveIndexer(setting.Indexer.IssuePath)
			exist, err := issueIndexer.Init()
			if err != nil {
				log.Fatal("Unable to initialize Bleve Issue Indexer: %v", err)
			}
			populate = !exist
			holder.set(issueIndexer)
		case "db":
			issueIndexer := &DBIndexer{}
			holder.set(issueIndexer)
			dummyQueue = true
		default:
			log.Fatal("Unknown issue indexer type: %s", setting.Indexer.IssueType)
		}

		if dummyQueue {
			issueIndexerQueue = &DummyQueue{}
		} else {
			var err error
			switch setting.Indexer.IssueQueueType {
			case setting.LevelQueueType:
				issueIndexerQueue, err = NewLevelQueue(
					holder.get(),
					setting.Indexer.IssueQueueDir,
					setting.Indexer.IssueQueueBatchNumber)
				if err != nil {
					log.Fatal(
						"Unable create level queue for issue queue dir: %s batch number: %d : %v",
						setting.Indexer.IssueQueueDir,
						setting.Indexer.IssueQueueBatchNumber,
						err)
				}
			case setting.ChannelQueueType:
				issueIndexerQueue = NewChannelQueue(holder.get(), setting.Indexer.IssueQueueBatchNumber)
			case setting.RedisQueueType:
				addrs, pass, idx, err := parseConnStr(setting.Indexer.IssueQueueConnStr)
				if err != nil {
					log.Fatal("Unable to parse connection string for RedisQueueType: %s : %v",
						setting.Indexer.IssueQueueConnStr,
						err)
				}
				issueIndexerQueue, err = NewRedisQueue(addrs, pass, idx, holder.get(), setting.Indexer.IssueQueueBatchNumber)
				if err != nil {
					log.Fatal("Unable to create RedisQueue: %s : %v",
						setting.Indexer.IssueQueueConnStr,
						err)
				}
			default:
				log.Fatal("Unsupported indexer queue type: %v",
					setting.Indexer.IssueQueueType)
			}

			go func() {
				err = issueIndexerQueue.Run()
				if err != nil {
					log.Error("issueIndexerQueue.Run: %v", err)
				}
			}()
		}

		go func() {
			for data := range issueIndexerChannel {
				_ = issueIndexerQueue.Push(data)
			}
		}()

		if populate {
			if syncReindex {
				populateIssueIndexer()
			} else {
				go populateIssueIndexer()
			}
		}
		waitChannel <- time.Since(start)
	}()
	if syncReindex {
		<-waitChannel
	} else if setting.Indexer.StartupTimeout > 0 {
		go func() {
			timeout := setting.Indexer.StartupTimeout
			if graceful.Manager.IsChild() && setting.GracefulHammerTime > 0 {
				timeout += setting.GracefulHammerTime
			}
			select {
			case duration := <-waitChannel:
				log.Info("Issue Indexer Initialization took %v", duration)
			case <-time.After(timeout):
				log.Fatal("Issue Indexer Initialization timed-out after: %v", timeout)
			}
		}()
	}
}

// populateIssueIndexer populate the issue indexer with issue data
func populateIssueIndexer() {
	for page := 1; ; page++ {
		repos, _, err := models.SearchRepositoryByName(&models.SearchRepoOptions{
			Page:        page,
			PageSize:    models.RepositoryListDefaultPageSize,
			OrderBy:     models.SearchOrderByID,
			Private:     true,
			Collaborate: util.OptionalBoolFalse,
		})
		if err != nil {
			log.Error("SearchRepositoryByName: %v", err)
			continue
		}
		if len(repos) == 0 {
			return
		}

		for _, repo := range repos {
			is, err := models.Issues(&models.IssuesOptions{
				RepoIDs:  []int64{repo.ID},
				IsClosed: util.OptionalBoolNone,
				IsPull:   util.OptionalBoolNone,
			})
			if err != nil {
				log.Error("Issues: %v", err)
				continue
			}
			if err = models.IssueList(is).LoadDiscussComments(); err != nil {
				log.Error("LoadComments: %v", err)
				continue
			}
			for _, issue := range is {
				UpdateIssueIndexer(issue)
			}
		}
	}
}

// UpdateIssueIndexer add/update an issue to the issue indexer
func UpdateIssueIndexer(issue *models.Issue) {
	var comments []string
	for _, comment := range issue.Comments {
		if comment.Type == models.CommentTypeComment {
			comments = append(comments, comment.Content)
		}
	}
	issueIndexerChannel <- &IndexerData{
		ID:       issue.ID,
		RepoID:   issue.RepoID,
		Title:    issue.Title,
		Content:  issue.Content,
		Comments: comments,
	}
}

// DeleteRepoIssueIndexer deletes repo's all issues indexes
func DeleteRepoIssueIndexer(repo *models.Repository) {
	var ids []int64
	ids, err := models.GetIssueIDsByRepoID(repo.ID)
	if err != nil {
		log.Error("getIssueIDsByRepoID failed: %v", err)
		return
	}

	if len(ids) == 0 {
		return
	}

	issueIndexerChannel <- &IndexerData{
		IDs:      ids,
		IsDelete: true,
	}
}

// SearchIssuesByKeyword search issue ids by keywords and repo id
func SearchIssuesByKeyword(repoIDs []int64, keyword string) ([]int64, error) {
	var issueIDs []int64
	res, err := holder.get().Search(keyword, repoIDs, 1000, 0)
	if err != nil {
		return nil, err
	}
	for _, r := range res.Hits {
		issueIDs = append(issueIDs, r.ID)
	}
	return issueIDs, nil
}
