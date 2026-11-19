// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package websocket

import "gitea.dev/services/pubsub"

func PullHeadEvent(headCommitID string) []byte {
	return makeEventMessage("pull-head", "", headCommitID)
}

func PublishPullHead(prID int64, headCommitID string) {
	topic := pubsub.PullHeadTopic(prID)
	if pubsub.DefaultBroker.HasTopicSubscribers(topic) {
		pubsub.DefaultBroker.Publish(topic, PullHeadEvent(headCommitID))
	}
}
