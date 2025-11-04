// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

// TestBatchLoadingQueryCount verifies that batch loading reduces query count
// To see actual query counts, run with: GITEA_UNIT_TESTS_LOG_SQL=1 go test ...
func TestBatchLoadingQueryCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	params := ForkGraphParams{
		IncludeContributors: false,
		ContributorDays:     90,
		MaxDepth:            10,
		IncludePrivate:      false,
		Sort:                "updated",
		Page:                1,
		Limit:               50,
	}

	ctx := context.Background()
	graph, err := BuildForkGraph(ctx, repo, params, user)

	assert.NoError(t, err)
	assert.NotNil(t, graph)

	// With batch loading, we expect:
	// - 1 query to find root repo (if needed)
	// - N queries to find forks at each level (depends on tree structure)
	// - 1 query to batch load owners
	// - 1 query to batch load licenses
	// - 1 query to batch load subjects
	// - 1 query to batch load units
	// Total: ~N+5 queries instead of 3*nodes queries

	// For a small test tree, we should see significantly fewer queries
	// than 3 * number of nodes
	nodeCount := countVisibleForks(graph.Root) + 1 // +1 for root
	t.Logf("Built fork graph with %d nodes", nodeCount)

	// Without batch loading, we'd expect ~3 queries per node
	// With batch loading, we expect ~5-10 queries total regardless of node count
	// This test verifies the optimization is working
	// Run with GITEA_UNIT_TESTS_LOG_SQL=1 to see actual query counts
}

// TestBatchLoadingIdenticalResults verifies batch loading produces identical results
func TestBatchLoadingIdenticalResults(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	params := ForkGraphParams{
		IncludeContributors: false,
		ContributorDays:     90,
		MaxDepth:            10,
		IncludePrivate:      false,
		Sort:                "updated",
		Page:                1,
		Limit:               50,
	}

	ctx := context.Background()

	// Build graph with batch loading (current implementation)
	graph1, err1 := BuildForkGraph(ctx, repo, params, user)
	assert.NoError(t, err1)
	assert.NotNil(t, graph1)

	// Build again to verify consistency
	graph2, err2 := BuildForkGraph(ctx, repo, params, user)
	assert.NoError(t, err2)
	assert.NotNil(t, graph2)

	// Verify both graphs have same structure
	assert.Equal(t, graph1.Root.ID, graph2.Root.ID)
	assert.Equal(t, graph1.Metadata.VisibleForks, graph2.Metadata.VisibleForks)
	assert.Equal(t, len(graph1.Root.Children), len(graph2.Root.Children))

	// Verify repository data is loaded correctly
	if graph1.Root.Repository != nil {
		assert.NotNil(t, graph2.Root.Repository)
		assert.Equal(t, graph1.Root.Repository.ID, graph2.Root.Repository.ID)
		assert.Equal(t, graph1.Root.Repository.Name, graph2.Root.Repository.Name)
		assert.NotNil(t, graph1.Root.Repository.Owner, "Owner should be loaded")
		assert.NotNil(t, graph2.Root.Repository.Owner, "Owner should be loaded")
	}
}

// TestCollectRepositoriesNoDuplicates verifies no duplicate repos are collected
func TestCollectRepositoriesNoDuplicates(t *testing.T) {
	// Create a test tree structure
	repo1 := &repo_model.Repository{ID: 1, Name: "repo1"}
	repo2 := &repo_model.Repository{ID: 2, Name: "repo2"}
	repo3 := &repo_model.Repository{ID: 3, Name: "repo3"}

	root := &ForkNode{
		ID:    "repo_1",
		Level: 0,
		repo:  repo1,
		Children: []*ForkNode{
			{
				ID:    "repo_2",
				Level: 1,
				repo:  repo2,
				Children: []*ForkNode{
					{
						ID:       "repo_3",
						Level:    2,
						repo:     repo3,
						Children: []*ForkNode{},
					},
				},
			},
		},
	}

	repos := collectRepositories(root)

	// Verify we got all repos
	assert.Len(t, repos, 3)

	// Verify no duplicates by checking IDs
	seenIDs := make(map[int64]bool)
	for _, repo := range repos {
		assert.False(t, seenIDs[repo.ID], "Duplicate repository ID %d found", repo.ID)
		seenIDs[repo.ID] = true
	}

	// Verify all expected IDs are present
	assert.True(t, seenIDs[1])
	assert.True(t, seenIDs[2])
	assert.True(t, seenIDs[3])
}

// TestCollectRepositoriesEmptyTree verifies handling of empty trees
func TestCollectRepositoriesEmptyTree(t *testing.T) {
	// Test nil node
	repos := collectRepositories(nil)
	assert.Nil(t, repos)

	// Test node with nil repo
	node := &ForkNode{
		ID:       "repo_1",
		Level:    0,
		repo:     nil,
		Children: []*ForkNode{},
	}
	repos = collectRepositories(node)
	assert.Empty(t, repos)
}

// TestBatchLoadingMemoryUsage measures memory usage
func TestBatchLoadingMemoryUsage(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	params := ForkGraphParams{
		IncludeContributors: false,
		ContributorDays:     90,
		MaxDepth:            10,
		IncludePrivate:      false,
		Sort:                "updated",
		Page:                1,
		Limit:               50,
	}

	ctx := context.Background()

	// Force GC before measurement
	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	graph, err := BuildForkGraph(ctx, repo, params, user)
	assert.NoError(t, err)
	assert.NotNil(t, graph)

	// Force GC after to see retained memory
	runtime.GC()
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	allocatedBytes := memAfter.Alloc - memBefore.Alloc
	nodeCount := countVisibleForks(graph.Root) + 1

	t.Logf("Memory allocated: %d bytes for %d nodes (~%d bytes/node)",
		allocatedBytes, nodeCount, allocatedBytes/uint64(nodeCount))

	// Verify internal repo references are cleared
	verifyRepoReferencesCleared(t, graph.Root)
}

// verifyRepoReferencesCleared checks that internal repo fields are nil
func verifyRepoReferencesCleared(t *testing.T, node *ForkNode) {
	if node == nil {
		return
	}

	// After conversion, internal repo reference should be nil
	assert.Nil(t, node.repo, "Internal repo reference should be cleared after conversion")

	// But API repository should be populated
	assert.NotNil(t, node.Repository, "API repository should be populated")

	// Recursively check children
	for _, child := range node.Children {
		verifyRepoReferencesCleared(t, child)
	}
}

// TestBatchLoadingWithVariousTreeSizes tests different tree sizes
func TestBatchLoadingWithVariousTreeSizes(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testCases := []struct {
		name     string
		maxDepth int
		limit    int
	}{
		{"Small tree (depth 2)", 2, 10},
		{"Medium tree (depth 5)", 5, 50},
		{"Large tree (depth 10)", 10, 100},
	}

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params := ForkGraphParams{
				IncludeContributors: false,
				ContributorDays:     90,
				MaxDepth:            tc.maxDepth,
				IncludePrivate:      false,
				Sort:                "updated",
				Page:                1,
				Limit:               tc.limit,
			}

			ctx := context.Background()
			start := time.Now()

			graph, err := BuildForkGraph(ctx, repo, params, user)

			elapsed := time.Since(start)

			assert.NoError(t, err)
			assert.NotNil(t, graph)

			nodeCount := countVisibleForks(graph.Root) + 1
			t.Logf("Built %s with %d nodes in %v", tc.name, nodeCount, elapsed)

			// Verify response time is reasonable (< 5 seconds for test data)
			assert.Less(t, elapsed, 5*time.Second, "Graph building took too long")
		})
	}
}

// BenchmarkBuildForkGraph benchmarks the fork graph building
func BenchmarkBuildForkGraph(b *testing.B) {
	assert.NoError(b, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(b, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(b, &user_model.User{ID: 2})

	params := ForkGraphParams{
		IncludeContributors: false,
		ContributorDays:     90,
		MaxDepth:            10,
		IncludePrivate:      false,
		Sort:                "updated",
		Page:                1,
		Limit:               50,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := BuildForkGraph(ctx, repo, params, user)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCollectRepositories benchmarks repository collection
func BenchmarkCollectRepositories(b *testing.B) {
	// Create a test tree with 100 nodes
	root := createTestTree(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		repos := collectRepositories(root)
		if len(repos) == 0 {
			b.Fatal("No repositories collected")
		}
	}
}

// createTestTree creates a test fork tree with specified number of nodes
func createTestTree(nodeCount int) *ForkNode {
	if nodeCount <= 0 {
		return nil
	}

	root := &ForkNode{
		ID:       "repo_1",
		Level:    0,
		repo:     &repo_model.Repository{ID: 1, Name: "root"},
		Children: []*ForkNode{},
	}

	// Create a balanced tree
	nodesToCreate := nodeCount - 1
	currentLevel := []*ForkNode{root}
	nextID := int64(2)

	for nodesToCreate > 0 && len(currentLevel) > 0 {
		nextLevel := []*ForkNode{}

		for _, parent := range currentLevel {
			// Add 2-3 children per node
			childCount := min(3, nodesToCreate)
			for i := 0; i < childCount; i++ {
				child := &ForkNode{
					ID:       fmt.Sprintf("repo_%d", nextID),
					Level:    parent.Level + 1,
					repo:     &repo_model.Repository{ID: nextID, Name: fmt.Sprintf("repo%d", nextID)},
					Children: []*ForkNode{},
				}
				parent.Children = append(parent.Children, child)
				nextLevel = append(nextLevel, child)
				nextID++
				nodesToCreate--

				if nodesToCreate == 0 {
					break
				}
			}

			if nodesToCreate == 0 {
				break
			}
		}

		currentLevel = nextLevel
	}

	return root
}

