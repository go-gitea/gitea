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

// Commit represents a commit at co-ordinate X, Y with the data
type Commit struct {
	Flow        int64
	Row         int
	Column      int
	Branch      string
	Rev         string
	Date        string
	Author      string
	AuthorEmail string
	ShortRev    string
	Subject     string
}

// CX returns the centre of the circle for this commit
func (c *Commit) CX() int {
	return c.Column*5 + 5
}

// CY returns the centre of the circle for this commit
func (c *Commit) CY() int {
	return c.Row*10 + 5
}

// OnlyRelation returns whether this a relation only commit
func (c *Commit) OnlyRelation() bool {
	return c.Row == -1
}

func newCommit(row, column, idx int, line []byte) (*Commit, error) {
	data := bytes.SplitN(line[idx+5:], []byte("|"), 7)
	if len(data) < 7 {
		return nil, fmt.Errorf("malformed data section on line %d with commit: %s", row, string(line))
	}
	return &Commit{
		Row:    row,
		Column: column,
		// 0 matches git log --pretty=format:%d => ref names, like the --decorate option of git-log(1)
		Branch: string(data[0]),
		// 1 matches git log --pretty=format:%H => commit hash
		Rev: string(data[1]),
		// 2 matches git log --pretty=format:%ad => author date (format respects --date= option)
		Date: string(data[2]),
		// 3 matches git log --pretty=format:%an => author name
		Author: string(data[3]),
		// 4 matches git log --pretty=format:%ae => author email
		AuthorEmail: string(data[4]),
		// 5 matches git log --pretty=format:%h => abbreviated commit hash
		ShortRev: string(data[5]),
		// 6 matches git log --pretty=format:%s => subject
		Subject: string(data[6]),
	}, nil
}

// Glyph represents a co-ordinate and glyph
type Glyph struct {
	Row    int
	Column int
	Glyph  byte
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
func (f *Flow) Color16() int {
	return f.ColorNumber % 16
}

type parserState struct {
	glyphs           []byte
	oldGlyphs        []byte
	flows            []int64
	oldFlows         []int64
	maxFlow          int64
	colors           []int
	oldColors        []int
	availableColors  []int
	nextAvailable    int
	firstInUse       int
	firstAvailable   int
	maxAllowedColors int
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
func (g *Graph) Width() int {
	return g.MaxColumn - g.MinColumn + 1
}

// Height returns the height of the graph
func (g *Graph) Height() int {
	return g.MaxRow - g.MinRow + 1
}

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

func (state *parserState) reset() {
	state.glyphs = state.glyphs[0:0]
	state.oldGlyphs = state.oldGlyphs[0:0]
	state.flows = state.flows[0:0]
	state.oldFlows = state.oldFlows[0:0]
	state.maxFlow = 0
	state.colors = state.colors[0:0]
	state.oldColors = state.oldColors[0:0]
	state.availableColors = state.availableColors[0:0]
	state.availableColors = append(state.availableColors, 1, 2)
	state.nextAvailable = 0
	state.firstInUse = -1
	state.firstAvailable = 0
	state.maxAllowedColors = 0
}

func (state *parserState) parseFlows(graph *Graph, row int, line []byte) error {
	idx := bytes.Index(line, []byte("DATA:"))
	if idx < 0 {
		state.parseGlyphs(line)
	} else {
		state.parseGlyphs(line[:idx])
	}

	var err error
	commitDone := false

	for column, glyph := range state.glyphs {
		flowID := state.flows[column]
		if glyph == ' ' {
			continue
		}

		flow, ok := graph.Flows[flowID]
		if !ok {
			flow = &Flow{
				ID:          flowID,
				ColorNumber: state.colors[column],
				MinRow:      row,
				MinColumn:   column,
				MaxRow:      row,
				MaxColumn:   column,
			}
			graph.Flows[flowID] = flow
		}
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

		flow.Glyphs = append(flow.Glyphs, Glyph{
			row,
			column,
			glyph,
		})

		if glyph == '*' {
			if commitDone {
				if err != nil {
					err = fmt.Errorf("double commit on line %d: %s. %w", row, string(line), err)
				} else {
					err = fmt.Errorf("double commit on line %d: %s", row, string(line))
				}
			}
			commitDone = true
			if idx < 0 {
				if err != nil {
					err = fmt.Errorf("missing data section on line %d with commit: %s. %w", row, string(line), err)
				} else {
					err = fmt.Errorf("missing data section on line %d with commit: %s", row, string(line))
				}
				continue
			}
			commit, err2 := newCommit(row, column, idx, line)
			if err != nil && err2 != nil {
				err = fmt.Errorf("%v %w", err2, err)
				continue
			} else if err2 != nil {
				err = err2
				continue
			}
			commit.Flow = flowID
			graph.Commits = append(graph.Commits, commit)
			flow.Commits = append(flow.Commits, commit)
		}
	}
	if !commitDone {
		graph.Commits = append(graph.Commits, graph.relationCommit)
	}
	return err
}

func (state *parserState) releaseUnusedColors() {
	if state.firstInUse > -1 {
		// Here we step through the old colors, searching for them in the
		// "in-use" section of availableColors (that is, the colors between
		// firstInUse and firstAvailable)
		// Ensure that the benchmarks are not worsened with proposed changes
		stepstaken := 0
		position := state.firstInUse
		for _, color := range state.oldColors {
			if color == 0 {
				continue
			}
			found := false
			i := position
			for j := stepstaken; i != state.firstAvailable && j < len(state.availableColors); j++ {
				colorToCheck := state.availableColors[i]
				if colorToCheck == color {
					found = true
					break
				}
				i = (i + 1) % len(state.availableColors)
			}
			if !found {
				// Duplicate color
				continue
			}
			// Swap them around
			state.availableColors[position], state.availableColors[i] = state.availableColors[i], state.availableColors[position]
			stepstaken++
			position = (state.firstInUse + stepstaken) % len(state.availableColors)
			if position == state.firstAvailable || stepstaken == len(state.availableColors) {
				break
			}
		}
		if stepstaken == len(state.availableColors) {
			state.firstAvailable = -1
		} else {
			state.firstAvailable = position
			if state.nextAvailable == -1 {
				state.nextAvailable = state.firstAvailable
			}
		}
	}
}

func (state *parserState) parseGlyphs(glyphs []byte) {

	// Clean state for parsing this row
	state.glyphs, state.oldGlyphs = state.oldGlyphs, state.glyphs
	state.glyphs = state.glyphs[0:0]
	state.flows, state.oldFlows = state.oldFlows, state.flows
	state.flows = state.flows[0:0]
	state.colors, state.oldColors = state.oldColors, state.colors

	// Ensure we have enough flows and colors
	state.colors = state.colors[0:0]
	for range glyphs {
		state.flows = append(state.flows, 0)
		state.colors = append(state.colors, 0)
	}

	// Copy the provided glyphs in to state.glyphs for safekeeping
	state.glyphs = append(state.glyphs, glyphs...)

	// release unused colors
	state.releaseUnusedColors()

	for i := len(glyphs) - 1; i >= 0; i-- {
		glyph := glyphs[i]
		switch glyph {
		case '|':
			fallthrough
		case '*':
			state.setUpFlow(i)
		case '/':
			state.setOutFlow(i)
		case '\\':
			state.setInFlow(i)
		case '_':
			state.setRightFlow(i)
		case '.':
			fallthrough
		case '-':
			state.setLeftFlow(i)
		case ' ':
			// no-op
		default:
			state.newFlow(i)
		}
	}
}

func (state *parserState) takePreviousFlow(i, j int) {
	if j < len(state.oldFlows) && state.oldFlows[j] > 0 {
		state.flows[i] = state.oldFlows[j]
		state.oldFlows[j] = 0
		state.colors[i] = state.oldColors[j]
		state.oldColors[j] = 0
	} else {
		state.newFlow(i)
	}
}

func (state *parserState) takeCurrentFlow(i, j int) {
	if j < len(state.flows) && state.flows[j] > 0 {
		state.flows[i] = state.flows[j]
		state.colors[i] = state.colors[j]
	} else {
		state.newFlow(i)
	}
}

func (state *parserState) newFlow(i int) {
	state.maxFlow++
	state.flows[i] = state.maxFlow

	// Now give this flow a color
	if state.nextAvailable == -1 {
		next := len(state.availableColors)
		if state.maxAllowedColors < 1 || next < state.maxAllowedColors {
			state.nextAvailable = next
			state.firstAvailable = next
			state.availableColors = append(state.availableColors, next+1)
		}
	}
	state.colors[i] = state.availableColors[state.nextAvailable]
	if state.firstInUse == -1 {
		state.firstInUse = state.nextAvailable
	}
	state.availableColors[state.firstAvailable], state.availableColors[state.nextAvailable] = state.availableColors[state.nextAvailable], state.availableColors[state.firstAvailable]

	state.nextAvailable = (state.nextAvailable + 1) % len(state.availableColors)
	state.firstAvailable = (state.firstAvailable + 1) % len(state.availableColors)

	if state.nextAvailable == state.firstInUse {
		state.nextAvailable = state.firstAvailable
	}
	if state.nextAvailable == state.firstInUse {
		state.nextAvailable = -1
		state.firstAvailable = -1
	}
}

// setUpFlow handles '|' or '*'
func (state *parserState) setUpFlow(i int) {
	// In preference order:
	//
	// Previous Row: '\? ' ' |' '  /'
	// Current Row:  ' | ' ' |' ' | '
	if i > 0 && i-1 < len(state.oldGlyphs) && state.oldGlyphs[i-1] == '\\' {
		state.takePreviousFlow(i, i-1)
	} else if i < len(state.oldGlyphs) && (state.oldGlyphs[i] == '|' || state.oldGlyphs[i] == '*') {
		state.takePreviousFlow(i, i)
	} else if i+1 < len(state.oldGlyphs) && state.oldGlyphs[i+1] == '/' {
		state.takePreviousFlow(i, i+1)
	} else {
		state.newFlow(i)
	}
}

// setOutFlow handles '/'
func (state *parserState) setOutFlow(i int) {
	// In preference order:
	//
	// Previous Row: ' |/' ' |_' ' |' ' /' ' _' '\'
	// Current Row:  '/| ' '/| ' '/ ' '/ ' '/ ' '/'
	if i+2 < len(state.oldGlyphs) &&
		(state.oldGlyphs[i+1] == '|' || state.oldGlyphs[i+1] == '*') &&
		(state.oldGlyphs[i+2] == '/' || state.oldGlyphs[i+2] == '_') &&
		i+1 < len(state.glyphs) &&
		(state.glyphs[i+1] == '|' || state.glyphs[i+1] == '*') {
		state.takePreviousFlow(i, i+2)
	} else if i+1 < len(state.oldGlyphs) &&
		(state.oldGlyphs[i+1] == '|' || state.oldGlyphs[i+1] == '*' ||
			state.oldGlyphs[i+1] == '/' || state.oldGlyphs[i+1] == '_') {
		state.takePreviousFlow(i, i+1)
		if state.oldGlyphs[i+1] == '/' {
			state.glyphs[i] = '|'
		}
	} else if i < len(state.oldGlyphs) && state.oldGlyphs[i] == '\\' {
		state.takePreviousFlow(i, i)
	} else {
		state.newFlow(i)
	}
}

// setInFlow handles '\'
func (state *parserState) setInFlow(i int) {
	// In preference order:
	//
	// Previous Row: '| ' '-. ' '| ' '\ ' '/' '---'
	// Current Row:  '|\' '  \' ' \' ' \' '\' ' \ '
	if i > 0 && i-1 < len(state.oldGlyphs) &&
		(state.oldGlyphs[i-1] == '|' || state.oldGlyphs[i-1] == '*') &&
		(state.glyphs[i-1] == '|' || state.glyphs[i-1] == '*') {
		state.newFlow(i)
	} else if i > 0 && i-1 < len(state.oldGlyphs) &&
		(state.oldGlyphs[i-1] == '|' || state.oldGlyphs[i-1] == '*' ||
			state.oldGlyphs[i-1] == '.' || state.oldGlyphs[i-1] == '\\') {
		state.takePreviousFlow(i, i-1)
		if state.oldGlyphs[i-1] == '\\' {
			state.glyphs[i] = '|'
		}
	} else if i < len(state.oldGlyphs) && state.oldGlyphs[i] == '/' {
		state.takePreviousFlow(i, i)
	} else {
		state.newFlow(i)
	}
}

// setRightFlow handles '_'
func (state *parserState) setRightFlow(i int) {
	// In preference order:
	//
	// Current Row:  '__' '_/' '_|_' '_|/'
	if i+1 < len(state.glyphs) &&
		(state.glyphs[i+1] == '_' || state.glyphs[i+1] == '/') {
		state.takeCurrentFlow(i, i+1)
	} else if i+2 < len(state.glyphs) &&
		(state.glyphs[i+1] == '|' || state.glyphs[i+1] == '*') &&
		(state.glyphs[i+2] == '_' || state.glyphs[i+2] == '/') {
		state.takeCurrentFlow(i, i+2)
	} else {
		state.newFlow(i)
	}
}

// setLeftFlow handles '----.'
func (state *parserState) setLeftFlow(i int) {
	if state.glyphs[i] == '.' {
		state.newFlow(i)
	} else if i+1 < len(state.glyphs) &&
		(state.glyphs[i+1] == '-' || state.glyphs[i+1] == '.') {
		state.takeCurrentFlow(i, i+1)
	} else {
		state.newFlow(i)
	}
}

// GetCommitGraph return a list of commit (GraphItems) from all branches
func GetCommitGraph(r *git.Repository, page int, maxAllowedColors int) (*Graph, error) {
	format := "DATA:%d|%H|%ad|%an|%ae|%h|%s"

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
	graph := NewGraph()

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
		parser := &parserState{}
		parser.firstInUse = -1
		parser.maxAllowedColors = maxAllowedColors
		if maxAllowedColors > 0 {
			parser.availableColors = make([]int, maxAllowedColors)
			for i := range parser.availableColors {
				parser.availableColors[i] = i + 1
			}
		} else {
			parser.availableColors = []int{1, 2}
		}
		for commitsToSkip > 0 && scanner.Scan() {
			line := scanner.Bytes()
			dataIdx := bytes.Index(line, []byte("DATA:"))
			if dataIdx < 0 {
				dataIdx = len(line)
			}
			starIdx := bytes.IndexByte(line, '*')
			if starIdx >= 0 && starIdx < dataIdx {
				commitsToSkip--
			}
			parser.parseGlyphs(line[:dataIdx])
		}

		row := 0

		// Skip initial non-commit lines
		for scanner.Scan() {
			line := scanner.Bytes()
			if bytes.IndexByte(line, '*') >= 0 {
				if err := parser.parseFlows(graph, row, line); err != nil {
					cancel()
					return err
				}
				break
			}
			parser.parseGlyphs(line)
		}

		for scanner.Scan() {
			row++
			line := scanner.Bytes()
			if err := parser.parseFlows(graph, row, line); err != nil {
				cancel()
				return err
			}
		}
		return scanner.Err()
	}); err != nil {
		return graph, err
	}
	return graph, nil
}
