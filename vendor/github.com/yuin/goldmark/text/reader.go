package text

import (
	"io"
	"regexp"
	"unicode/utf8"

	"github.com/yuin/goldmark/util"
)

const invalidValue = -1

// EOF indicates the end of file.
const EOF = byte(0xff)

// A Reader interface provides abstracted method for reading text.
type Reader interface {
	io.RuneReader

	// Source returns a source of the reader.
	Source() []byte

	// ResetPosition resets positions.
	ResetPosition()

	// Peek returns a byte at current position without advancing the internal pointer.
	Peek() byte

	// PeekLine returns the current line without advancing the internal pointer.
	PeekLine() ([]byte, Segment)

	// PrecendingCharacter returns a character just before current internal pointer.
	PrecendingCharacter() rune

	// Value returns a value of the given segment.
	Value(Segment) []byte

	// LineOffset returns a distance from the line head to current position.
	LineOffset() int

	// Position returns current line number and position.
	Position() (int, Segment)

	// SetPosition sets current line number and position.
	SetPosition(int, Segment)

	// SetPadding sets padding to the reader.
	SetPadding(int)

	// Advance advances the internal pointer.
	Advance(int)

	// AdvanceAndSetPadding advances the internal pointer and add padding to the
	// reader.
	AdvanceAndSetPadding(int, int)

	// AdvanceLine advances the internal pointer to the next line head.
	AdvanceLine()

	// SkipSpaces skips space characters and returns a non-blank line.
	// If it reaches EOF, returns false.
	SkipSpaces() (Segment, int, bool)

	// SkipSpaces skips blank lines and returns a non-blank line.
	// If it reaches EOF, returns false.
	SkipBlankLines() (Segment, int, bool)

	// Match performs regular expression matching to current line.
	Match(reg *regexp.Regexp) bool

	// Match performs regular expression searching to current line.
	FindSubMatch(reg *regexp.Regexp) [][]byte
}

type reader struct {
	source       []byte
	sourceLength int
	line         int
	peekedLine   []byte
	pos          Segment
	head         int
	lineOffset   int
}

// NewReader return a new Reader that can read UTF-8 bytes .
func NewReader(source []byte) Reader {
	r := &reader{
		source:       source,
		sourceLength: len(source),
	}
	r.ResetPosition()
	return r
}

func (r *reader) ResetPosition() {
	r.line = -1
	r.head = 0
	r.lineOffset = -1
	r.AdvanceLine()
}

func (r *reader) Source() []byte {
	return r.source
}

func (r *reader) Value(seg Segment) []byte {
	return seg.Value(r.source)
}

func (r *reader) Peek() byte {
	if r.pos.Start >= 0 && r.pos.Start < r.sourceLength {
		if r.pos.Padding != 0 {
			return space[0]
		}
		return r.source[r.pos.Start]
	}
	return EOF
}

func (r *reader) PeekLine() ([]byte, Segment) {
	if r.pos.Start >= 0 && r.pos.Start < r.sourceLength {
		if r.peekedLine == nil {
			r.peekedLine = r.pos.Value(r.Source())
		}
		return r.peekedLine, r.pos
	}
	return nil, r.pos
}

// io.RuneReader interface
func (r *reader) ReadRune() (rune, int, error) {
	return readRuneReader(r)
}

func (r *reader) LineOffset() int {
	if r.lineOffset < 0 {
		v := 0
		for i := r.head; i < r.pos.Start; i++ {
			if r.source[i] == '\t' {
				v += util.TabWidth(v)
			} else {
				v++
			}
		}
		r.lineOffset = v - r.pos.Padding
	}
	return r.lineOffset
}

func (r *reader) PrecendingCharacter() rune {
	if r.pos.Start <= 0 {
		if r.pos.Padding != 0 {
			return rune(' ')
		}
		return rune('\n')
	}
	i := r.pos.Start - 1
	for ; i >= 0; i-- {
		if utf8.RuneStart(r.source[i]) {
			break
		}
	}
	rn, _ := utf8.DecodeRune(r.source[i:])
	return rn
}

func (r *reader) Advance(n int) {
	r.lineOffset = -1
	if n < len(r.peekedLine) && r.pos.Padding == 0 {
		r.pos.Start += n
		r.peekedLine = nil
		return
	}
	r.peekedLine = nil
	l := r.sourceLength
	for ; n > 0 && r.pos.Start < l; n-- {
		if r.pos.Padding != 0 {
			r.pos.Padding--
			continue
		}
		if r.source[r.pos.Start] == '\n' {
			r.AdvanceLine()
			continue
		}
		r.pos.Start++
	}
}

func (r *reader) AdvanceAndSetPadding(n, padding int) {
	r.Advance(n)
	if padding > r.pos.Padding {
		r.SetPadding(padding)
	}
}

func (r *reader) AdvanceLine() {
	r.lineOffset = -1
	r.peekedLine = nil
	r.pos.Start = r.pos.Stop
	r.head = r.pos.Start
	if r.pos.Start < 0 {
		return
	}
	r.pos.Stop = r.sourceLength
	for i := r.pos.Start; i < r.sourceLength; i++ {
		c := r.source[i]
		if c == '\n' {
			r.pos.Stop = i + 1
			break
		}
	}
	r.line++
	r.pos.Padding = 0
}

func (r *reader) Position() (int, Segment) {
	return r.line, r.pos
}

func (r *reader) SetPosition(line int, pos Segment) {
	r.lineOffset = -1
	r.line = line
	r.pos = pos
}

func (r *reader) SetPadding(v int) {
	r.pos.Padding = v
}

func (r *reader) SkipSpaces() (Segment, int, bool) {
	return skipSpacesReader(r)
}

func (r *reader) SkipBlankLines() (Segment, int, bool) {
	return skipBlankLinesReader(r)
}

func (r *reader) Match(reg *regexp.Regexp) bool {
	return matchReader(r, reg)
}

func (r *reader) FindSubMatch(reg *regexp.Regexp) [][]byte {
	return findSubMatchReader(r, reg)
}

// A BlockReader interface is a reader that is optimized for Blocks.
type BlockReader interface {
	Reader
	// Reset resets current state and sets new segments to the reader.
	Reset(segment *Segments)
}

type blockReader struct {
	source         []byte
	segments       *Segments
	segmentsLength int
	line           int
	pos            Segment
	head           int
	last           int
	lineOffset     int
}

// NewBlockReader returns a new BlockReader.
func NewBlockReader(source []byte, segments *Segments) BlockReader {
	r := &blockReader{
		source: source,
	}
	if segments != nil {
		r.Reset(segments)
	}
	return r
}

func (r *blockReader) ResetPosition() {
	r.line = -1
	r.head = 0
	r.last = 0
	r.lineOffset = -1
	r.pos.Start = -1
	r.pos.Stop = -1
	r.pos.Padding = 0
	if r.segmentsLength > 0 {
		last := r.segments.At(r.segmentsLength - 1)
		r.last = last.Stop
	}
	r.AdvanceLine()
}

func (r *blockReader) Reset(segments *Segments) {
	r.segments = segments
	r.segmentsLength = segments.Len()
	r.ResetPosition()
}

func (r *blockReader) Source() []byte {
	return r.source
}

func (r *blockReader) Value(seg Segment) []byte {
	line := r.segmentsLength - 1
	ret := make([]byte, 0, seg.Stop-seg.Start+1)
	for ; line >= 0; line-- {
		if seg.Start >= r.segments.At(line).Start {
			break
		}
	}
	i := seg.Start
	for ; line < r.segmentsLength; line++ {
		s := r.segments.At(line)
		if i < 0 {
			i = s.Start
		}
		ret = s.ConcatPadding(ret)
		for ; i < seg.Stop && i < s.Stop; i++ {
			ret = append(ret, r.source[i])
		}
		i = -1
		if s.Stop > seg.Stop {
			break
		}
	}
	return ret
}

// io.RuneReader interface
func (r *blockReader) ReadRune() (rune, int, error) {
	return readRuneReader(r)
}

func (r *blockReader) PrecendingCharacter() rune {
	if r.pos.Padding != 0 {
		return rune(' ')
	}
	if r.pos.Start <= 0 {
		return rune('\n')
	}
	l := len(r.source)
	i := r.pos.Start - 1
	for ; i < l && i >= 0; i-- {
		if utf8.RuneStart(r.source[i]) {
			break
		}
	}
	if i < 0 || i >= l {
		return rune('\n')
	}
	rn, _ := utf8.DecodeRune(r.source[i:])
	return rn
}

func (r *blockReader) LineOffset() int {
	if r.lineOffset < 0 {
		v := 0
		for i := r.head; i < r.pos.Start; i++ {
			if r.source[i] == '\t' {
				v += util.TabWidth(v)
			} else {
				v++
			}
		}
		r.lineOffset = v - r.pos.Padding
	}
	return r.lineOffset
}

func (r *blockReader) Peek() byte {
	if r.line < r.segmentsLength && r.pos.Start >= 0 && r.pos.Start < r.last {
		if r.pos.Padding != 0 {
			return space[0]
		}
		return r.source[r.pos.Start]
	}
	return EOF
}

func (r *blockReader) PeekLine() ([]byte, Segment) {
	if r.line < r.segmentsLength && r.pos.Start >= 0 && r.pos.Start < r.last {
		return r.pos.Value(r.source), r.pos
	}
	return nil, r.pos
}

func (r *blockReader) Advance(n int) {
	r.lineOffset = -1

	if n < r.pos.Stop-r.pos.Start && r.pos.Padding == 0 {
		r.pos.Start += n
		return
	}

	for ; n > 0; n-- {
		if r.pos.Padding != 0 {
			r.pos.Padding--
			continue
		}
		if r.pos.Start >= r.pos.Stop-1 && r.pos.Stop < r.last {
			r.AdvanceLine()
			continue
		}
		r.pos.Start++
	}
}

func (r *blockReader) AdvanceAndSetPadding(n, padding int) {
	r.Advance(n)
	if padding > r.pos.Padding {
		r.SetPadding(padding)
	}
}

func (r *blockReader) AdvanceLine() {
	r.SetPosition(r.line+1, NewSegment(invalidValue, invalidValue))
	r.head = r.pos.Start
}

func (r *blockReader) Position() (int, Segment) {
	return r.line, r.pos
}

func (r *blockReader) SetPosition(line int, pos Segment) {
	r.lineOffset = -1
	r.line = line
	if pos.Start == invalidValue {
		if r.line < r.segmentsLength {
			s := r.segments.At(line)
			r.head = s.Start
			r.pos = s
		}
	} else {
		r.pos = pos
		if r.line < r.segmentsLength {
			s := r.segments.At(line)
			r.head = s.Start
		}
	}
}

func (r *blockReader) SetPadding(v int) {
	r.lineOffset = -1
	r.pos.Padding = v
}

func (r *blockReader) SkipSpaces() (Segment, int, bool) {
	return skipSpacesReader(r)
}

func (r *blockReader) SkipBlankLines() (Segment, int, bool) {
	return skipBlankLinesReader(r)
}

func (r *blockReader) Match(reg *regexp.Regexp) bool {
	return matchReader(r, reg)
}

func (r *blockReader) FindSubMatch(reg *regexp.Regexp) [][]byte {
	return findSubMatchReader(r, reg)
}

func skipBlankLinesReader(r Reader) (Segment, int, bool) {
	lines := 0
	for {
		line, seg := r.PeekLine()
		if line == nil {
			return seg, lines, false
		}
		if util.IsBlank(line) {
			lines++
			r.AdvanceLine()
		} else {
			return seg, lines, true
		}
	}
}

func skipSpacesReader(r Reader) (Segment, int, bool) {
	chars := 0
	for {
		line, segment := r.PeekLine()
		if line == nil {
			return segment, chars, false
		}
		for i, c := range line {
			if util.IsSpace(c) {
				chars++
				r.Advance(1)
				continue
			}
			return segment.WithStart(segment.Start + i + 1), chars, true
		}
	}
}

func matchReader(r Reader, reg *regexp.Regexp) bool {
	oldline, oldseg := r.Position()
	match := reg.FindReaderSubmatchIndex(r)
	r.SetPosition(oldline, oldseg)
	if match == nil {
		return false
	}
	r.Advance(match[1] - match[0])
	return true
}

func findSubMatchReader(r Reader, reg *regexp.Regexp) [][]byte {
	oldline, oldseg := r.Position()
	match := reg.FindReaderSubmatchIndex(r)
	r.SetPosition(oldline, oldseg)
	if match == nil {
		return nil
	}
	runes := make([]rune, 0, match[1]-match[0])
	for i := 0; i < match[1]; {
		r, size, _ := readRuneReader(r)
		i += size
		runes = append(runes, r)
	}
	result := [][]byte{}
	for i := 0; i < len(match); i += 2 {
		result = append(result, []byte(string(runes[match[i]:match[i+1]])))
	}

	r.SetPosition(oldline, oldseg)
	r.Advance(match[1] - match[0])
	return result
}

func readRuneReader(r Reader) (rune, int, error) {
	line, _ := r.PeekLine()
	if line == nil {
		return 0, 0, io.EOF
	}
	rn, size := utf8.DecodeRune(line)
	if rn == utf8.RuneError {
		return 0, 0, io.EOF
	}
	r.Advance(size)
	return rn, size, nil
}
