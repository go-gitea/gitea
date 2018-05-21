//  Copyright (c) 2017 Couchbase, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 		http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package regexp

import (
	"regexp/syntax"
	"unicode"

	"github.com/couchbase/vellum/utf8"
)

type compiler struct {
	sizeLimit uint
	insts     prog
}

func newCompiler(sizeLimit uint) *compiler {
	return &compiler{
		sizeLimit: sizeLimit,
	}
}

func (c *compiler) compile(ast *syntax.Regexp) (prog, error) {
	err := c.c(ast)
	if err != nil {
		return nil, err
	}
	c.insts = append(c.insts, &inst{
		op: OpMatch,
	})
	return c.insts, nil
}

func (c *compiler) c(ast *syntax.Regexp) error {
	if ast.Flags&syntax.NonGreedy > 1 {
		return ErrNoLazy
	}

	switch ast.Op {
	case syntax.OpEndLine, syntax.OpBeginLine,
		syntax.OpBeginText, syntax.OpEndText:
		return ErrNoEmpty
	case syntax.OpWordBoundary, syntax.OpNoWordBoundary:
		return ErrNoWordBoundary
	case syntax.OpEmptyMatch:
		return nil
	case syntax.OpLiteral:
		for _, r := range ast.Rune {
			if ast.Flags&syntax.FoldCase > 0 {
				next := syntax.Regexp{
					Op:    syntax.OpCharClass,
					Flags: ast.Flags & syntax.FoldCase,
					Rune0: [2]rune{r, r},
				}
				next.Rune = next.Rune0[0:2]
				return c.c(&next)
			}
			seqs, err := utf8.NewSequences(r, r)
			if err != nil {
				return err
			}
			for _, seq := range seqs {
				c.compileUtf8Ranges(seq)
			}
		}
	case syntax.OpAnyChar:
		next := syntax.Regexp{
			Op:    syntax.OpCharClass,
			Flags: ast.Flags & syntax.FoldCase,
			Rune0: [2]rune{0, unicode.MaxRune},
		}
		next.Rune = next.Rune0[:2]
		return c.c(&next)
	case syntax.OpAnyCharNotNL:
		next := syntax.Regexp{
			Op:    syntax.OpCharClass,
			Flags: ast.Flags & syntax.FoldCase,
			Rune:  []rune{0, 0x09, 0x0B, unicode.MaxRune},
		}
		return c.c(&next)
	case syntax.OpCharClass:
		return c.compileClass(ast)
	case syntax.OpCapture:
		return c.c(ast.Sub[0])
	case syntax.OpConcat:
		for _, sub := range ast.Sub {
			err := c.c(sub)
			if err != nil {
				return err
			}
		}
		return nil
	case syntax.OpAlternate:
		if len(ast.Sub) == 0 {
			return nil
		}
		jmpsToEnd := []uint{}

		// does not handle last entry
		for i := 0; i < len(ast.Sub)-1; i++ {
			sub := ast.Sub[i]
			split := c.emptySplit()
			j1 := c.top()
			err := c.c(sub)
			if err != nil {
				return err
			}
			jmpsToEnd = append(jmpsToEnd, c.emptyJump())
			j2 := c.top()
			c.setSplit(split, j1, j2)
		}
		// handle last entry
		err := c.c(ast.Sub[len(ast.Sub)-1])
		if err != nil {
			return err
		}
		end := uint(len(c.insts))
		for _, jmpToEnd := range jmpsToEnd {
			c.setJump(jmpToEnd, end)
		}
	case syntax.OpQuest:
		split := c.emptySplit()
		j1 := c.top()
		err := c.c(ast.Sub[0])
		if err != nil {
			return err
		}
		j2 := c.top()
		c.setSplit(split, j1, j2)

	case syntax.OpStar:
		j1 := c.top()
		split := c.emptySplit()
		j2 := c.top()
		err := c.c(ast.Sub[0])
		if err != nil {
			return err
		}
		jmp := c.emptyJump()
		j3 := uint(len(c.insts))

		c.setJump(jmp, j1)
		c.setSplit(split, j2, j3)

	case syntax.OpPlus:
		j1 := c.top()
		err := c.c(ast.Sub[0])
		if err != nil {
			return err
		}
		split := c.emptySplit()
		j2 := c.top()
		c.setSplit(split, j1, j2)

	case syntax.OpRepeat:
		if ast.Max == -1 {
			for i := 0; i < ast.Min; i++ {
				err := c.c(ast.Sub[0])
				if err != nil {
					return err
				}
			}
			next := syntax.Regexp{
				Op:    syntax.OpStar,
				Flags: ast.Flags,
				Sub:   ast.Sub,
				Sub0:  ast.Sub0,
				Rune:  ast.Rune,
				Rune0: ast.Rune0,
			}
			return c.c(&next)
		}
		for i := 0; i < ast.Min; i++ {
			err := c.c(ast.Sub[0])
			if err != nil {
				return err
			}
		}
		var splits, starts []uint
		for i := ast.Min; i < ast.Max; i++ {
			splits = append(splits, c.emptySplit())
			starts = append(starts, uint(len(c.insts)))
			err := c.c(ast.Sub[0])
			if err != nil {
				return err
			}
		}
		end := uint(len(c.insts))
		for i := 0; i < len(splits); i++ {
			c.setSplit(splits[i], starts[i], end)
		}

	}

	return c.checkSize()
}

func (c *compiler) checkSize() error {
	if uint(len(c.insts)*instSize) > c.sizeLimit {
		return ErrCompiledTooBig
	}
	return nil
}

func (c *compiler) compileClass(ast *syntax.Regexp) error {
	if len(ast.Rune) == 0 {
		return nil
	}
	var jmps []uint

	// does not do last pair
	for i := 0; i < len(ast.Rune)-2; i += 2 {
		rstart := ast.Rune[i]
		rend := ast.Rune[i+1]

		split := c.emptySplit()
		j1 := c.top()
		err := c.compileClassRange(rstart, rend)
		if err != nil {
			return err
		}
		jmps = append(jmps, c.emptyJump())
		j2 := c.top()
		c.setSplit(split, j1, j2)
	}
	// handle last pair
	rstart := ast.Rune[len(ast.Rune)-2]
	rend := ast.Rune[len(ast.Rune)-1]
	err := c.compileClassRange(rstart, rend)
	if err != nil {
		return err
	}
	end := c.top()
	for _, jmp := range jmps {
		c.setJump(jmp, end)
	}
	return nil
}

func (c *compiler) compileClassRange(startR, endR rune) error {
	seqs, err := utf8.NewSequences(startR, endR)
	if err != nil {
		return err
	}
	var jmps []uint

	// does not do last entry
	for i := 0; i < len(seqs)-1; i++ {
		seq := seqs[i]
		split := c.emptySplit()
		j1 := c.top()
		c.compileUtf8Ranges(seq)
		jmps = append(jmps, c.emptyJump())
		j2 := c.top()
		c.setSplit(split, j1, j2)
	}
	// handle last entry
	c.compileUtf8Ranges(seqs[len(seqs)-1])
	end := c.top()
	for _, jmp := range jmps {
		c.setJump(jmp, end)
	}

	return nil
}

func (c *compiler) compileUtf8Ranges(seq utf8.Sequence) {
	for _, r := range seq {
		c.insts = append(c.insts, &inst{
			op:         OpRange,
			rangeStart: r.Start,
			rangeEnd:   r.End,
		})
	}
}

func (c *compiler) emptySplit() uint {
	c.insts = append(c.insts, &inst{
		op: OpSplit,
	})
	return c.top() - 1
}

func (c *compiler) emptyJump() uint {
	c.insts = append(c.insts, &inst{
		op: OpJmp,
	})
	return c.top() - 1
}

func (c *compiler) setSplit(i, pc1, pc2 uint) {
	split := c.insts[i]
	split.splitA = pc1
	split.splitB = pc2
}

func (c *compiler) setJump(i, pc uint) {
	jmp := c.insts[i]
	jmp.to = pc
}

func (c *compiler) top() uint {
	return uint(len(c.insts))
}
