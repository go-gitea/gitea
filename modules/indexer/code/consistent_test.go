// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package code_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/charset"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/indexer"
	"gitea.dev/modules/indexer/code"
	"gitea.dev/modules/indexer/code/bleve"
	"gitea.dev/modules/indexer/code/elasticsearch"
	"gitea.dev/modules/indexer/code/gitgrep"
	"gitea.dev/modules/indexer/code/internal"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsistentCodeSearch(t *testing.T) {
	unittest.PrepareTestEnv(t)
	defer test.MockVariableValue(&setting.Indexer.IncludePatterns, setting.IndexerGlobFromString("**.txt,**.js"))()
	defer test.MockVariableValue(&setting.Indexer.ExcludePatterns, setting.IndexerGlobFromString("**excluded**"))()
	defer test.MockVariableValue(&setting.Indexer.ExcludeVendored, true)()
	defer test.MockVariableValue(&setting.Indexer.MaxIndexerFileSize, 8*1024)()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 62})
	files := []git.FastImportFile{
		{Path: "lines/nonempty.txt", Content: "hello world\n"},
		{Path: "lines/binary.txt", Content: "\x00abc"},
		{Path: "lines/empty.txt", Content: ""},
		{Path: "lines/blank.txt", Content: "first\n\nlast\n"},
		{Path: "cafe\u0301.txt", Content: "unrelated"},
		{Path: "split.txt", Content: "Alpha\nintervening\nBeta\n"},
		{Path: "Alpha/mixed.txt", Content: "Beta"},
		{Path: "Alpha/Beta.txt", Content: "file start\nwithout terms"},
		{Path: "Alpha/Beta.bin.txt", Content: "\x00Alpha\nBeta"},
		{Path: "other.bin.txt", Content: "\x00Alpha\nBeta"},
		{Path: "case.txt", Content: "aLPHa\nbETA\nÄPFEL\nİSTANBUL\n"},
		{Path: "vendor/dependency.txt", Content: "Alpha\nBeta"},
		{Path: "oversized.txt", Content: "Alpha\nBeta\n" + strings.Repeat("x", 8*1024)},
		{Path: "nested/excluded.txt", Content: "Alpha\nBeta"},
		{Path: "ignored.md", Content: "Alpha\nBeta"},
		{Path: "partial.txt", Content: "Alpha"},
		{Path: "executable.txt", Mode: git.EntryModeExec, Content: "executable token"},
		{Path: "symlink.txt", Mode: git.EntryModeSymlink, Content: "executable token"},
		{Path: "literal.txt", Content: "Foo(x) fooXx\nFoo bar\n--verbose\nFOOBAR\nfoo BAR\nFoo"},
		{Path: "unicode.txt", Content: "Поставщик\n日本語のテキスト\nहिन्दी\nภาษาไทย\ngap\n<em>literal</em>\nabc bcd cde def\n"},
		{Path: "example-file.js", Content: "console.log(\"Hello, World!\")"},
		{Path: "cucumber.txt", Content: "Salad is good for your health"},
		{Path: "avocado.txt", Content: "# repo1\n\npineaple pie of cucumber juice"},
		{Path: "ham.txt", Content: "This is also not cheese"},
		{Path: "potato/ham.txt", Content: "This is not cheese"},
		{Path: "z[dir]/one.txt", Content: "before\nCapToken\nafter\n\n\n\nbefore\nSecondToken\nÄPFEL\nafter\n"},
		{Path: "z[dir]/two.txt", Content: "CapToken\nSecondToken\n"},
		{Path: "z[dir]/excluded.txt", Content: "CapToken\nSecondToken\n"},
		{Path: "z[dir]extra/other.txt", Content: "CapToken\nSecondToken\n"},
		{Path: "occurrences/cap.txt", Content: strings.Repeat("q", 1001) + "İ"},
	}
	for i := range gitgrep.MaxResultLimit + 1 {
		files = append(files, git.FastImportFile{Path: fmt.Sprintf("c%03d.cap.txt", i), Content: "CapToken\nSecondToken\n"})
	}
	for i, content := range []string{"abc bcd cde def", "abcdef", "abc bcd cde def", "abcdef", "abc bcd cde def"} {
		files = append(files, git.FastImportFile{Path: fmt.Sprintf("grams/%d.txt", i), Content: content})
	}
	require.NoError(t, git.ForceFastImport(t.Context(), repo, []git.FastImportCommit{{Ref: "refs/heads/consistent", Files: files}}))
	gitRepo, err := git.OpenRepository(t.Context(), repo)
	require.NoError(t, err)
	defer gitRepo.Close()
	commitID, err := gitRepo.GetRefCommitID(t.Context(), "consistent")
	require.NoError(t, err)
	require.NoError(t, gitcmd.NewCommand("fast-import").WithRepo(gitRepo).WithStdinBytes([]byte("tag annotated\nfrom "+commitID+"\ntagger Gitea <gitea@example.com> 1500000000 +0000\ndata 3\ntag\ndone\n")).RunWithStderr(t.Context()))
	tagID, err := gitRepo.GetTagID(t.Context(), "annotated")
	require.NoError(t, err)
	require.NotEqual(t, commitID, tagID)
	contents := make(map[string]string, len(files))
	for _, file := range files {
		if !strings.ContainsRune(file.Content, 0) {
			contents[file.Path] = string(charset.ToUTF8DropErrors([]byte(file.Content)))
		}
	}
	updates, err := code.ListFiles(t.Context(), gitRepo, commitID)
	require.NoError(t, err)

	for _, engine := range []string{"bleve", "elasticsearch", "gitgrep"} {
		t.Run(engine, func(t *testing.T) {
			var idx internal.Indexer
			switch engine {
			case "bleve":
				idx = bleve.NewIndexer(t.TempDir())
			case "elasticsearch":
				url := test.ExternalServiceHTTP(t, "TEST_ELASTICSEARCH_URL", "http://elasticsearch:9200")
				idx = elasticsearch.NewIndexer(url, fmt.Sprintf("consistent_%d", time.Now().UnixNano()))
			}
			if idx != nil {
				_, err := idx.Init(t.Context())
				require.NoError(t, err)
				defer idx.Close()
				defer func() { require.NoError(t, idx.Delete(t.Context(), repo.ID)) }()
				require.NoError(t, idx.Index(t.Context(), repo, commitID, &internal.RepoChanges{Updates: updates}))
				if es, ok := idx.(*elasticsearch.Indexer); ok {
					require.NoError(t, es.Refresh(t.Context()))
				}
			}
			occurrenceRanges := make([]internal.MatchRange, 1000, 1001)
			for i := range occurrenceRanges {
				occurrenceRanges[i] = internal.MatchRange{Start: i, End: i + 1}
			}
			occurrenceRanges = append(occurrenceRanges, internal.MatchRange{Start: 1001, End: 1003})
			for _, query := range []struct {
				name                            string
				terms                           []string
				path                            string
				language                        string
				expected                        []string
				ranges                          []internal.MatchRange
				fragments                       []*code.ResultFragment
				regexp                          bool
				page, pageSize                  int
				total                           int64
				incomplete, pathOnly, wrongRepo bool
			}{
				{name: "normalized filename", terms: []string{"café"}, expected: []string{"cafe\u0301.txt"}, pathOnly: true},
				{name: "normalized regexp filename", terms: []string{`^café\.txt$`}, regexp: true, expected: []string{"cafe\u0301.txt"}, pathOnly: true},
				{name: "real blank lines only", terms: []string{"^$"}, path: "lines", regexp: true, expected: []string{"lines/blank.txt"}, pathOnly: true},
				{name: "occurrence cap and unicode byte offsets", terms: []string{"q", "i"}, path: "occurrences", expected: []string{"occurrences/cap.txt"}, ranges: occurrenceRanges},
				{name: "occurrence cap missing term", terms: []string{"q", "missing"}, path: "occurrences"},
				{name: "terms across lines or path and content", terms: []string{"alpha", "beta"}, expected: []string{"split.txt", "Alpha/mixed.txt", "Alpha/Beta.txt", "Alpha/Beta.bin.txt", "case.txt"}},
				{name: "path substring", terms: []string{"lph", "eta.txt"}, expected: []string{"Alpha/Beta.txt"}, pathOnly: true},
				{name: "binary path only", terms: []string{"Beta.bin"}, expected: []string{"Alpha/Beta.bin.txt"}, pathOnly: true},
				{name: "binary content excluded", terms: []string{"other.bin", "alpha"}},
				{name: "case insensitive unicode", terms: []string{"äpfel"}, expected: []string{"case.txt", "z[dir]/one.txt"}},
				{name: "unicode lowercasing", terms: []string{"istanbul"}, expected: []string{"case.txt"}, ranges: []internal.MatchRange{{Start: 18, End: 27}}},
				{name: "path filter", terms: []string{"alpha", "beta"}, path: "Alpha", expected: []string{"Alpha/mixed.txt", "Alpha/Beta.txt", "Alpha/Beta.bin.txt"}},
				{name: "regular and executable blobs", terms: []string{"executable", "token"}, expected: []string{"executable.txt"}},
				{name: "all terms required", terms: []string{"alpha", "missing"}},
				{name: "no terms"},
				{name: "wrong repository", terms: []string{"alpha"}, wrongRepo: true},
				{name: "literal punctuation", terms: []string{"foo(x)"}, expected: []string{"literal.txt"}, ranges: []internal.MatchRange{{Start: 0, End: 6}}},
				{name: "phrase and dash", terms: []string{"FOO bar", "--verbose"}, expected: []string{"literal.txt"}, ranges: []internal.MatchRange{{Start: 13, End: 20}, {Start: 21, End: 30}, {Start: 38, End: 45}}},
				{name: "overlapping terms", terms: []string{"foobar", "foo"}, expected: []string{"literal.txt"}, ranges: []internal.MatchRange{{Start: 0, End: 3}, {Start: 7, End: 10}, {Start: 13, End: 16}, {Start: 31, End: 37}, {Start: 38, End: 41}, {Start: 46, End: 49}}},
				{name: "line anchors", terms: []string{"^f.o$"}, regexp: true, expected: []string{"literal.txt"}, ranges: []internal.MatchRange{{Start: 46, End: 49}}},
				{name: "regexp case override", terms: []string{"(?-i)Foo", "bar"}, regexp: true, expected: []string{"literal.txt"}, ranges: []internal.MatchRange{{Start: 0, End: 3}, {Start: 13, End: 16}, {Start: 17, End: 20}, {Start: 34, End: 37}, {Start: 42, End: 45}, {Start: 46, End: 49}}},
				{name: "regexp whole path", terms: []string{`^Alpha/.*\.txt$`}, regexp: true, expected: []string{"Alpha/mixed.txt", "Alpha/Beta.txt", "Alpha/Beta.bin.txt"}, pathOnly: true},
				{name: "regexp per line", terms: []string{`(?s)Alpha.*intervening`}, regexp: true},
				{name: "empty matches", terms: []string{"^"}, path: "grams", regexp: true, expected: []string{"grams/0.txt", "grams/1.txt", "grams/2.txt", "grams/3.txt", "grams/4.txt"}, pathOnly: true},
				{name: "content and filename", terms: []string{"cucumber"}, expected: []string{"cucumber.txt", "avocado.txt"}},
				{name: "filename case", terms: []string{"AVOCADO.TXT"}, expected: []string{"avocado.txt"}, pathOnly: true},
				{name: "filename anywhere", terms: []string{"ham"}, expected: []string{"ham.txt", "potato/ham.txt"}, pathOnly: true},
				{name: "nested filename", terms: []string{"potato/ham"}, expected: []string{"potato/ham.txt"}, pathOnly: true},
				{name: "phrase", terms: []string{"This is not cheese"}, expected: []string{"potato/ham.txt"}, ranges: []internal.MatchRange{{Start: 0, End: 18}}},
				{name: "language filter", terms: []string{"log"}, language: "JavaScript", expected: []string{"example-file.js"}},
				{name: "language mismatch", terms: []string{"log"}, language: "Go"},
				{name: "code punctuation", terms: []string{"console.log"}, expected: []string{"example-file.js"}},
				{name: "cyrillic substring", terms: []string{"ставщ"}, expected: []string{"unicode.txt"}, ranges: []internal.MatchRange{{Start: 4, End: 14}}},
				{name: "unicode fragments", terms: []string{"日本", "literal"}, expected: []string{"unicode.txt"}, fragments: []*code.ResultFragment{{StartLine: 1, Content: "Поставщик\n日本語のテキスト\nहिन्दी\n", Matches: []code.MatchRange{{Start: 19, End: 25}}}, {StartLine: 5, Content: "gap\n<em>literal</em>\nabc bcd cde def\n", Matches: []code.MatchRange{{Start: 8, End: 15}}}}},
				{name: "phrase requires contiguous content", terms: []string{"日本 ภาษา"}},
				{name: "directory", terms: []string{"cheese"}, path: "potato", expected: []string{"potato/ham.txt"}},
				{name: "directory boundary", terms: []string{"cheese"}, path: "pot"},
				{name: "directory case", terms: []string{"cheese"}, path: "Potato"},
				{name: "literal path filter before cap", terms: []string{"CapToken", "SecondToken"}, path: "z[dir]", expected: []string{"z[dir]/one.txt", "z[dir]/two.txt"}},
				{name: "reject candidates before cap", terms: []string{"CapToken", "äpfel"}, expected: []string{"z[dir]/one.txt"}},
				{name: "pagination", terms: []string{"CapToken", "SecondToken"}, path: "z[dir]", page: 2, pageSize: 1, total: 2, expected: []string{"z[dir]/two.txt"}},
				{name: "last page", terms: []string{"CapToken"}, path: "z[dir]", page: 3, pageSize: 1, total: 2},
				{name: "path only cap", terms: []string{".txt"}, incomplete: true, total: gitgrep.MaxResultLimit, pathOnly: true},
				{name: "path only exact cap", terms: []string{`^c0[0-4][0-9]\.cap\.txt$`}, regexp: true, total: gitgrep.MaxResultLimit, pathOnly: true},
				{name: "cap", terms: []string{"CapToken", "SecondToken"}, incomplete: true, total: gitgrep.MaxResultLimit},
			} {
				t.Run(query.name, func(t *testing.T) {
					if query.language != "" && idx == nil {
						t.Skip("engine capability")
					}
					if idx != nil && (query.regexp || query.total == gitgrep.MaxResultLimit || query.name == "pagination") || idx == nil && query.wrongRepo {
						t.Skip("engine capability")
					}
					terms := literalTerms(query.terms)
					if query.regexp {
						terms[0].Kind = indexer.SearchTermRegexp
					}
					var results []*code.Result
					var total int64
					if idx == nil {
						var incomplete bool
						results, total, incomplete, err = gitgrep.PerformSearch(t.Context(), gitRepo, &gitgrep.SearchOptions{RepoID: repo.ID, Ref: git.RefNameFromTag("annotated"), Terms: terms, Path: query.path, Page: query.page, PageSize: query.pageSize})
						require.NoError(t, err)
						assert.Equal(t, query.incomplete, incomplete)
					} else {
						var languages []*code.SearchResultLanguages
						total, results, languages, err = idx.Search(t.Context(), &internal.SearchOptions{RepoIDs: []int64{util.Iif(query.wrongRepo, 2, repo.ID)}, Terms: query.terms, Path: query.path, Language: query.language, Paginator: &db.ListOptions{Page: max(query.page, 1), PageSize: util.IfZero(query.pageSize, 50)}})
						require.NoError(t, err)
						if query.language != "" || strings.HasPrefix(query.name, "code ") {
							require.Len(t, languages, util.Iif(engine == "bleve" && query.language == "Go", 0, 1))
							if len(languages) > 0 {
								assert.Equal(t, "JavaScript", languages[0].Language)
								assert.Equal(t, 1, languages[0].Count)
							}
						}
					}
					var filenames []string
					for _, result := range results {
						filenames = append(filenames, result.Filename)
						assert.Equal(t, repo.ID, result.RepoID)
						assert.Equal(t, commitID, result.CommitID)
						assert.Equal(t, contents[result.Filename], result.Content)
						if query.pathOnly {
							assert.Empty(t, result.ContentMatches)
						}
						if query.ranges != nil {
							assert.Equal(t, query.ranges, result.ContentMatches)
						}
						if query.name == "content and filename" && result.Filename == "avocado.txt" {
							assert.Equal(t, []internal.MatchRange{{Start: 25, End: 33}}, result.ContentMatches)
						}
						if query.fragments != nil {
							assert.Equal(t, query.fragments, code.BuildResultFragments(result.Content, result.ContentMatches))
						}
					}
					if query.total == gitgrep.MaxResultLimit {
						assert.Len(t, filenames, gitgrep.MaxResultLimit)
					} else {
						assert.ElementsMatch(t, query.expected, filenames)
					}
					assert.Equal(t, util.IfZero(query.total, int64(len(query.expected))), total)
				})
			}
			t.Run("pages of verified candidates", func(t *testing.T) {
				var filenames []string
				for page := 1; page <= 3; page++ {
					var results []*internal.SearchResult
					var total int64
					if idx != nil {
						var err error
						total, results, _, err = idx.Search(t.Context(), &internal.SearchOptions{Terms: []string{"abcdef"}, Path: "grams", Paginator: &db.ListOptions{Page: page, PageSize: 1}})
						require.NoError(t, err)
					} else {
						hits, count, incomplete, err := gitgrep.PerformSearch(t.Context(), gitRepo, &gitgrep.SearchOptions{Ref: "consistent", Terms: literalTerms([]string{"abcdef"}), Path: "grams", Page: page, PageSize: 1})
						require.NoError(t, err)
						assert.False(t, incomplete)
						total = count
						results = hits
					}
					require.Len(t, results, util.Iif(page < 3, 1, 0))
					if page == 3 {
						assert.EqualValues(t, 2, total)
					}
					for _, result := range results {
						filenames = append(filenames, result.Filename)
					}
				}
				assert.ElementsMatch(t, []string{"grams/1.txt", "grams/3.txt"}, filenames)
			})
			if idx == nil {
				for _, ref := range []git.RefName{"missing", "consistent"} {
					terms := []indexer.SearchTerm{{Kind: indexer.SearchTermRegexp, Text: "["}}
					if ref == "missing" {
						terms = literalTerms([]string{"alpha"})
					}
					_, _, _, err := gitgrep.PerformSearch(t.Context(), gitRepo, &gitgrep.SearchOptions{Ref: ref, Terms: terms})
					require.Error(t, err)
					if ref == "consistent" {
						var queryError *indexer.SearchQueryError
						assert.ErrorAs(t, err, &queryError)
					}
				}
			}
		})
	}
	t.Run("empty repository", func(t *testing.T) {
		repoDir := t.TempDir()
		require.NoError(t, git.InitRepositoryLocal(t.Context(), repoDir, true, "sha1", "main"))
		emptyRepo, err := git.OpenRepositoryLocal(t.Context(), repoDir)
		require.NoError(t, err)
		defer emptyRepo.Close()
		results, total, incomplete, err := gitgrep.PerformSearch(t.Context(), emptyRepo, &gitgrep.SearchOptions{Ref: git.RefNameHead, Terms: literalTerms([]string{"foo"})})
		require.NoError(t, err)
		assert.Empty(t, results)
		assert.Zero(t, total)
		assert.False(t, incomplete)
	})
}

func literalTerms(terms []string) []indexer.SearchTerm {
	result := make([]indexer.SearchTerm, len(terms))
	for i, term := range terms {
		result[i] = indexer.SearchTerm{Text: term}
	}
	return result
}
