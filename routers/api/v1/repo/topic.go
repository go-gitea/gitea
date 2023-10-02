// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/convert"
)

// ListTopics returns list of current topics for repo
func ListTopics(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/topics repository repoListTopics
	// ---
	// summary: Get list of topics that a repository has
	// produces:
	//   - application/json
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
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/TopicNames"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opts := &repo_model.FindTopicOptions{
		ListOptions: utils.GetListOptions(ctx),
		RepoID:      ctx.Repo.Repository.ID,
	}

	topics, total, err := repo_model.FindTopics(ctx, opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	topicNames := make([]string, len(topics))
	for i, topic := range topics {
		topicNames[i] = topic.Name
	}

	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, map[string]any{
		"topics": topicNames,
	})
}

// UpdateTopics updates repo with a new set of topics
func UpdateTopics(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/topics repository repoUpdateTopics
	// ---
	// summary: Replace list of topics for a repository
	// produces:
	//   - application/json
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
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/RepoTopicOptions"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/invalidTopicsError"

	form := web.GetForm(ctx).(*api.RepoTopicOptions)
	topicNames := form.Topics
	validTopics, invalidTopics := repo_model.SanitizeAndValidateTopics(topicNames)

	if len(validTopics) > 25 {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]any{
			"invalidTopics": nil,
			"message":       "Exceeding maximum number of topics per repo",
		})
		return
	}

	if len(invalidTopics) > 0 {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]any{
			"invalidTopics": invalidTopics,
			"message":       "Topic names are invalid",
		})
		return
	}

	err := repo_model.SaveTopics(ctx, ctx.Repo.Repository.ID, validTopics...)
	if err != nil {
		log.Error("SaveTopics failed: %v", err)
		ctx.InternalServerError(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// AddTopic adds a topic name to a repo
func AddTopic(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/topics/{topic} repository repoAddTopic
	// ---
	// summary: Add a topic to a repository
	// produces:
	//   - application/json
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
	// - name: topic
	//   in: path
	//   description: name of the topic to add
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/invalidTopicsError"

	topicName := strings.TrimSpace(strings.ToLower(ctx.Params(":topic")))

	if !repo_model.ValidateTopic(topicName) {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]any{
			"invalidTopics": topicName,
			"message":       "Topic name is invalid",
		})
		return
	}

	// Prevent adding more topics than allowed to repo
	count, err := repo_model.CountTopics(ctx, &repo_model.FindTopicOptions{
		RepoID: ctx.Repo.Repository.ID,
	})
	if err != nil {
		log.Error("CountTopics failed: %v", err)
		ctx.InternalServerError(err)
		return
	}
	if count >= 25 {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]any{
			"message": "Exceeding maximum allowed topics per repo.",
		})
		return
	}

	_, err = repo_model.AddTopic(ctx, ctx.Repo.Repository.ID, topicName)
	if err != nil {
		log.Error("AddTopic failed: %v", err)
		ctx.InternalServerError(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// DeleteTopic removes topic name from repo
func DeleteTopic(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/topics/{topic} repository repoDeleteTopic
	// ---
	// summary: Delete a topic from a repository
	// produces:
	//   - application/json
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
	// - name: topic
	//   in: path
	//   description: name of the topic to delete
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/invalidTopicsError"

	topicName := strings.TrimSpace(strings.ToLower(ctx.Params(":topic")))

	if !repo_model.ValidateTopic(topicName) {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]any{
			"invalidTopics": topicName,
			"message":       "Topic name is invalid",
		})
		return
	}

	topic, err := repo_model.DeleteTopic(ctx, ctx.Repo.Repository.ID, topicName)
	if err != nil {
		log.Error("DeleteTopic failed: %v", err)
		ctx.InternalServerError(err)
		return
	}

	if topic == nil {
		ctx.NotFound()
		return
	}

	ctx.Status(http.StatusNoContent)
}

// TopicSearch search for creating topic
func TopicSearch(ctx *context.APIContext) {
	// swagger:operation GET /topics/search repository topicSearch
	// ---
	// summary: search topics via keyword
	// produces:
	//   - application/json
	// parameters:
	//   - name: q
	//     in: query
	//     description: keywords to search
	//     required: true
	//     type: string
	//   - name: page
	//     in: query
	//     description: page number of results to return (1-based)
	//     type: integer
	//   - name: limit
	//     in: query
	//     description: page size of results
	//     type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/TopicListResponse"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opts := &repo_model.FindTopicOptions{
		Keyword:     ctx.FormString("q"),
		ListOptions: utils.GetListOptions(ctx),
	}

	topics, total, err := repo_model.FindTopics(ctx, opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	topicResponses := make([]*api.TopicResponse, len(topics))
	for i, topic := range topics {
		topicResponses[i] = convert.ToTopicResponse(topic)
	}

	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, map[string]any{
		"topics": topicResponses,
	})
}
