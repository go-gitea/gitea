// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import "code.gitea.io/gitea/models/db"

// SearchOrderByMap represents all possible search order
var SearchOrderByMap = map[string]map[string]db.SearchOrderBy{
	"asc": {
		"alpha":    "owner_name ASC, name ASC",
		"created":  db.SearchOrderByOldest,
		"updated":  db.SearchOrderByLeastUpdated,
		"size":     "size ASC",
		"git_size": "git_size ASC",
		"lfs_size": "lfs_size ASC",
		"id":       db.SearchOrderByID,
		"stars":    db.SearchOrderByStars,
		"forks":    db.SearchOrderByForks,
	},
	"desc": {
		"alpha":    "owner_name DESC, name DESC",
		"created":  db.SearchOrderByNewest,
		"updated":  db.SearchOrderByRecentUpdated,
		"size":     "size DESC",
		"git_size": "git_size DESC",
		"lfs_size": "lfs_size DESC",
		"id":       db.SearchOrderByIDReverse,
		"stars":    db.SearchOrderByStarsReverse,
		"forks":    db.SearchOrderByForksReverse,
	},
}

// SearchOrderByFlatMap is similar to SearchOrderByMap but use human language keywords
// to decide between asc and desc
var SearchOrderByFlatMap = map[string]db.SearchOrderBy{
	"newest":                SearchOrderByMap["desc"]["created"],
	"oldest":                SearchOrderByMap["asc"]["created"],
	"leastupdate":           SearchOrderByMap["asc"]["updated"],
	"reversealphabetically": SearchOrderByMap["desc"]["alpha"],
	"alphabetically":        SearchOrderByMap["asc"]["alpha"],
	"reversesize":           SearchOrderByMap["desc"]["size"],
	"size":                  SearchOrderByMap["asc"]["size"],
	"reversegitsize":        SearchOrderByMap["desc"]["git_size"],
	"gitsize":               SearchOrderByMap["asc"]["git_size"],
	"reverselfssize":        SearchOrderByMap["desc"]["lfs_size"],
	"lfssize":               SearchOrderByMap["asc"]["lfs_size"],
	"moststars":             SearchOrderByMap["desc"]["stars"],
	"feweststars":           SearchOrderByMap["asc"]["stars"],
	"mostforks":             SearchOrderByMap["desc"]["forks"],
	"fewestforks":           SearchOrderByMap["asc"]["forks"],
}
