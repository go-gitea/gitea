// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitgraph

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
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
	format := "DATA:|%d|%H|%ad|%an|%ae|%h|%s"

	if page == 0 {
		page = 1
	}

	graphCmd := git.NewCommand("log")
	graphCmd.AddArguments("--graph",
		"--date-order",
		"--all",
		"-C",
		"-M",
		fmt.Sprintf("-n %d", setting.UI.GraphMaxCommitNum*page),
		"--date=iso",
		fmt.Sprintf("--pretty=format:%s", format),
	)
	commitGraph := make([]GraphItem, 0, 100)
	stderr := new(strings.Builder)
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	commitsToSkip := setting.UI.GraphMaxCommitNum * (page - 1)

	scanner := bufio.NewScanner(stdoutReader)

	if err := graphCmd.RunInDirTimeoutEnvFullPipelineFunc(nil, -1, r.Path, stdoutWriter, stderr, nil, func(ctx context.Context, cancel context.CancelFunc) error {
		_ = stdoutWriter.Close()
		defer stdoutReader.Close()
		for commitsToSkip > 0 && scanner.Scan() {
			line := scanner.Bytes()
			dataIdx := bytes.Index(line, []byte("DATA:"))
			starIdx := bytes.IndexByte(line, '*')
			if starIdx >= 0 && starIdx < dataIdx {
				commitsToSkip--
			}
		}
		// Skip initial non-commit lines
		for scanner.Scan() {
			if bytes.IndexByte(scanner.Bytes(), '*') >= 0 {
				line := scanner.Text()
				graphItem, err := graphItemFromString(line, r)
				if err != nil {
					cancel()
					return err
				}
				commitGraph = append(commitGraph, graphItem)
				break
			}
		}

		for scanner.Scan() {
			line := scanner.Text()
			graphItem, err := graphItemFromString(line, r)
			if err != nil {
				cancel()
				return err
			}
			commitGraph = append(commitGraph, graphItem)
		}
		return scanner.Err()
	}); err != nil {
		return commitGraph, err
	}

	return commitGraph, nil
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
