package zoekt

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/analyze"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/indexer/code/internal"
	indexer_internal "code.gitea.io/gitea/modules/indexer/internal"
	inner_zoekt "code.gitea.io/gitea/modules/indexer/internal/zoekt"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/typesniffer"
	"github.com/go-enry/go-enry/v2"
	"github.com/sourcegraph/zoekt"
	"github.com/sourcegraph/zoekt/build"
	"github.com/sourcegraph/zoekt/query"
)

type Indexer struct {
	indexer_internal.Indexer // do not composite inner_zoekt.Indexer directly to avoid exposing too much
	inner                    *inner_zoekt.Indexer
	indexDir                 string
}

func NewIndexer(indexDir string) *Indexer {
	indexer := inner_zoekt.NewIndexer(indexDir)
	return &Indexer{
		Indexer:  indexer,
		inner:    indexer,
		indexDir: indexDir,
	}
}

func newZoektIndexBuilder(indexDir string, repo *repo_model.Repository, targetSHA string) (*build.Builder, error) {
	opts := build.Options{
		IndexDir: indexDir,
		SizeMax:  int(setting.Indexer.MaxIndexerFileSize),
		RepositoryDescription: zoekt.Repository{
			ID:   uint32(repo.ID),
			Name: strconv.FormatInt(repo.ID, 10),
			Branches: []zoekt.RepositoryBranch{
				{
					Name:    "HEAD",
					Version: targetSHA,
				},
			},
		},
	}
	opts.SetDefaults()

	builder, err := build.NewBuilder(opts)
	if err != nil {
		return nil, fmt.Errorf("build.newZoektIndexBuilder: %w", err)
	}

	return builder, nil
}

func (b *Indexer) addDelete(builder *build.Builder, filename string) {
	builder.MarkFileAsChangedOrRemoved(filename)
}

func (b *Indexer) addUpdate(ctx context.Context, builder *build.Builder, batchWriter git.WriteCloserError, batchReader *bufio.Reader, sha string, update internal.FileUpdate, repo *repo_model.Repository) error {
	// Ignore vendored files in code search
	if setting.Indexer.ExcludeVendored && analyze.IsVendor(update.Filename) {
		return nil
	}

	size := update.Size
	var err error
	if !update.Sized {
		var stdout string
		stdout, _, err = git.NewCommand("cat-file", "-s").AddDynamicArguments(update.BlobSha).RunStdString(ctx, &git.RunOpts{Dir: repo.RepoPath()})
		if err != nil {
			return err
		}
		if size, err = strconv.ParseInt(strings.TrimSpace(stdout), 10, 64); err != nil {
			return fmt.Errorf("misformatted git cat-file output: %w", err)
		}
	}
	if size > setting.Indexer.MaxIndexerFileSize {
		b.addDelete(builder, update.Filename)
		return nil
	}

	if _, err := batchWriter.Write([]byte(update.BlobSha + "\n")); err != nil {
		return err
	}

	_, _, size, err = git.ReadBatchLine(batchReader)
	if err != nil {
		return err
	}

	fileContents, err := io.ReadAll(io.LimitReader(batchReader, size))
	if err != nil {
		return err
	} else if !typesniffer.DetectContentType(fileContents).IsText() {
		// FIXME: UTF-16 files will probably fail here
		return nil
	}

	if _, err = batchReader.Discard(1); err != nil {
		return err
	}

	builder.MarkFileAsChangedOrRemoved(update.Filename)

	//branches := []string{repo.DefaultBranch}
	branches := []string{"HEAD"}

	err = builder.Add(
		zoekt.Document{
			Name:     update.Filename,
			Content:  charset.ToUTF8DropErrors(fileContents, charset.ConvertOpts{}),
			Branches: branches,
		})
	if err != nil {
		return fmt.Errorf("error adding document with name %s: %w", update.Filename, err)
	}

	return nil
}

// Index will save the index data
func (b *Indexer) Index(ctx context.Context, repo *repo_model.Repository, sha string, changes *internal.RepoChanges) error {
	builder, err := newZoektIndexBuilder(b.indexDir, repo, sha)
	if err != nil {
		return fmt.Errorf("error creating builder: %w", err)
	}

	if len(changes.Updates) > 0 {
		r, err := gitrepo.OpenRepository(ctx, repo)
		if err != nil {
			return err
		}
		defer r.Close()
		batch, err := r.NewBatch(ctx)
		if err != nil {
			return err
		}
		defer batch.Close()

		for _, update := range changes.Updates {
			err := b.addUpdate(ctx, builder, batch.Writer, batch.Reader, sha, update, repo)
			if err != nil {
				return err
			}
		}
		batch.Close()
	}

	for _, filename := range changes.RemovedFilenames {
		b.addDelete(builder, filename)
	}

	return builder.Finish()
}

// Delete entries by repoId
func (b *Indexer) Delete(ctx context.Context, repoID int64) error {
	// TODO 直接在磁盘上删除索引文件

	return nil
}

func (b *Indexer) Search(ctx context.Context, opts *internal.SearchOptions) (int64, []*internal.SearchResult, []*internal.SearchResultLanguages, error) {
	var searchResults []*internal.SearchResult

	searchOpts := &zoekt.SearchOptions{
		Whole: true,
	}

	q, err := query.Parse(opts.Keyword)
	if err != nil {
		return 0, nil, nil, err
	}

	if len(opts.RepoIDs) > 0 {
		repoIds := make([]uint32, 0, len(opts.RepoIDs))
		for _, repoID := range opts.RepoIDs {
			repoIds = append(repoIds, uint32(repoID))
		}
		q = query.NewAnd(q, query.NewRepoIDs(repoIds...))
	}

	if opts.Language != "" {
		langQuery, err := query.Parse("lang:" + opts.Language)
		if err != nil {
			return 0, nil, nil, err
		}
		q = query.NewAnd(q, query.NewAnd(langQuery))
	}

	log.Info("Search query: %s", q.String())
	result, err := b.inner.Searcher.Search(ctx, q, searchOpts)
	if err != nil {
		return 0, nil, nil, err
	}
	log.Info("len of (result): %d", len(result.Files))

	for _, file := range result.Files {
		startIndex, endIndex := -1, -1
		for _, line := range file.LineMatches {
			for _, frag := range line.LineFragments {
				fragStart := (int)(frag.Offset)
				fragEnd := (int)(frag.Offset) + frag.MatchLength
				if startIndex < 0 || fragStart < startIndex {
					startIndex = fragStart
				}
				if endIndex < 0 || fragEnd > endIndex {
					endIndex = fragEnd
				}
			}
		}

		searchResults = append(searchResults, &internal.SearchResult{
			Filename:    file.FileName,
			Content:     string(file.Content),
			RepoID:      int64(file.RepositoryID),
			StartIndex:  startIndex,
			EndIndex:    endIndex,
			Language:    file.Language,
			Color:       enry.GetColor(file.Language),
			CommitID:    file.Version,
			UpdatedUnix: timeutil.TimeStamp(time.Now().Unix()),
		})
	}

	return int64(result.Stats.FileCount), searchResults, extractAggs(result), nil
}

func extractAggs(searchResult *zoekt.SearchResult) []*internal.SearchResultLanguages {
	var searchResultLanguages []*internal.SearchResultLanguages
	var languages = make(map[string]int)

	for _, file := range searchResult.Files {
		if _, ok := languages[file.Language]; ok {
			languages[file.Language]++
		} else {
			languages[file.Language] = 1
		}
	}
	for lang, count := range languages {
		searchResultLanguages = append(searchResultLanguages, &internal.SearchResultLanguages{
			Language: lang,
			Count:    count,
			Color:    enry.GetColor(lang),
		})
	}

	return searchResultLanguages
}
