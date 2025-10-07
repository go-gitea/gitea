// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// TopicResponse for returning topics
type TopicResponse struct {
	// The unique identifier of the topic
	ID int64 `json:"id"`
	// The name of the topic
	Name string `json:"topic_name"`
	// The number of repositories using this topic
	RepoCount int `json:"repo_count"`
	// The date and time when the topic was created
	Created time.Time `json:"created"`
	// The date and time when the topic was last updated
	Updated time.Time `json:"updated"`
}

// TopicName a list of repo topic names
type TopicName struct {
	// List of topic names
	TopicNames []string `json:"topics"`
}

// RepoTopicOptions a collection of repo topic names
type RepoTopicOptions struct {
	// list of topic names
	Topics []string `json:"topics"`
}
