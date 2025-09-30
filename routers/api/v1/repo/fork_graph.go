// Copyright 2025 okTurtles Foundation. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/repository"
)

// ForkGraphParams represents the query parameters for fork graph endpoint
type ForkGraphParams struct {
	IncludeContributors bool   `form:"include_contributors"`
	ContributorDays     int    `form:"contributor_days"`
	MaxDepth            int    `form:"max_depth"`
	IncludePrivate      bool   `form:"include_private"`
	Sort                string `form:"sort"`
	Page                int    `form:"page"`
	Limit               int    `form:"limit"`
}

// setDefaults sets default values for parameters
func (p *ForkGraphParams) setDefaults() {
	if p.ContributorDays == 0 {
		p.ContributorDays = 90
	}
	if p.MaxDepth == 0 {
		p.MaxDepth = 10
	}
	if p.Sort == "" {
		p.Sort = "updated"
	}
	if p.Page == 0 {
		p.Page = 1
	}
	if p.Limit == 0 {
		p.Limit = 50
	}
}

// validate validates the parameters
func (p *ForkGraphParams) validate() error {
	if p.ContributorDays < 1 || p.ContributorDays > 365 {
		return fmt.Errorf("contributor_days must be between 1 and 365")
	}
	if p.MaxDepth < 1 || p.MaxDepth > 20 {
		return fmt.Errorf("max_depth must be between 1 and 20")
	}
	if p.Limit < 1 || p.Limit > 100 {
		return fmt.Errorf("limit must be between 1 and 100")
	}
	if p.Page < 1 {
		return fmt.Errorf("page must be at least 1")
	}
	validSorts := map[string]bool{"updated": true, "created": true, "stars": true, "forks": true}
	if !validSorts[p.Sort] {
		return fmt.Errorf("sort must be one of: updated, created, stars, forks")
	}
	return nil
}

// getCacheKey generates a cache key for the fork graph
func getCacheKey(repoID int64, params ForkGraphParams, userID int64) string {
	paramsHash := hashParams(params)
	return fmt.Sprintf("fork_graph:%d:%s:%d", repoID, paramsHash, userID)
}

// hashParams creates a hash of the parameters
func hashParams(params ForkGraphParams) string {
	data := fmt.Sprintf("%t:%d:%d:%t:%s:%d:%d",
		params.IncludeContributors, params.ContributorDays, params.MaxDepth,
		params.IncludePrivate, params.Sort, params.Page, params.Limit)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8]) // First 8 bytes for brevity
}

// getUserID safely gets the user ID
func getUserID(doer interface{}) int64 {
	if doer == nil {
		return 0
	}
	return doer.(int64)
}

// getCacheTTL returns the cache TTL based on repository and parameters
func getCacheTTL(isPrivate bool, includeContributors bool) time.Duration {
	if isPrivate {
		return 5 * time.Minute
	}
	if includeContributors {
		return 15 * time.Minute
	}
	return 30 * time.Minute
}

// GetForkGraph returns the fork graph for a repository
func GetForkGraph(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/forks/graph repository getForkGraph
	// ---
	// summary: Get repository fork graph
	// description: Returns a hierarchical tree structure of all forks with optional contributor statistics
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: include_contributors
	//   in: query
	//   description: Include contributor count for each fork
	//   type: boolean
	//   default: false
	// - name: contributor_days
	//   in: query
	//   description: Days to look back for contributor activity (1-365)
	//   type: integer
	//   default: 90
	// - name: max_depth
	//   in: query
	//   description: Maximum depth of fork tree traversal (1-20)
	//   type: integer
	//   default: 10
	// - name: include_private
	//   in: query
	//   description: Include private forks (requires appropriate permissions)
	//   type: boolean
	//   default: false
	// - name: sort
	//   in: query
	//   description: Sort order for child nodes (updated, created, stars, forks)
	//   type: string
	//   default: updated
	// - name: page
	//   in: query
	//   description: Page number for pagination
	//   type: integer
	//   default: 1
	// - name: limit
	//   in: query
	//   description: Number of forks per level per page (1-100)
	//   type: integer
	//   default: 50
	// responses:
	//   "200":
	//     "$ref": "#/responses/ForkGraph"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// Parse query parameters with defaults
	params := ForkGraphParams{
		IncludeContributors: ctx.FormBool("include_contributors"),
		ContributorDays:     90,  // default
		MaxDepth:            10,  // default
		IncludePrivate:      ctx.FormBool("include_private"),
		Sort:                "updated", // default
		Page:                1,   // default
		Limit:               50,  // default
	}

	// Override defaults if parameters are explicitly provided
	if ctx.FormString("contributor_days") != "" {
		params.ContributorDays = ctx.FormInt("contributor_days")
	}
	if ctx.FormString("max_depth") != "" {
		params.MaxDepth = ctx.FormInt("max_depth")
	}
	if ctx.FormString("sort") != "" {
		params.Sort = ctx.FormString("sort")
	}
	if ctx.FormString("page") != "" {
		params.Page = ctx.FormInt("page")
	}
	if ctx.FormString("limit") != "" {
		params.Limit = ctx.FormInt("limit")
	}

	if err := params.validate(); err != nil {
		ctx.APIError(http.StatusBadRequest, err)
		return
	}

	// Check repository access
	if !ctx.Repo.Permission.HasAnyUnitAccessOrPublicAccess() {
		ctx.APIErrorNotFound()
		return
	}

	// Get user ID for cache key
	var userID int64
	if ctx.Doer != nil {
		userID = ctx.Doer.ID
	}

	// Try cache first
	cacheKey := getCacheKey(ctx.Repo.Repository.ID, params, userID)
	c := cache.GetCache()
	if c != nil {
		var cachedResponse repository.ForkGraphResponse
		found, err := c.GetJSON(cacheKey, &cachedResponse)
		if err == nil && found {
			cachedResponse.Metadata.CacheStatus = "hit"
			ctx.JSON(http.StatusOK, cachedResponse)
			return
		}
	}

	// Convert params to service params
	serviceParams := repository.ForkGraphParams{
		IncludeContributors: params.IncludeContributors,
		ContributorDays:     params.ContributorDays,
		MaxDepth:            params.MaxDepth,
		IncludePrivate:      params.IncludePrivate,
		Sort:                params.Sort,
		Page:                params.Page,
		Limit:               params.Limit,
	}

	// Generate graph
	graph, err := repository.BuildForkGraph(ctx, ctx.Repo.Repository, serviceParams, ctx.Doer)
	if err != nil {
		handleForkGraphError(ctx, err)
		return
	}

	// Set cache status
	graph.Metadata.CacheStatus = "miss"

	// Cache result
	if c != nil {
		ttl := getCacheTTL(ctx.Repo.Repository.IsPrivate, params.IncludeContributors)
		_ = c.PutJSON(cacheKey, graph, int64(ttl.Seconds()))
	}

	ctx.JSON(http.StatusOK, graph)
}

// handleForkGraphError handles errors from fork graph generation
func handleForkGraphError(ctx *context.APIContext, err error) {
	switch {
	case repository.IsErrMaxDepthExceeded(err):
		ctx.APIError(http.StatusBadRequest, err)
	case repository.IsErrTooManyNodes(err):
		ctx.APIError(http.StatusBadRequest, err)
	case repository.IsErrProcessingTimeout(err):
		ctx.APIError(http.StatusRequestTimeout, err)
	default:
		ctx.APIErrorInternal(err)
	}
}
