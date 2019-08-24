// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/convert"
)

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
	//     "$ref": "#/responses/TopicListResponse"

	topics, err := models.FindTopics(&models.FindTopicOptions{
		RepoID: ctx.Repo.Repository.ID,
	})
	if err != nil {
		log.Error("ListTopics failed: %v", err)
		ctx.JSON(500, map[string]interface{}{
			"message": "ListTopics failed.",
		})
		return
	}

	topicResponses := make([]*api.TopicResponse, len(topics))
	for i, topic := range topics {
		topicResponses[i] = convert.ToTopicResponse(topic)
	}
	ctx.JSON(200, map[string]interface{}{
		"topics": topicResponses,
	})
}

func HasTopic(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/topics/{topic} repository repoHasTopic
	// ---
	// summary: Check if a repository has topic
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
	//   description: name of the topic to check for
	//   type: string
	//   required: true
	// responses:
	//   "201":
	//     "$ref": "#/responses/TopicResponse"
	//   "404":
	//     "$ref": "#/responses/empty"
	topicName := strings.TrimSpace(strings.ToLower(ctx.Params(":topic")))

	topics, err := models.FindTopics(&models.FindTopicOptions{
		RepoID:  ctx.Repo.Repository.ID,
		Keyword: topicName,
	})
	if err != nil {
		log.Error("HasTopic failed: %v", err)
		ctx.JSON(500, map[string]interface{}{
			"message": "HasTopic failed.",
		})
		return
	}

	if len(topics) == 0 {
		ctx.NotFound()
	}

	ctx.JSON(200, map[string]interface{}{
		"topic": convert.ToTopicResponse(topics[0]),
	})
}

func AddTopic(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/topics/{topic} repository repoAddTopíc
	// ---
	// summary: Add a topic from a repository
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
	//   "201":
	//     "$ref": "#/responses/TopicResponse"

	topicName := strings.TrimSpace(strings.ToLower(ctx.Params(":topic")))

	if !models.ValidateTopic(topicName) {
		ctx.Error(http.StatusUnprocessableEntity, "", "Topic name is invalid")
		return
	}

	topic, err := models.AddTopic(ctx.Repo.Repository.ID, topicName)
	if err != nil {
		log.Error("AddTopic failed: %v", err)
		ctx.JSON(500, map[string]interface{}{
			"message": "AddTopic failed.",
		})
		return
	}

	ctx.JSON(201, map[string]interface{}{
		"topic": convert.ToTopicResponse(topic),
	})
}

func DeleteTopic(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/topics/{topic} repository repoDeleteTopic
	// ---
	// summary: delete a topic from a repository
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
	//   "201":
	//     "$ref": "#/responses/TopicResponse"
	topicName := strings.TrimSpace(strings.ToLower(ctx.Params(":topic")))

	if !models.ValidateTopic(topicName) {
		ctx.Error(http.StatusUnprocessableEntity, "", "Topic name is invalid")
		return
	}

	topic, err := models.DeleteTopic(ctx.Repo.Repository.ID, topicName)
	if err != nil {
		log.Error("DeleteTopic failed: %v", err)
		ctx.JSON(500, map[string]interface{}{
			"message": "DeleteTopic failed.",
		})
		return
	}

	if topic == nil {
		ctx.NotFound()
	}

	ctx.JSON(201, map[string]interface{}{
		"topic": convert.ToTopicResponse(topic),
	})
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
	if ctx.User == nil {
		ctx.JSON(403, map[string]interface{}{
			"message": "Only owners could change the topics.",
		})
		return
	}

	kw := ctx.Query("q")

	topics, err := models.FindTopics(&models.FindTopicOptions{
		Keyword: kw,
		Limit:   10,
	})
	if err != nil {
		log.Error("SearchTopics failed: %v", err)
		ctx.JSON(500, map[string]interface{}{
			"message": "Search topics failed.",
		})
		return
	}

	topicResponses := make([]*api.TopicResponse, len(topics))
	for i, topic := range topics {
		topicResponses[i] = convert.ToTopicResponse(topic)
	}
	ctx.JSON(200, map[string]interface{}{
		"topics": topicResponses,
	})
}
