// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"mvdan.cc/xurls/v2"
)

// NOTE: All below regex matching do not perform any extra validation.
// Thus a link is produced even if the linked entity does not exist.
// While fast, this is also incorrect and lead to false positives.
// TODO: fix invalid linking issue

// LinkRegex is a regexp matching a valid link
var LinkRegex, _ = xurls.StrictMatchingScheme("https?://")
