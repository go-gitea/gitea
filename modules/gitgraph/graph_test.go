// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitgraph

import (
	"fmt"
	"testing"

	"code.gitea.io/gitea/modules/git"
)

func BenchmarkGetCommitGraph(b *testing.B) {

	currentRepo, err := git.OpenRepository(".")
	if err != nil {
		b.Error("Could not open repository")
	}
	defer currentRepo.Close()

	for i := 0; i < b.N; i++ {
		graph, err := GetCommitGraph(currentRepo, 1)
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

func TestCommitStringParsing(t *testing.T) {
	dataFirstPart := "* DATA:||4e61bacab44e9b4730e44a6615d04098dd3a8eaf|2016-12-20 21:10:41 +0100|Author|user@mail.something|4e61bac|"
	tests := []struct {
		shouldPass    bool
		testName      string
		commitMessage string
	}{
		{true, "normal", "not a fancy message"},
		{true, "extra pipe", "An extra pipe: |"},
		{true, "extra 'Data:'", "DATA: might be trouble"},
	}

	for _, test := range tests {

		t.Run(test.testName, func(t *testing.T) {
			testString := fmt.Sprintf("%s%s", dataFirstPart, test.commitMessage)
			graphItem, err := graphItemFromString(testString, nil)
			if err != nil && test.shouldPass {
				t.Errorf("Could not parse %s", testString)
				return
			}

			if test.commitMessage != graphItem.Subject {
				t.Errorf("%s does not match %s", test.commitMessage, graphItem.Subject)
			}
		})
	}
}
