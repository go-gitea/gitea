// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
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
	// responses:
	//   "200":
	//     "$ref": "#/responses/TopicNames"

	topics, err := models.FindTopics(&models.FindTopicOptions{
		RepoID: ctx.Repo.Repository.ID,
	})
	if err != nil {
		log.Error("ListTopics failed: %v", err)
		ctx.InternalServerError(err)
		return
	}

	topicNames := make([]string, len(topics))
	for i, topic := range topics {
		topicNames[i] = topic.Name
	}
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"topics": topicNames,
	})
}

// UpdateTopics updates repo with a new set of topics
func UpdateTopics(ctx *context.APIContext, form api.RepoTopicOptions) {
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
	//   "422":
	//     "$ref": "#/responses/invalidTopicsError"

	topicNames := form.Topics
	validTopics, invalidTopics := models.SanitizeAndValidateTopics(topicNames)

	if len(validTopics) > 25 {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]interface{}{
			"invalidTopics": nil,
			"message":       "Exceeding maximum number of topics per repo",
		})
		return
	}

	if len(invalidTopics) > 0 {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]interface{}{
			"invalidTopics": invalidTopics,
			"message":       "Topic names are invalid",
		})
		return
	}

	err := models.SaveTopics(ctx.Repo.Repository.ID, validTopics...)
	if err != nil {
		log.Error("SaveTopics failed: %v", err)
		ctx.InternalServerError(err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// AddTopic adds a topic name to a repo
func AddTopic(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/topics/{topic} repository repoAddTopíc
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
	//   "422":
	//     "$ref": "#/responses/invalidTopicsError"

	topicName := strings.TrimSpace(strings.ToLower(ctx.Params(":topic")))

	if !models.ValidateTopic(topicName) {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]interface{}{
			"invalidTopics": topicName,
			"message":       "Topic name is invalid",
		})
		return
	}

	// Prevent adding more topics than allowed to repo
	topics, err := models.FindTopics(&models.FindTopicOptions{
		RepoID: ctx.Repo.Repository.ID,
	})
	if err != nil {
		log.Error("AddTopic failed: %v", err)
		ctx.InternalServerError(err)
		return
	}
	if len(topics) >= 25 {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]interface{}{
			"message": "Exceeding maximum allowed topics per repo.",
		})
		return
	}

	_, err = models.AddTopic(ctx.Repo.Repository.ID, topicName)
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
	//   "422":
	//     "$ref": "#/responses/invalidTopicsError"

	topicName := strings.TrimSpace(strings.ToLower(ctx.Params(":topic")))

	if !models.ValidateTopic(topicName) {
		ctx.JSON(http.StatusUnprocessableEntity, map[string]interface{}{
			"invalidTopics": topicName,
			"message":       "Topic name is invalid",
		})
		return
	}

	topic, err := models.DeleteTopic(ctx.Repo.Repository.ID, topicName)
	if err != nil {
		log.Error("DeleteTopic failed: %v", err)
		ctx.InternalServerError(err)
		return
	}

	if topic == nil {
		ctx.NotFound()
	}

	ctx.Status(http.StatusNoContent)
}

// TopicSearch search for creating topic
func TopicSearch(ctx *context.Context) {
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
	// responses:
	//   "200":
	//     "$ref": "#/responses/TopicListResponse"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	if ctx.User == nil {
		ctx.Error(http.StatusForbidden, "UserIsNil", "Only owners could change the topics.")
		return
	}

	kw := ctx.Query("q")

	topics, err := models.FindTopics(&models.FindTopicOptions{
		Keyword: kw,
		Limit:   10,
	})
	if err != nil {
		log.Error("SearchTopics failed: %v", err)
		ctx.InternalServerError(err)
		return
	}

	topicResponses := make([]*api.TopicResponse, len(topics))
	for i, topic := range topics {
		topicResponses[i] = convert.ToTopicResponse(topic)
	}
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"topics": topicResponses,
	})
}
