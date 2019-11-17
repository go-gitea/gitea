// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitgraph

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
)

// GraphItem represent one commit, or one relation in timeline
type GraphItem struct {
	GraphAcii    string
	Relation     string
	Branch       string
	Rev          string
	Date         string
	Author       string
	AuthorEmail  string
	ShortRev     string
	Subject      string
	OnlyRelation bool
}

// GraphItems is a list of commits from all branches
type GraphItems []GraphItem

// GetCommitGraph return a list of commit (GraphItems) from all branches
func GetCommitGraph(r *git.Repository, page int) (GraphItems, error) {

	var CommitGraph []GraphItem

	format := "DATA:|%d|%H|%ad|%an|%ae|%h|%s"

	graphCmd := git.NewCommand("log")
	graphCmd.AddArguments("--graph",
		"--date-order",
		"--all",
		"-C",
		"-M",
		fmt.Sprintf("-n %d", setting.UI.GraphMaxCommitNum),
		fmt.Sprintf("--skip=%d", setting.UI.GraphMaxCommitNum*(page-1)),
		"--date=iso",
		fmt.Sprintf("--pretty=format:%s", format),
	)
	graph, err := graphCmd.RunInDir(r.Path)
	if err != nil {
		return CommitGraph, err
	}

	CommitGraph = make([]GraphItem, 0, 100)
	for _, s := range strings.Split(graph, "\n") {
		GraphItem, err := graphItemFromString(s, r)
		if err != nil {
			return CommitGraph, err
		}
		CommitGraph = append(CommitGraph, GraphItem)
	}

	return CommitGraph, nil
}

func graphItemFromString(s string, r *git.Repository) (GraphItem, error) {

	var ascii string
	var data = "|||||||"
	lines := strings.SplitN(s, "DATA:", 2)

	switch len(lines) {
	case 1:
		ascii = lines[0]
	case 2:
		ascii = lines[0]
		data = lines[1]
	default:
		return GraphItem{}, fmt.Errorf("Failed parsing grap line:%s. Expect 1 or two fields", s)
	}

	rows := strings.SplitN(data, "|", 8)
	if len(rows) < 8 {
		return GraphItem{}, fmt.Errorf("Failed parsing grap line:%s - Should containt 8 datafields", s)
	}

	/* // see format in getCommitGraph()
	   0	Relation string
	   1	Branch string
	   2	Rev string
	   3	Date string
	   4	Author string
	   5	AuthorEmail string
	   6	ShortRev string
	   7	Subject string
	*/
	gi := GraphItem{ascii,
		rows[0],
		rows[1],
		rows[2],
		rows[3],
		rows[4],
		rows[5],
		rows[6],
		rows[7],
		len(rows[2]) == 0, // no commits referred to, only relation in current line.
	}
	return gi, nil
}
