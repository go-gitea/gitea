// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/indexer/internal"
	"code.gitea.io/gitea/modules/log"
)

func FilenameIndexerID(repoID int64, isWiki bool, filename string) string {
	t := "r"
	if isWiki {
		t = "w"
	}
	return internal.Base36(repoID) + "_" + t + "_" + filename
}

func ParseIndexerID(indexerID string) (int64, bool, string, error) {
	parts := strings.SplitN(indexerID, "_", 3)
	if len(parts) != 3 {
		return 0, false, "", fmt.Errorf("unexpected ID in repo indexer: %s", indexerID)
	}
	repoID, _ := internal.ParseBase36(parts[0])
	isWiki := parts[1] == "w"
	return repoID, isWiki, parts[2], nil
}

func FilenameOfIndexerID(indexerID string) string {
	_, _, name, err := ParseIndexerID(indexerID)
	if err != nil {
		log.Error(err.Error())
	}
	return name
}
