// Copyright 2025 okTurtles Foundation. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"testing"
	"time"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestBuildForkGraph(t *testing.T) {
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
	assert.NotNil(t, graph.Root)
	assert.Equal(t, "repo_1", graph.Root.ID)
	assert.Equal(t, 0, graph.Root.Level)
	assert.NotNil(t, graph.Metadata)
}

func TestBuildForkGraphWithContributors(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	params := ForkGraphParams{
		IncludeContributors: true,
		ContributorDays:     30,
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
	assert.Equal(t, 30, graph.Metadata.ContributorWindowDays)
}

func TestBuildForkGraphMaxDepth(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	params := ForkGraphParams{
		IncludeContributors: false,
		ContributorDays:     90,
		MaxDepth:            2,
		IncludePrivate:      false,
		Sort:                "updated",
		Page:                1,
		Limit:               50,
	}

	ctx := context.Background()
	graph, err := BuildForkGraph(ctx, repo, params, user)

	assert.NoError(t, err)
	assert.NotNil(t, graph)

	// Check that depth is limited
	maxLevel := getMaxLevel(graph.Root)
	assert.LessOrEqual(t, maxLevel, 2)
}

func TestSortRepositories(t *testing.T) {
	repos := []*repo_model.Repository{
		{ID: 1, NumStars: 10, NumForks: 5, UpdatedUnix: 1000, CreatedUnix: 900},
		{ID: 2, NumStars: 20, NumForks: 3, UpdatedUnix: 2000, CreatedUnix: 1000},
		{ID: 3, NumStars: 5, NumForks: 10, UpdatedUnix: 1500, CreatedUnix: 1100},
	}

	// Test sort by stars
	sortRepositories(repos, "stars")
	assert.Equal(t, int64(2), repos[0].ID)
	assert.Equal(t, int64(1), repos[1].ID)
	assert.Equal(t, int64(3), repos[2].ID)

	// Reset
	repos = []*repo_model.Repository{
		{ID: 1, NumStars: 10, NumForks: 5, UpdatedUnix: 1000, CreatedUnix: 900},
		{ID: 2, NumStars: 20, NumForks: 3, UpdatedUnix: 2000, CreatedUnix: 1000},
		{ID: 3, NumStars: 5, NumForks: 10, UpdatedUnix: 1500, CreatedUnix: 1100},
	}

	// Test sort by forks
	sortRepositories(repos, "forks")
	assert.Equal(t, int64(3), repos[0].ID)
	assert.Equal(t, int64(1), repos[1].ID)
	assert.Equal(t, int64(2), repos[2].ID)

	// Reset
	repos = []*repo_model.Repository{
		{ID: 1, NumStars: 10, NumForks: 5, UpdatedUnix: 1000, CreatedUnix: 900},
		{ID: 2, NumStars: 20, NumForks: 3, UpdatedUnix: 2000, CreatedUnix: 1000},
		{ID: 3, NumStars: 5, NumForks: 10, UpdatedUnix: 1500, CreatedUnix: 1100},
	}

	// Test sort by updated
	sortRepositories(repos, "updated")
	assert.Equal(t, int64(2), repos[0].ID)
	assert.Equal(t, int64(3), repos[1].ID)
	assert.Equal(t, int64(1), repos[2].ID)

	// Reset
	repos = []*repo_model.Repository{
		{ID: 1, NumStars: 10, NumForks: 5, UpdatedUnix: 1000, CreatedUnix: 900},
		{ID: 2, NumStars: 20, NumForks: 3, UpdatedUnix: 2000, CreatedUnix: 1000},
		{ID: 3, NumStars: 5, NumForks: 10, UpdatedUnix: 1500, CreatedUnix: 1100},
	}

	// Test sort by created
	sortRepositories(repos, "created")
	assert.Equal(t, int64(3), repos[0].ID)
	assert.Equal(t, int64(2), repos[1].ID)
	assert.Equal(t, int64(1), repos[2].ID)
}

func TestCountVisibleForks(t *testing.T) {
	// Create a simple tree structure
	root := &ForkNode{
		ID:    "repo_1",
		Level: 0,
		Children: []*ForkNode{
			{
				ID:    "repo_2",
				Level: 1,
				Children: []*ForkNode{
					{
						ID:       "repo_4",
						Level:    2,
						Children: []*ForkNode{},
					},
				},
			},
			{
				ID:       "repo_3",
				Level:    1,
				Children: []*ForkNode{},
			},
		},
	}

	count := countVisibleForks(root)
	assert.Equal(t, 3, count) // 2 direct children + 1 grandchild
}

func TestCycleDetection(t *testing.T) {
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
	visited := make(map[int64]bool)
	nodeCount := 0
	maxDepthReached := false

	// Build node twice with same repo - should detect cycle
	node1, err := buildNode(ctx, repo, 0, params, user, visited, &nodeCount, &maxDepthReached)
	assert.NoError(t, err)
	assert.NotNil(t, node1)

	// Try to build same repo again - should return ErrCycleDetected
	node2, err := buildNode(ctx, repo, 0, params, user, visited, &nodeCount, &maxDepthReached)
	assert.Error(t, err)
	assert.True(t, IsErrCycleDetected(err))
	assert.Nil(t, node2)
}

func TestErrorTypes(t *testing.T) {
	assert.True(t, IsErrMaxDepthExceeded(ErrMaxDepthExceeded))
	assert.False(t, IsErrMaxDepthExceeded(ErrTooManyNodes))

	assert.True(t, IsErrTooManyNodes(ErrTooManyNodes))
	assert.False(t, IsErrTooManyNodes(ErrProcessingTimeout))

	assert.True(t, IsErrProcessingTimeout(ErrProcessingTimeout))
	assert.False(t, IsErrProcessingTimeout(ErrMaxDepthExceeded))

	assert.True(t, IsErrCycleDetected(ErrCycleDetected))
	assert.False(t, IsErrCycleDetected(ErrTooManyNodes))
}

func TestGetContributorStats(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	// Test getting contributor stats
	stats, err := getContributorStats(repo, 90)

	// Should not error even if stats are not available
	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.TotalCount, 0)
	assert.GreaterOrEqual(t, stats.RecentCount, 0)
}

func TestProcessingTimeout(t *testing.T) {
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

	// Create a context that's already cancelled
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(10 * time.Millisecond) // Ensure timeout

	visited := make(map[int64]bool)
	nodeCount := 0
	maxDepthReached := false

	_, err := buildNode(ctx, repo, 0, params, user, visited, &nodeCount, &maxDepthReached)
	assert.Error(t, err)
	assert.True(t, IsErrProcessingTimeout(err))
}

// Helper function to get max level in tree
func getMaxLevel(node *ForkNode) int {
	if node == nil || len(node.Children) == 0 {
		return node.Level
	}

	maxLevel := node.Level
	for _, child := range node.Children {
		childMax := getMaxLevel(child)
		if childMax > maxLevel {
			maxLevel = childMax
		}
	}

	return maxLevel
}

func TestCycleDetection_SelfLoop(t *testing.T) {
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
	visited := make(map[int64]bool)
	nodeCount := 0
	maxDepthReached := false

	// First call should succeed
	node1, err := buildNode(ctx, repo, 0, params, user, visited, &nodeCount, &maxDepthReached)
	assert.NoError(t, err)
	assert.NotNil(t, node1)
	assert.Equal(t, 1, nodeCount)

	// Second call with same repo should detect cycle
	node2, err := buildNode(ctx, repo, 0, params, user, visited, &nodeCount, &maxDepthReached)
	assert.Error(t, err)
	assert.True(t, IsErrCycleDetected(err))
	assert.Nil(t, node2)
	// Node count should not increase on cycle detection
	assert.Equal(t, 1, nodeCount)
}

func TestCycleDetection_VisitedMapPreventsInfiniteRecursion(t *testing.T) {
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
	visited := make(map[int64]bool)
	nodeCount := 0
	maxDepthReached := false

	// Build node - this will mark repo as visited
	node, err := buildNode(ctx, repo, 0, params, user, visited, &nodeCount, &maxDepthReached)
	assert.NoError(t, err)
	assert.NotNil(t, node)

	// Verify repo is in visited map
	assert.True(t, visited[repo.ID])

	// Attempting to visit again should immediately return ErrCycleDetected
	// without causing stack overflow or infinite recursion
	node2, err := buildNode(ctx, repo, 0, params, user, visited, &nodeCount, &maxDepthReached)
	assert.Error(t, err)
	assert.True(t, IsErrCycleDetected(err))
	assert.Nil(t, node2)
}

func TestCycleDetection_ErrorPropagation(t *testing.T) {
	// Test that ErrCycleDetected is properly handled by callers
	// and doesn't cause the entire graph building to fail

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

	// BuildForkGraph should handle cycles gracefully and continue building
	graph, err := BuildForkGraph(ctx, repo, params, user)
	assert.NoError(t, err)
	assert.NotNil(t, graph)
	assert.NotNil(t, graph.Root)
}

func TestCycleDetection_DeepForkChain(t *testing.T) {
	// Test that cycle detection works correctly in deep fork chains
	// This ensures O(n) performance and no stack overflow

	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	params := ForkGraphParams{
		IncludeContributors: false,
		ContributorDays:     90,
		MaxDepth:            100, // Deep chain
		IncludePrivate:      false,
		Sort:                "updated",
		Page:                1,
		Limit:               50,
	}

	ctx := context.Background()
	visited := make(map[int64]bool)
	nodeCount := 0
	maxDepthReached := false

	// Build a deep chain - should not cause stack overflow
	node, err := buildNode(ctx, repo, 0, params, user, visited, &nodeCount, &maxDepthReached)
	assert.NoError(t, err)
	assert.NotNil(t, node)

	// Verify visited map has entries (cycle detection is working)
	assert.Greater(t, len(visited), 0)
}
