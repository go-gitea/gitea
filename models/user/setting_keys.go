// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

const (
	// SettingsKeyHiddenCommentTypes is the setting key for hidden comment types
	SettingsKeyHiddenCommentTypes = "issue.hidden_comment_types"
	// SettingsKeyDiffWhitespaceBehavior is the setting key for whitespace behavior of diff
	SettingsKeyDiffWhitespaceBehavior = "diff.whitespace_behaviour"
	// SettingsKeyShowOutdatedComments is the setting key wether or not to show outdated comments in PRs
	SettingsKeyShowOutdatedComments = "comment_code.show_outdated"
	// UserActivityPubPrivPem is user's private key
	UserActivityPubPrivPem = "activitypub.priv_pem"
	// UserActivityPubPubPem is user's public key
	UserActivityPubPubPem = "activitypub.pub_pem"
	// SignupIP is the IP address that the user signed up with
	SignupIP = "signup.ip"
	// SignupUserAgent is the user agent that the user signed up with
	SignupUserAgent = "signup.user_agent"
)
