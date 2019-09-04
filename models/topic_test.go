// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddTopic(t *testing.T) {
	totalNrOfTopics := 6
	repo1NrOfTopics := 3
	repo2NrOfTopics := 2

	assert.NoError(t, PrepareTestDatabase())

	topics, err := FindTopics(&FindTopicOptions{})
	assert.NoError(t, err)
	assert.EqualValues(t, totalNrOfTopics, len(topics))

	topics, err = FindTopics(&FindTopicOptions{
		Limit: 2,
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 2, len(topics))

	topics, err = FindTopics(&FindTopicOptions{
		RepoID: 1,
	})
	assert.NoError(t, err)
	assert.EqualValues(t, repo1NrOfTopics, len(topics))

	assert.NoError(t, SaveTopics(2, "golang"))
	repo2NrOfTopics = 1
	topics, err = FindTopics(&FindTopicOptions{})
	assert.NoError(t, err)
	assert.EqualValues(t, totalNrOfTopics, len(topics))

	topics, err = FindTopics(&FindTopicOptions{
		RepoID: 2,
	})
	assert.NoError(t, err)
	assert.EqualValues(t, repo2NrOfTopics, len(topics))

	assert.NoError(t, SaveTopics(2, "golang", "gitea"))
	repo2NrOfTopics = 2
	totalNrOfTopics++
	topic, err := GetTopicByName("gitea")
	assert.NoError(t, err)
	assert.EqualValues(t, 1, topic.RepoCount)

	topics, err = FindTopics(&FindTopicOptions{})
	assert.NoError(t, err)
	assert.EqualValues(t, totalNrOfTopics, len(topics))

	topics, err = FindTopics(&FindTopicOptions{
		RepoID: 2,
	})
	assert.NoError(t, err)
	assert.EqualValues(t, repo2NrOfTopics, len(topics))
}

func TestTopicValidator(t *testing.T) {
	assert.True(t, ValidateTopic("12345"))
	assert.True(t, ValidateTopic("2-test"))
	assert.True(t, ValidateTopic("test-3"))
	assert.True(t, ValidateTopic("first"))
	assert.True(t, ValidateTopic("second-test-topic"))
	assert.True(t, ValidateTopic("third-project-topic-with-max-length"))

	assert.False(t, ValidateTopic("$fourth-test,topic"))
	assert.False(t, ValidateTopic("-fifth-test-topic"))
	assert.False(t, ValidateTopic("sixth-go-project-topic-with-excess-length"))
}
