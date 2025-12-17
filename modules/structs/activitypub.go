// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// ActivityPub type
type ActivityPub struct {
	// Context defines the JSON-LD context for ActivityPub
	Context string `json:"@context"`
}
