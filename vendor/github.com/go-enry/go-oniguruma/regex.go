package rubex

/*
#cgo CFLAGS: -I/usr/local/include
#cgo LDFLAGS: -L/usr/local/lib -lonig
#include <stdlib.h>
#include <oniguruma.h>
#include "chelper.h"
*/
import "C"

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strconv"
	"sync"
	"unicode/utf8"
	"unsafe"
)

const numMatchStartSize = 4
const numReadBufferStartSize = 256

var mutex sync.Mutex

type NamedGroupInfo map[string]int

type Regexp struct {
	pattern   string
	regex     C.OnigRegex
	encoding  C.OnigEncoding
	errorInfo *C.OnigErrorInfo
	errorBuf  *C.char

	numCaptures    int32
	namedGroupInfo NamedGroupInfo
}

// NewRegexp creates and initializes a new Regexp with the given pattern and option.
func NewRegexp(pattern string, option int) (*Regexp, error) {
	return initRegexp(&Regexp{pattern: pattern, encoding: C.ONIG_ENCODING_UTF8}, option)
}

// NewRegexpASCII is equivalent to NewRegexp, but with the encoding restricted to ASCII.
func NewRegexpASCII(pattern string, option int) (*Regexp, error) {
	return initRegexp(&Regexp{pattern: pattern, encoding: C.ONIG_ENCODING_ASCII}, option)
}

func initRegexp(re *Regexp, option int) (*Regexp, error) {
	patternCharPtr := C.CString(re.pattern)
	defer C.free(unsafe.Pointer(patternCharPtr))

	mutex.Lock()
	defer mutex.Unlock()

	errorCode := C.NewOnigRegex(patternCharPtr, C.int(len(re.pattern)), C.int(option), &re.regex, &re.encoding, &re.errorInfo, &re.errorBuf)
	if errorCode != C.ONIG_NORMAL {
		return re, errors.New(C.GoString(re.errorBuf))
	}

	re.numCaptures = int32(C.onig_number_of_captures(re.regex)) + 1
	re.namedGroupInfo = re.getNamedGroupInfo()

	runtime.SetFinalizer(re, (*Regexp).Free)

	return re, nil
}

func Compile(str string) (*Regexp, error) {
	return NewRegexp(str, ONIG_OPTION_DEFAULT)
}

func MustCompile(str string) *Regexp {
	regexp, error := NewRegexp(str, ONIG_OPTION_DEFAULT)
	if error != nil {
		panic("regexp: compiling " + str + ": " + error.Error())
	}

	return regexp
}

func CompileWithOption(str string, option int) (*Regexp, error) {
	return NewRegexp(str, option)
}

func MustCompileWithOption(str string, option int) *Regexp {
	regexp, error := NewRegexp(str, option)
	if error != nil {
		panic("regexp: compiling " + str + ": " + error.Error())
	}

	return regexp
}

// MustCompileASCII is equivalent to MustCompile, but with the encoding restricted to ASCII.
func MustCompileASCII(str string) *Regexp {
	regexp, error := NewRegexpASCII(str, ONIG_OPTION_DEFAULT)
	if error != nil {
		panic("regexp: compiling " + str + ": " + error.Error())
	}

	return regexp
}

func (re *Regexp) Free() {
	mutex.Lock()
	if re.regex != nil {
		C.onig_free(re.regex)
		re.regex = nil
	}
	mutex.Unlock()
	if re.errorInfo != nil {
		C.free(unsafe.Pointer(re.errorInfo))
		re.errorInfo = nil
	}
	if re.errorBuf != nil {
		C.free(unsafe.Pointer(re.errorBuf))
		re.errorBuf = nil
	}
}

func (re *Regexp) getNamedGroupInfo() NamedGroupInfo {
	numNamedGroups := int(C.onig_number_of_names(re.regex))
	// when any named capture exists, there is no numbered capture even if
	// there are unnamed captures.
	if numNamedGroups == 0 {
		return nil
	}

	namedGroupInfo := make(map[string]int)

	//try to get the names
	bufferSize := len(re.pattern) * 2
	nameBuffer := make([]byte, bufferSize)
	groupNumbers := make([]int32, numNamedGroups)
	bufferPtr := unsafe.Pointer(&nameBuffer[0])
	numbersPtr := unsafe.Pointer(&groupNumbers[0])

	length := int(C.GetCaptureNames(re.regex, bufferPtr, (C.int)(bufferSize), (*C.int)(numbersPtr)))
	if length == 0 {
		panic(fmt.Errorf("could not get the capture group names from %q", re.String()))
	}

	namesAsBytes := bytes.Split(nameBuffer[:length], ([]byte)(";"))
	if len(namesAsBytes) != numNamedGroups {
		panic(fmt.Errorf(
			"the number of named groups (%d) does not match the number names found (%d)",
			numNamedGroups, len(namesAsBytes),
		))
	}

	for i, nameAsBytes := range namesAsBytes {
		name := string(nameAsBytes)
		namedGroupInfo[name] = int(groupNumbers[i])
	}

	return namedGroupInfo
}

func (re *Regexp) find(b []byte, n int, offset int) []int {
	match := make([]int, re.numCaptures*2)

	if n == 0 {
		b = []byte{0}
	}

	bytesPtr := unsafe.Pointer(&b[0])

	// captures contains two pairs of ints, start and end, so we need list
	// twice the size of the capture groups.
	captures := make([]C.int, re.numCaptures*2)
	capturesPtr := unsafe.Pointer(&captures[0])

	var numCaptures int32
	numCapturesPtr := unsafe.Pointer(&numCaptures)

	pos := int(C.SearchOnigRegex(
		bytesPtr, C.int(n), C.int(offset), C.int(ONIG_OPTION_DEFAULT),
		re.regex, re.errorInfo, (*C.char)(nil), (*C.int)(capturesPtr), (*C.int)(numCapturesPtr),
	))

	if pos < 0 {
		return nil
	}

	if numCaptures <= 0 {
		panic("cannot have 0 captures when processing a match")
	}

	if re.numCaptures != numCaptures {
		panic(fmt.Errorf("expected %d captures but got %d", re.numCaptures, numCaptures))
	}

	for i := range captures {
		match[i] = int(captures[i])
	}

	return match
}

func getCapture(b []byte, beg int, end int) []byte {
	if beg < 0 || end < 0 {
		return nil
	}

	return b[beg:end]
}

func (re *Regexp) match(b []byte, n int, offset int) bool {
	if n == 0 {
		b = []byte{0}
	}

	bytesPtr := unsafe.Pointer(&b[0])
	pos := int(C.SearchOnigRegex(
		bytesPtr, C.int(n), C.int(offset), C.int(ONIG_OPTION_DEFAULT),
		re.regex, re.errorInfo, nil, nil, nil,
	))

	return pos >= 0
}

func (re *Regexp) findAll(b []byte, n int) [][]int {
	if n < 0 {
		n = len(b)
	}

	capture := make([][]int, 0, numMatchStartSize)
	var offset int
	for offset <= n {
		match := re.find(b, n, offset)
		if match == nil {
			break
		}

		capture = append(capture, match)

		// move offset to the ending index of the current match and prepare to
		// find the next non-overlapping match.
		offset = match[1]

		// if match[0] == match[1], it means the current match does not advance
		// the search. we need to exit the loop to avoid getting stuck here.
		if match[0] == match[1] {
			if offset < n && offset >= 0 {
				//there are more bytes, so move offset by a word
				_, width := utf8.DecodeRune(b[offset:])
				offset += width
			} else {
				//search is over, exit loop
				break
			}
		}
	}

	return capture
}

func (re *Regexp) FindIndex(b []byte) []int {
	match := re.find(b, len(b), 0)
	if len(match) == 0 {
		return nil
	}

	return match[:2]
}

func (re *Regexp) Find(b []byte) []byte {
	loc := re.FindIndex(b)
	if loc == nil {
		return nil
	}

	return getCapture(b, loc[0], loc[1])
}

func (re *Regexp) FindString(s string) string {
	mb := re.Find([]byte(s))
	if mb == nil {
		return ""
	}

	return string(mb)
}

func (re *Regexp) FindStringIndex(s string) []int {
	return re.FindIndex([]byte(s))
}

func (re *Regexp) FindAllIndex(b []byte, n int) [][]int {
	matches := re.findAll(b, n)
	if len(matches) == 0 {
		return nil
	}

	return matches
}

func (re *Regexp) FindAll(b []byte, n int) [][]byte {
	matches := re.FindAllIndex(b, n)
	if matches == nil {
		return nil
	}

	matchBytes := make([][]byte, 0, len(matches))
	for _, match := range matches {
		matchBytes = append(matchBytes, getCapture(b, match[0], match[1]))
	}

	return matchBytes
}

func (re *Regexp) FindAllString(s string, n int) []string {
	b := []byte(s)
	matches := re.FindAllIndex(b, n)
	if matches == nil {
		return nil
	}

	matchStrings := make([]string, 0, len(matches))
	for _, match := range matches {
		m := getCapture(b, match[0], match[1])
		if m == nil {
			matchStrings = append(matchStrings, "")
		} else {
			matchStrings = append(matchStrings, string(m))
		}
	}

	return matchStrings

}

func (re *Regexp) FindAllStringIndex(s string, n int) [][]int {
	return re.FindAllIndex([]byte(s), n)
}

func (re *Regexp) FindSubmatchIndex(b []byte) []int {
	match := re.find(b, len(b), 0)
	if len(match) == 0 {
		return nil
	}

	return match
}

func (re *Regexp) FindSubmatch(b []byte) [][]byte {
	match := re.FindSubmatchIndex(b)
	if match == nil {
		return nil
	}

	length := len(match) / 2
	if length == 0 {
		return nil
	}

	results := make([][]byte, 0, length)
	for i := 0; i < length; i++ {
		results = append(results, getCapture(b, match[2*i], match[2*i+1]))
	}

	return results
}

func (re *Regexp) FindStringSubmatch(s string) []string {
	b := []byte(s)
	match := re.FindSubmatchIndex(b)
	if match == nil {
		return nil
	}

	length := len(match) / 2
	if length == 0 {
		return nil
	}

	results := make([]string, 0, length)
	for i := 0; i < length; i++ {
		cap := getCapture(b, match[2*i], match[2*i+1])
		if cap == nil {
			results = append(results, "")
		} else {
			results = append(results, string(cap))
		}
	}

	return results
}

func (re *Regexp) FindStringSubmatchIndex(s string) []int {
	return re.FindSubmatchIndex([]byte(s))
}

func (re *Regexp) FindAllSubmatchIndex(b []byte, n int) [][]int {
	matches := re.findAll(b, n)
	if len(matches) == 0 {
		return nil
	}

	return matches
}

func (re *Regexp) FindAllSubmatch(b []byte, n int) [][][]byte {
	matches := re.findAll(b, n)
	if len(matches) == 0 {
		return nil
	}

	allCapturedBytes := make([][][]byte, 0, len(matches))
	for _, match := range matches {
		length := len(match) / 2
		capturedBytes := make([][]byte, 0, length)
		for i := 0; i < length; i++ {
			capturedBytes = append(capturedBytes, getCapture(b, match[2*i], match[2*i+1]))
		}

		allCapturedBytes = append(allCapturedBytes, capturedBytes)
	}

	return allCapturedBytes
}

func (re *Regexp) FindAllStringSubmatch(s string, n int) [][]string {
	b := []byte(s)

	matches := re.findAll(b, n)
	if len(matches) == 0 {
		return nil
	}

	allCapturedStrings := make([][]string, 0, len(matches))
	for _, match := range matches {
		length := len(match) / 2
		capturedStrings := make([]string, 0, length)
		for i := 0; i < length; i++ {
			cap := getCapture(b, match[2*i], match[2*i+1])
			if cap == nil {
				capturedStrings = append(capturedStrings, "")
			} else {
				capturedStrings = append(capturedStrings, string(cap))
			}
		}

		allCapturedStrings = append(allCapturedStrings, capturedStrings)
	}

	return allCapturedStrings
}

func (re *Regexp) FindAllStringSubmatchIndex(s string, n int) [][]int {
	return re.FindAllSubmatchIndex([]byte(s), n)
}

func (re *Regexp) Match(b []byte) bool {
	return re.match(b, len(b), 0)
}

func (re *Regexp) MatchString(s string) bool {
	return re.Match([]byte(s))
}

func (re *Regexp) NumSubexp() int {
	return (int)(C.onig_number_of_captures(re.regex))
}

func fillCapturedValues(repl []byte, _ []byte, capturedBytes map[string][]byte) []byte {
	replLen := len(repl)
	newRepl := make([]byte, 0, replLen*3)
	groupName := make([]byte, 0, replLen)

	var inGroupNameMode, inEscapeMode bool
	for index := 0; index < replLen; index++ {
		ch := repl[index]
		if inGroupNameMode && ch == byte('<') {
		} else if inGroupNameMode && ch == byte('>') {
			inGroupNameMode = false
			capBytes := capturedBytes[string(groupName)]
			newRepl = append(newRepl, capBytes...)
			groupName = groupName[:0] //reset the name
		} else if inGroupNameMode {
			groupName = append(groupName, ch)
		} else if inEscapeMode && ch <= byte('9') && byte('1') <= ch {
			capNumStr := string(ch)
			capBytes := capturedBytes[capNumStr]
			newRepl = append(newRepl, capBytes...)
		} else if inEscapeMode && ch == byte('k') && (index+1) < replLen && repl[index+1] == byte('<') {
			inGroupNameMode = true
			inEscapeMode = false
			index++ //bypass the next char '<'
		} else if inEscapeMode {
			newRepl = append(newRepl, '\\')
			newRepl = append(newRepl, ch)
		} else if ch != '\\' {
			newRepl = append(newRepl, ch)
		}
		if ch == byte('\\') || inEscapeMode {
			inEscapeMode = !inEscapeMode
		}
	}

	return newRepl
}

func (re *Regexp) replaceAll(src, repl []byte, replFunc func([]byte, []byte, map[string][]byte) []byte) []byte {
	srcLen := len(src)
	matches := re.findAll(src, srcLen)
	if len(matches) == 0 {
		return src
	}

	dest := make([]byte, 0, srcLen)
	for i, match := range matches {
		length := len(match) / 2
		capturedBytes := make(map[string][]byte)

		if re.namedGroupInfo == nil {
			for j := 0; j < length; j++ {
				capturedBytes[strconv.Itoa(j)] = getCapture(src, match[2*j], match[2*j+1])
			}
		} else {
			for name, j := range re.namedGroupInfo {
				capturedBytes[name] = getCapture(src, match[2*j], match[2*j+1])
			}
		}

		matchBytes := getCapture(src, match[0], match[1])
		newRepl := replFunc(repl, matchBytes, capturedBytes)
		prevEnd := 0
		if i > 0 {
			prevMatch := matches[i-1][:2]
			prevEnd = prevMatch[1]
		}

		if match[0] > prevEnd && prevEnd >= 0 && match[0] <= srcLen {
			dest = append(dest, src[prevEnd:match[0]]...)
		}

		dest = append(dest, newRepl...)
	}

	lastEnd := matches[len(matches)-1][1]
	if lastEnd < srcLen && lastEnd >= 0 {
		dest = append(dest, src[lastEnd:]...)
	}

	return dest
}

func (re *Regexp) ReplaceAll(src, repl []byte) []byte {
	return re.replaceAll(src, repl, fillCapturedValues)
}

func (re *Regexp) ReplaceAllFunc(src []byte, repl func([]byte) []byte) []byte {
	return re.replaceAll(src, nil, func(_ []byte, matchBytes []byte, _ map[string][]byte) []byte {
		return repl(matchBytes)
	})
}

func (re *Regexp) ReplaceAllString(src, repl string) string {
	return string(re.ReplaceAll([]byte(src), []byte(repl)))
}

func (re *Regexp) ReplaceAllStringFunc(src string, repl func(string) string) string {
	return string(re.replaceAll([]byte(src), nil, func(_ []byte, matchBytes []byte, _ map[string][]byte) []byte {
		return []byte(repl(string(matchBytes)))
	}))
}

func (re *Regexp) String() string {
	return re.pattern
}

func growBuffer(b []byte, offset int, n int) []byte {
	if offset+n > cap(b) {
		buf := make([]byte, 2*cap(b)+n)
		copy(buf, b[:offset])
		return buf
	}

	return b
}

func fromReader(r io.RuneReader) []byte {
	b := make([]byte, numReadBufferStartSize)

	var offset int
	for {
		rune, runeWidth, err := r.ReadRune()
		if err != nil {
			break
		}

		b = growBuffer(b, offset, runeWidth)
		writeWidth := utf8.EncodeRune(b[offset:], rune)
		if runeWidth != writeWidth {
			panic("reading rune width not equal to the written rune width")
		}

		offset += writeWidth
	}

	return b[:offset]
}

func (re *Regexp) FindReaderIndex(r io.RuneReader) []int {
	b := fromReader(r)
	return re.FindIndex(b)
}

func (re *Regexp) FindReaderSubmatchIndex(r io.RuneReader) []int {
	b := fromReader(r)
	return re.FindSubmatchIndex(b)
}

func (re *Regexp) MatchReader(r io.RuneReader) bool {
	b := fromReader(r)
	return re.Match(b)
}

func (re *Regexp) LiteralPrefix() (prefix string, complete bool) {
	//no easy way to implement this
	return "", false
}

func MatchString(pattern string, s string) (matched bool, error error) {
	re, err := Compile(pattern)
	if err != nil {
		return false, err
	}

	return re.MatchString(s), nil
}

func (re *Regexp) Gsub(src, repl string) string {
	return string(re.replaceAll([]byte(src), []byte(repl), fillCapturedValues))
}

func (re *Regexp) GsubFunc(src string, replFunc func(string, map[string]string) string) string {
	replaced := re.replaceAll([]byte(src), nil,
		func(_ []byte, matchBytes []byte, capturedBytes map[string][]byte) []byte {
			capturedStrings := make(map[string]string)
			for name, capBytes := range capturedBytes {
				capturedStrings[name] = string(capBytes)
			}
			matchString := string(matchBytes)
			return ([]byte)(replFunc(matchString, capturedStrings))
		},
	)

	return string(replaced)
}
