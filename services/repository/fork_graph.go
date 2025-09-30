// Copyright 2025 okTurtles Foundation. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/convert"
)

// Error definitions
var (
	ErrMaxDepthExceeded  = errors.New("maximum depth exceeded")
	ErrTooManyNodes      = errors.New("too many nodes in graph")
	ErrProcessingTimeout = errors.New("processing timeout")
	errAwaitGeneration   = errors.New("generation took longer than expected")
)

// IsErrMaxDepthExceeded checks if an error is ErrMaxDepthExceeded
func IsErrMaxDepthExceeded(err error) bool {
	return errors.Is(err, ErrMaxDepthExceeded)
}

// IsErrTooManyNodes checks if an error is ErrTooManyNodes
func IsErrTooManyNodes(err error) bool {
	return errors.Is(err, ErrTooManyNodes)
}

// IsErrProcessingTimeout checks if an error is ErrProcessingTimeout
func IsErrProcessingTimeout(err error) bool {
	return errors.Is(err, ErrProcessingTimeout)
}

// ForkGraphParams represents parameters for building fork graph
type ForkGraphParams struct {
	IncludeContributors bool
	ContributorDays     int
	MaxDepth            int
	IncludePrivate      bool
	Sort                string
	Page                int
	Limit               int
}

// ForkGraphResponse represents the complete fork graph response
type ForkGraphResponse struct {
	Root       *ForkNode       `json:"root"`
	Metadata   GraphMetadata   `json:"metadata"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

// ForkNode represents a node in the fork tree
type ForkNode struct {
	ID           string            `json:"id"`
	Repository   *api.Repository   `json:"repository"`
	Contributors *ContributorStats `json:"contributors,omitempty"`
	Level        int               `json:"level"`
	Children     []*ForkNode       `json:"children"`
}

// ContributorStats represents contributor statistics
type ContributorStats struct {
	TotalCount  int `json:"total_count"`
	RecentCount int `json:"recent_count"`
}

// GraphMetadata represents metadata about the fork graph
type GraphMetadata struct {
	TotalForks            int       `json:"total_forks"`
	VisibleForks          int       `json:"visible_forks"`
	MaxDepthReached       bool      `json:"max_depth_reached"`
	CacheStatus           string    `json:"cache_status"`
	GeneratedAt           time.Time `json:"generated_at"`
	ContributorWindowDays int       `json:"contributor_window_days,omitempty"`
}

// PaginationInfo represents pagination information
type PaginationInfo struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
}

const (
	maxNodes          = 10000
	processingTimeout = 30 * time.Second
)

// findForksInternal gets forks using the model layer directly
func findForksInternal(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, listOptions db.ListOptions) ([]*repo_model.Repository, int64, error) {
	// Use the model function directly to get forks
	forks, err := repo_model.GetRepositoriesByForkID(ctx, repo.ID)
	if err != nil {
		return nil, 0, err
	}

	// Filter by permissions
	filtered := make([]*repo_model.Repository, 0, len(forks))
	for _, fork := range forks {
		permission, err := access_model.GetUserRepoPermission(ctx, fork, doer)
		if err != nil {
			continue
		}
		if permission.HasAnyUnitAccessOrPublicAccess() {
			filtered = append(filtered, fork)
		}
	}

	// Apply pagination
	start := (listOptions.Page - 1) * listOptions.PageSize
	end := start + listOptions.PageSize
	if start > len(filtered) {
		return []*repo_model.Repository{}, int64(len(filtered)), nil
	}
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[start:end], int64(len(filtered)), nil
}

// BuildForkGraph builds the fork graph for a repository
func BuildForkGraph(ctx context.Context, repo *repo_model.Repository, params ForkGraphParams, doer *user_model.User) (*ForkGraphResponse, error) {

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, processingTimeout)
	defer cancel()

	// Initialize tracking
	visited := make(map[int64]bool)
	nodeCount := 0
	maxDepthReached := false

	// Build the tree
	rootNode, err := buildNode(timeoutCtx, repo, 0, params, doer, visited, &nodeCount, &maxDepthReached)
	if err != nil {
		return nil, err
	}

	// Count total and visible forks
	totalForks := repo.NumForks
	visibleForks := countVisibleForks(rootNode)

	// Build response
	response := &ForkGraphResponse{
		Root: rootNode,
		Metadata: GraphMetadata{
			TotalForks:      totalForks,
			VisibleForks:    visibleForks,
			MaxDepthReached: maxDepthReached,
			CacheStatus:     "miss",
			GeneratedAt:     time.Now(),
		},
	}

	if params.IncludeContributors {
		response.Metadata.ContributorWindowDays = params.ContributorDays
	}

	return response, nil
}

// buildNode recursively builds a fork node
func buildNode(ctx context.Context, repo *repo_model.Repository, level int, params ForkGraphParams, doer *user_model.User, visited map[int64]bool, nodeCount *int, maxDepthReached *bool) (*ForkNode, error) {
	// Check timeout
	select {
	case <-ctx.Done():
		return nil, ErrProcessingTimeout
	default:
	}

	// Check node limit
	if *nodeCount >= maxNodes {
		return nil, ErrTooManyNodes
	}

	// Check if already visited (cycle detection)
	if visited[repo.ID] {
		return nil, nil
	}
	visited[repo.ID] = true
	*nodeCount++

	// Check depth limit
	if level >= params.MaxDepth {
		*maxDepthReached = true
		return createLeafNode(ctx, repo, level, params, doer)
	}

	// Get direct forks
	forks, err := getDirectForks(ctx, repo.ID, doer, params)
	if err != nil {
		log.Error("Failed to get forks for repo %d: %v", repo.ID, err)
		return createLeafNode(ctx, repo, level, params, doer)
	}

	// Build children
	children := make([]*ForkNode, 0, len(forks))
	for _, fork := range forks {
		childNode, err := buildNode(ctx, fork, level+1, params, doer, visited, nodeCount, maxDepthReached)
		if err != nil {
			if errors.Is(err, ErrProcessingTimeout) || errors.Is(err, ErrTooManyNodes) {
				return nil, err
			}
			// Log error but continue with other children
			log.Error("Failed to build node for fork %d: %v", fork.ID, err)
			continue
		}
		if childNode != nil {
			children = append(children, childNode)
		}
	}

	// Create node
	node := &ForkNode{
		ID:       fmt.Sprintf("repo_%d", repo.ID),
		Level:    level,
		Children: children,
	}

	// Convert repository to API format
	permission, err := access_model.GetUserRepoPermission(ctx, repo, doer)
	if err != nil {
		return nil, err
	}
	node.Repository = convert.ToRepo(ctx, repo, permission)

	// Add contributor stats if requested
	if params.IncludeContributors {
		stats, err := getContributorStats(ctx, repo, params.ContributorDays)
		if err != nil {
			log.Warn("Failed to get contributor stats for repo %d: %v", repo.ID, err)
		} else {
			node.Contributors = stats
		}
	}

	return node, nil
}

// createLeafNode creates a leaf node without children
func createLeafNode(ctx context.Context, repo *repo_model.Repository, level int, params ForkGraphParams, doer *user_model.User) (*ForkNode, error) {
	node := &ForkNode{
		ID:       fmt.Sprintf("repo_%d", repo.ID),
		Level:    level,
		Children: []*ForkNode{},
	}

	permission, err := access_model.GetUserRepoPermission(ctx, repo, doer)
	if err != nil {
		return nil, err
	}
	node.Repository = convert.ToRepo(ctx, repo, permission)

	if params.IncludeContributors {
		stats, err := getContributorStats(ctx, repo, params.ContributorDays)
		if err != nil {
			log.Warn("Failed to get contributor stats for repo %d: %v", repo.ID, err)
		} else {
			node.Contributors = stats
		}
	}

	return node, nil
}

// getDirectForks gets direct forks of a repository with permission filtering
func getDirectForks(ctx context.Context, repoID int64, doer *user_model.User, params ForkGraphParams) ([]*repo_model.Repository, error) {
	// Get repository for FindForks
	repo := &repo_model.Repository{ID: repoID}

	// Use existing FindForks function with pagination
	listOpts := db.ListOptions{
		Page:     params.Page,
		PageSize: params.Limit,
	}

	// FindForks is defined in fork.go in the same package
	forks, _, err := findForksInternal(ctx, repo, doer, listOpts)
	if err != nil {
		return nil, err
	}

	// Filter by visibility if needed
	if !params.IncludePrivate {
		filtered := make([]*repo_model.Repository, 0, len(forks))
		for _, fork := range forks {
			if !fork.IsPrivate {
				filtered = append(filtered, fork)
			}
		}
		forks = filtered
	}

	// Sort forks
	sortRepositories(forks, params.Sort)

	return forks, nil
}

// sortRepositories sorts repositories based on the sort parameter
func sortRepositories(repos []*repo_model.Repository, sortBy string) {
	switch sortBy {
	case "updated":
		// Sort by updated time descending
		for i := 0; i < len(repos)-1; i++ {
			for j := i + 1; j < len(repos); j++ {
				if repos[i].UpdatedUnix < repos[j].UpdatedUnix {
					repos[i], repos[j] = repos[j], repos[i]
				}
			}
		}
	case "created":
		// Sort by created time descending
		for i := 0; i < len(repos)-1; i++ {
			for j := i + 1; j < len(repos); j++ {
				if repos[i].CreatedUnix < repos[j].CreatedUnix {
					repos[i], repos[j] = repos[j], repos[i]
				}
			}
		}
	case "stars":
		// Sort by stars descending
		for i := 0; i < len(repos)-1; i++ {
			for j := i + 1; j < len(repos); j++ {
				if repos[i].NumStars < repos[j].NumStars {
					repos[i], repos[j] = repos[j], repos[i]
				}
			}
		}
	case "forks":
		// Sort by forks descending
		for i := 0; i < len(repos)-1; i++ {
			for j := i + 1; j < len(repos); j++ {
				if repos[i].NumForks < repos[j].NumForks {
					repos[i], repos[j] = repos[j], repos[i]
				}
			}
		}
	}
}

// getContributorStatsInternal is a wrapper around the contributor stats service
// This avoids circular import issues
func getContributorStatsInternal(ctx context.Context, c cache.StringCache, repo *repo_model.Repository, revision string) (map[string]*ContributorData, error) {
	// Try to get from cache
	cacheKey := fmt.Sprintf("GetContributorStats/%s/%s", repo.FullName(), revision)

	var stats map[string]*ContributorData
	found, err := c.GetJSON(cacheKey, &stats)
	if err == nil && found {
		return stats, nil
	}

	// If not in cache, return error indicating stats need to be generated
	return nil, errAwaitGeneration
}

// getContributorStats gets contributor statistics for a repository
func getContributorStats(ctx context.Context, repo *repo_model.Repository, days int) (*ContributorStats, error) {
	// Use existing contributor stats service
	c := cache.GetCache()
	if c == nil {
		return &ContributorStats{TotalCount: 0, RecentCount: 0}, nil
	}

	stats, err := getContributorStatsInternal(ctx, c, repo, repo.DefaultBranch)
	if err != nil {
		// If contributor stats are not available, return zeros
		if errors.Is(err, errAwaitGeneration) {
			return &ContributorStats{TotalCount: 0, RecentCount: 0}, nil
		}
		return nil, err
	}

	// Count total contributors
	totalCount := len(stats)

	// Count recent contributors
	cutoffTime := time.Now().AddDate(0, 0, -days)
	recentCount := 0

	for _, contributor := range stats {
		// Check if contributor has commits in the time window
		for _, week := range contributor.Weeks {
			weekTime := time.UnixMilli(week.Week)
			if weekTime.After(cutoffTime) && week.Commits > 0 {
				recentCount++
				break
			}
		}
	}

	return &ContributorStats{
		TotalCount:  totalCount,
		RecentCount: recentCount,
	}, nil
}

// countVisibleForks counts the number of visible forks in the tree
func countVisibleForks(node *ForkNode) int {
	if node == nil {
		return 0
	}

	count := len(node.Children)
	for _, child := range node.Children {
		count += countVisibleForks(child)
	}

	return count
}
