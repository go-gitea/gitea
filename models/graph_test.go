// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/git"
)

func BenchmarkGetCommitGraph(b *testing.B) {

	currentRepo, err := git.OpenRepository(".")
	if err != nil {
		b.Error("Could not open repository")
	}

	for i := 0; i < b.N; i++ {
		graph, err := GetCommitGraph(currentRepo)
		if err != nil {
			b.Error("Could get commit graph")
		}

		if len(graph) < 100 {
			b.Error("Should get 100 log lines.")
		}
	}
}

func BenchmarkParseCommitString(b *testing.B) {
	testString := "* DATA:||4e61bacab44e9b4730e44a6615d04098dd3a8eaf|2016-12-20 21:10:41 +0100|Kjell Kvinge|kjell@kvinge.biz|4e61bac|Add route for graph"

	for i := 0; i < b.N; i++ {
		graphItem, err := graphItemFromString(testString, nil)
		if err != nil {
			b.Error("could not parse teststring")
		}

		if graphItem.Author != "Kjell Kvinge" {
			b.Error("Did not get expected data")
		}
	}
}
