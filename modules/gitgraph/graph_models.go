// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitgraph

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

// NewGraph creates a basic graph
func NewGraph() *Graph {
	graph := &Graph{}
	graph.relationCommit = &Commit{
		Row:    -1,
		Column: -1,
	}
	graph.Flows = map[int64]*Flow{}
	return graph
}

// Graph represents a collection of flows
type Graph struct {
	Flows          map[int64]*Flow
	Commits        []*Commit
	MinRow         int
	MinColumn      int
	MaxRow         int
	MaxColumn      int
	relationCommit *Commit
}

// Width returns the width of the graph
func (graph *Graph) Width() int {
	return graph.MaxColumn - graph.MinColumn + 1
}

// Height returns the height of the graph
func (graph *Graph) Height() int {
	return graph.MaxRow - graph.MinRow + 1
}

// AddGlyph adds glyph to flows
func (graph *Graph) AddGlyph(row, column int, flowID int64, color int, glyph byte) {
	flow, ok := graph.Flows[flowID]
	if !ok {
		flow = NewFlow(flowID, color, row, column)
		graph.Flows[flowID] = flow
	}
	flow.AddGlyph(row, column, glyph)

	if row < graph.MinRow {
		graph.MinRow = row
	}
	if row > graph.MaxRow {
		graph.MaxRow = row
	}
	if column < graph.MinColumn {
		graph.MinColumn = column
	}
	if column > graph.MaxColumn {
		graph.MaxColumn = column
	}
}

// AddCommit adds a commit at row, column on flowID with the provided data
func (graph *Graph) AddCommit(row, column int, flowID int64, data []byte) error {
	commit, err := NewCommit(row, column, data)
	if err != nil {
		return err
	}
	commit.Flow = flowID
	graph.Commits = append(graph.Commits, commit)

	graph.Flows[flowID].Commits = append(graph.Flows[flowID].Commits, commit)
	return nil
}

// LoadAndProcessCommits will load the git.Commits for each commit in the graph,
// the associate the commit with the user author, and check the commit verification
// before finally retrieving the latest status
func (graph *Graph) LoadAndProcessCommits(ctx context.Context, repository *repo_model.Repository, gitRepo *git.Repository) error {
	var err error
	var ok bool

	emails := map[string]*user_model.User{}
	keyMap := map[string]bool{}

	for _, c := range graph.Commits {
		if len(c.Rev) == 0 {
			continue
		}
		c.Commit, err = gitRepo.GetCommit(c.Rev)
		if err != nil {
			return fmt.Errorf("GetCommit: %s Error: %w", c.Rev, err)
		}

		if c.Commit.Author != nil {
			email := c.Commit.Author.Email
			if c.User, ok = emails[email]; !ok {
				c.User, _ = user_model.GetUserByEmail(ctx, email)
				emails[email] = c.User
			}
		}

		c.Verification = asymkey_model.ParseCommitWithSignature(ctx, c.Commit)

		_ = asymkey_model.CalculateTrustStatus(c.Verification, repository.GetTrustModel(), func(user *user_model.User) (bool, error) {
			return repo_model.IsOwnerMemberCollaborator(ctx, repository, user.ID)
		}, &keyMap)

		statuses, _, err := git_model.GetLatestCommitStatus(ctx, repository.ID, c.Commit.ID.String(), db.ListOptions{})
		if err != nil {
			log.Error("GetLatestCommitStatus: %v", err)
		} else {
			c.Status = git_model.CalcCommitStatus(statuses)
		}
	}
	return nil
}

// NewFlow creates a new flow
func NewFlow(flowID int64, color, row, column int) *Flow {
	return &Flow{
		ID:          flowID,
		ColorNumber: color,
		MinRow:      row,
		MinColumn:   column,
		MaxRow:      row,
		MaxColumn:   column,
	}
}

// Flow represents a series of glyphs
type Flow struct {
	ID          int64
	ColorNumber int
	Glyphs      []Glyph
	Commits     []*Commit
	MinRow      int
	MinColumn   int
	MaxRow      int
	MaxColumn   int
}

// Color16 wraps the color numbers around mod 16
func (flow *Flow) Color16() int {
	return flow.ColorNumber % 16
}

// AddGlyph adds glyph at row and column
func (flow *Flow) AddGlyph(row, column int, glyph byte) {
	if row < flow.MinRow {
		flow.MinRow = row
	}
	if row > flow.MaxRow {
		flow.MaxRow = row
	}
	if column < flow.MinColumn {
		flow.MinColumn = column
	}
	if column > flow.MaxColumn {
		flow.MaxColumn = column
	}

	flow.Glyphs = append(flow.Glyphs, Glyph{
		row,
		column,
		glyph,
	})
}

// Glyph represents a co-ordinate and glyph
type Glyph struct {
	Row    int
	Column int
	Glyph  byte
}

// RelationCommit represents an empty relation commit
var RelationCommit = &Commit{
	Row: -1,
}

func parseGitTime(timeStr string) time.Time {
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return time.Unix(0, 0)
	}
	return t
}

// NewCommit creates a new commit from a provided line
func NewCommit(row, column int, line []byte) (*Commit, error) {
	data := bytes.SplitN(line, []byte("|"), 5)
	if len(data) < 5 {
		return nil, fmt.Errorf("malformed data section on line %d with commit: %s", row, string(line))
	}
	return &Commit{
		Row:    row,
		Column: column,
		// 0 matches git log --pretty=format:%d => ref names, like the --decorate option of git-log(1)
		Refs: newRefsFromRefNames(data[0]),
		// 1 matches git log --pretty=format:%H => commit hash
		Rev: string(data[1]),
		// 2 matches git log --pretty=format:%ad => author date (format respects --date= option)
		Date: parseGitTime(string(data[2])),
		// 3 matches git log --pretty=format:%h => abbreviated commit hash
		ShortRev: string(data[3]),
		// 4 matches git log --pretty=format:%s => subject
		Subject: string(data[4]),
	}, nil
}

func newRefsFromRefNames(refNames []byte) []git.Reference {
	refBytes := bytes.Split(refNames, []byte{',', ' '})
	refs := make([]git.Reference, 0, len(refBytes))
	for _, refNameBytes := range refBytes {
		if len(refNameBytes) == 0 {
			continue
		}
		refName := string(refNameBytes)
		if strings.HasPrefix(refName, "tag: ") {
			refName = strings.TrimPrefix(refName, "tag: ")
		} else {
			refName = strings.TrimPrefix(refName, "HEAD -> ")
		}
		refs = append(refs, git.Reference{
			Name: refName,
		})
	}
	return refs
}

// Commit represents a commit at co-ordinate X, Y with the data
type Commit struct {
	Commit       *git.Commit
	User         *user_model.User
	Verification *asymkey_model.CommitVerification
	Status       *git_model.CommitStatus
	Flow         int64
	Row          int
	Column       int
	Refs         []git.Reference
	Rev          string
	Date         time.Time
	ShortRev     string
	Subject      string
}

// OnlyRelation returns whether this a relation only commit
func (c *Commit) OnlyRelation() bool {
	return c.Row == -1
}
