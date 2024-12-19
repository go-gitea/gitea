// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import "code.gitea.io/gitea/models/db"

// OrderByMap represents all possible search order
var OrderByMap = map[string]map[string]db.SearchOrderBy{
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

// OrderByFlatMap is similar to OrderByMap but use human language keywords
// to decide between asc and desc
var OrderByFlatMap = map[string]db.SearchOrderBy{
	"newest":                OrderByMap["desc"]["created"],
	"oldest":                OrderByMap["asc"]["created"],
	"recentupdate":          OrderByMap["desc"]["updated"],
	"leastupdate":           OrderByMap["asc"]["updated"],
	"reversealphabetically": OrderByMap["desc"]["alpha"],
	"alphabetically":        OrderByMap["asc"]["alpha"],
	"reversesize":           OrderByMap["desc"]["size"],
	"size":                  OrderByMap["asc"]["size"],
	"reversegitsize":        OrderByMap["desc"]["git_size"],
	"gitsize":               OrderByMap["asc"]["git_size"],
	"reverselfssize":        OrderByMap["desc"]["lfs_size"],
	"lfssize":               OrderByMap["asc"]["lfs_size"],
	"moststars":             OrderByMap["desc"]["stars"],
	"feweststars":           OrderByMap["asc"]["stars"],
	"mostforks":             OrderByMap["desc"]["forks"],
	"fewestforks":           OrderByMap["asc"]["forks"],
}
