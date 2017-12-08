// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
)

// RemoveUsernameParameterSuffix returns the username parameter without the (fullname) suffix - leaving just the username
func RemoveUsernameParameterSuffix(name string) string {
	if index := strings.Index(name, " ("); index >= 0 {
		name = name[:index]
	}
	return name
}

// ParseRepoOrderByType returns the OrderByType represented by the given string.
func ParseRepoOrderByType(s string) models.RepoOrderByType {
	switch s {
	case "alphabetically":
		return models.RepoOrderByAlphabetically
	case "reversealphabetically":
		return models.RepoOrderByAlphabeticallyReverse
	case "leastupdate":
		return models.RepoOrderByLeastUpdated
	case "recentupdate":
		return models.RepoOrderByRecentUpdated
	case "newest":
		return models.RepoOrderByNewest
	case "oldest":
		return models.RepoOrderByOldest
	case "size":
		return models.RepoOrderBySize
	case "reversesize":
		return models.RepoOrderBySizeReverse
	case "id":
		return models.RepoOrderByID
	default:
		return models.RepoOrderByRecentUpdated
	}
}

// ToQueryString returns the string equivalent of the given RepoOrderByType, for
// use in URL queries and HTML templates.
func ToQueryString(s models.RepoOrderByType) string {
	switch s {
	case models.RepoOrderByAlphabetically:
		return "alphabetically"
	case models.RepoOrderByAlphabeticallyReverse:
		return "reversealphabetically"
	case models.RepoOrderByLeastUpdated:
		return "leastupdate"
	case models.RepoOrderByRecentUpdated:
		return "recentupdate"
	case models.RepoOrderByNewest:
		return "newest"
	case models.RepoOrderByOldest:
		return "oldest"
	case models.RepoOrderBySize:
		return "size"
	case models.RepoOrderBySizeReverse:
		return "reversesize"
	case models.RepoOrderByID:
		return "id"
	default:
		log.Error(4, "Unrecognized models.RepoOrderBy: %v", s)
		return ""
	}
}
