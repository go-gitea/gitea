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
	"log"
	"runtime"
	"strconv"
	"sync"
	"unicode/utf8"
	"unsafe"
)

type strRange []int

const numMatchStartSize = 4
const numReadBufferStartSize = 256

var mutex sync.Mutex

type MatchData struct {
	count   int
	indexes [][]int32
}

type NamedGroupInfo map[string]int

type Regexp struct {
	pattern        string
	regex          C.OnigRegex
	region         *C.OnigRegion
	encoding       C.OnigEncoding
	errorInfo      *C.OnigErrorInfo
	errorBuf       *C.char
	matchData      *MatchData
	namedGroupInfo NamedGroupInfo
}

// NewRegexp creates and initializes a new Regexp with the given pattern and option.
func NewRegexp(pattern string, option int) (re *Regexp, err error) {
	return initRegexp(&Regexp{pattern: pattern, encoding: C.ONIG_ENCODING_UTF8}, option)
}

// NewRegexpASCII is equivalent to NewRegexp, but with the encoding restricted to ASCII.
func NewRegexpASCII(pattern string, option int) (re *Regexp, err error) {
	return initRegexp(&Regexp{pattern: pattern, encoding: C.ONIG_ENCODING_ASCII}, option)
}

func initRegexp(re *Regexp, option int) (*Regexp, error) {
	var err error
	patternCharPtr := C.CString(re.pattern)
	defer C.free(unsafe.Pointer(patternCharPtr))
	mutex.Lock()
	defer mutex.Unlock()
	errorCode := C.NewOnigRegex(patternCharPtr, C.int(len(re.pattern)), C.int(option), &re.regex, &re.region, &re.encoding, &re.errorInfo, &re.errorBuf)
	if errorCode != C.ONIG_NORMAL {
		err = errors.New(C.GoString(re.errorBuf))
	} else {
		err = nil
		numCapturesInPattern := int(C.onig_number_of_captures(re.regex)) + 1
		re.matchData = &MatchData{}
		re.matchData.indexes = make([][]int32, numMatchStartSize)
		for i := 0; i < numMatchStartSize; i++ {
			re.matchData.indexes[i] = make([]int32, numCapturesInPattern*2)
		}
		re.namedGroupInfo = re.getNamedGroupInfo()
		runtime.SetFinalizer(re, (*Regexp).Free)
	}
	return re, err
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
	if re.region != nil {
		C.onig_region_free(re.region, 1)
		re.region = nil
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

func (re *Regexp) getNamedGroupInfo() (namedGroupInfo NamedGroupInfo) {
	numNamedGroups := int(C.onig_number_of_names(re.regex))
	//when any named capture exisits, there is no numbered capture even if there are unnamed captures
	if numNamedGroups > 0 {
		namedGroupInfo = make(map[string]int)
		//try to get the names
		bufferSize := len(re.pattern) * 2
		nameBuffer := make([]byte, bufferSize)
		groupNumbers := make([]int32, numNamedGroups)
		bufferPtr := unsafe.Pointer(&nameBuffer[0])
		numbersPtr := unsafe.Pointer(&groupNumbers[0])
		length := int(C.GetCaptureNames(re.regex, bufferPtr, (C.int)(bufferSize), (*C.int)(numbersPtr)))
		if length > 0 {
			namesAsBytes := bytes.Split(nameBuffer[:length], ([]byte)(";"))
			if len(namesAsBytes) != numNamedGroups {
				log.Fatalf("the number of named groups (%d) does not match the number names found (%d)\n", numNamedGroups, len(namesAsBytes))
			}
			for i, nameAsBytes := range namesAsBytes {
				name := string(nameAsBytes)
				namedGroupInfo[name] = int(groupNumbers[i])
			}
		} else {
			log.Fatalf("could not get the capture group names from %q", re.String())
		}
	}
	return
}

func (re *Regexp) groupNameToId(name string) (id int) {
	if re.namedGroupInfo == nil {
		id = ONIGERR_UNDEFINED_NAME_REFERENCE
	} else {
		id = re.namedGroupInfo[name]
	}
	return
}

func (re *Regexp) processMatch(numCaptures int) (match []int32) {
	if numCaptures <= 0 {
		panic("cannot have 0 captures when processing a match")
	}
	matchData := re.matchData
	return matchData.indexes[matchData.count][:numCaptures*2]
}

func (re *Regexp) ClearMatchData() {
	matchData := re.matchData
	matchData.count = 0
}

func (re *Regexp) find(b []byte, n int, offset int) (match []int) {
	if n == 0 {
		b = []byte{0}
	}
	ptr := unsafe.Pointer(&b[0])
	matchData := re.matchData
	capturesPtr := unsafe.Pointer(&(matchData.indexes[matchData.count][0]))
	numCaptures := int32(0)
	numCapturesPtr := unsafe.Pointer(&numCaptures)
	pos := int(C.SearchOnigRegex((ptr), C.int(n), C.int(offset), C.int(ONIG_OPTION_DEFAULT), re.regex, re.region, re.errorInfo, (*C.char)(nil), (*C.int)(capturesPtr), (*C.int)(numCapturesPtr)))
	if pos >= 0 {
		if numCaptures <= 0 {
			panic("cannot have 0 captures when processing a match")
		}
		match2 := matchData.indexes[matchData.count][:numCaptures*2]
		match = make([]int, len(match2))
		for i := range match2 {
			match[i] = int(match2[i])
		}
		numCapturesInPattern := int32(C.onig_number_of_captures(re.regex)) + 1
		if numCapturesInPattern != numCaptures {
			log.Fatalf("expected %d captures but got %d\n", numCapturesInPattern, numCaptures)
		}
	}
	return
}

func getCapture(b []byte, beg int, end int) []byte {
	if beg < 0 || end < 0 {
		return nil
	}
	return b[beg:end]
}

func (re *Regexp) match(b []byte, n int, offset int) bool {
	re.ClearMatchData()
	if n == 0 {
		b = []byte{0}
	}
	ptr := unsafe.Pointer(&b[0])
	pos := int(C.SearchOnigRegex((ptr), C.int(n), C.int(offset), C.int(ONIG_OPTION_DEFAULT), re.regex, re.region, re.errorInfo, (*C.char)(nil), (*C.int)(nil), (*C.int)(nil)))
	return pos >= 0
}

func (re *Regexp) findAll(b []byte, n int) (matches [][]int) {
	re.ClearMatchData()

	if n < 0 {
		n = len(b)
	}
	matchData := re.matchData
	offset := 0
	for offset <= n {
		if matchData.count >= len(matchData.indexes) {
			length := len(matchData.indexes[0])
			matchData.indexes = append(matchData.indexes, make([]int32, length))
		}
		if match := re.find(b, n, offset); len(match) > 0 {
			matchData.count += 1
			//move offset to the ending index of the current match and prepare to find the next non-overlapping match
			offset = match[1]
			//if match[0] == match[1], it means the current match does not advance the search. we need to exit the loop to avoid getting stuck here.
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
		} else {
			break
		}
	}
	matches2 := matchData.indexes[:matchData.count]
	matches = make([][]int, len(matches2))
	for i, v := range matches2 {
		matches[i] = make([]int, len(v))
		for j, v2 := range v {
			matches[i][j] = int(v2)
		}
	}
	return
}

func (re *Regexp) FindIndex(b []byte) []int {
	re.ClearMatchData()
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
	b := []byte(s)
	mb := re.Find(b)
	if mb == nil {
		return ""
	}
	return string(mb)
}

func (re *Regexp) FindStringIndex(s string) []int {
	b := []byte(s)
	return re.FindIndex(b)
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
	b := []byte(s)
	return re.FindAllIndex(b, n)
}

func (re *Regexp) findSubmatchIndex(b []byte) (match []int) {
	re.ClearMatchData()
	match = re.find(b, len(b), 0)
	return
}

func (re *Regexp) FindSubmatchIndex(b []byte) []int {
	match := re.findSubmatchIndex(b)
	if len(match) == 0 {
		return nil
	}
	return match
}

func (re *Regexp) FindSubmatch(b []byte) [][]byte {
	match := re.findSubmatchIndex(b)
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
	match := re.findSubmatchIndex(b)
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
	b := []byte(s)
	return re.FindSubmatchIndex(b)
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
	b := []byte(s)
	return re.FindAllSubmatchIndex(b, n)
}

func (re *Regexp) Match(b []byte) bool {
	return re.match(b, len(b), 0)
}

func (re *Regexp) MatchString(s string) bool {
	b := []byte(s)
	return re.Match(b)
}

func (re *Regexp) NumSubexp() int {
	return (int)(C.onig_number_of_captures(re.regex))
}

func (re *Regexp) getNamedCapture(name []byte, capturedBytes [][]byte) []byte {
	nameStr := string(name)
	capNum := re.groupNameToId(nameStr)
	if capNum < 0 || capNum >= len(capturedBytes) {
		panic(fmt.Sprintf("capture group name (%q) has error\n", nameStr))
	}
	return capturedBytes[capNum]
}

func (re *Regexp) getNumberedCapture(num int, capturedBytes [][]byte) []byte {
	//when named capture groups exist, numbered capture groups returns ""
	if re.namedGroupInfo == nil && num <= (len(capturedBytes)-1) && num >= 0 {
		return capturedBytes[num]
	}
	return ([]byte)("")
}

func fillCapturedValues(repl []byte, _ []byte, capturedBytes map[string][]byte) []byte {
	replLen := len(repl)
	newRepl := make([]byte, 0, replLen*3)
	inEscapeMode := false
	inGroupNameMode := false
	groupName := make([]byte, 0, replLen)
	for index := 0; index < replLen; index += 1 {
		ch := repl[index]
		if inGroupNameMode && ch == byte('<') {
		} else if inGroupNameMode && ch == byte('>') {
			inGroupNameMode = false
			groupNameStr := string(groupName)
			capBytes := capturedBytes[groupNameStr]
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
			index += 1 //bypass the next char '<'
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
	return re.replaceAll(src, []byte(""), func(_ []byte, matchBytes []byte, _ map[string][]byte) []byte {
		return repl(matchBytes)
	})
}

func (re *Regexp) ReplaceAllString(src, repl string) string {
	return string(re.ReplaceAll([]byte(src), []byte(repl)))
}

func (re *Regexp) ReplaceAllStringFunc(src string, repl func(string) string) string {
	srcB := []byte(src)
	destB := re.replaceAll(srcB, []byte(""), func(_ []byte, matchBytes []byte, _ map[string][]byte) []byte {
		return []byte(repl(string(matchBytes)))
	})
	return string(destB)
}

func (re *Regexp) String() string {
	return re.pattern
}

func grow_buffer(b []byte, offset int, n int) []byte {
	if offset+n > cap(b) {
		buf := make([]byte, 2*cap(b)+n)
		copy(buf, b[:offset])
		return buf
	}
	return b
}

func fromReader(r io.RuneReader) []byte {
	b := make([]byte, numReadBufferStartSize)
	offset := 0
	var err error = nil
	for err == nil {
		rune, runeWidth, err := r.ReadRune()
		if err == nil {
			b = grow_buffer(b, offset, runeWidth)
			writeWidth := utf8.EncodeRune(b[offset:], rune)
			if runeWidth != writeWidth {
				panic("reading rune width not equal to the written rune width")
			}
			offset += writeWidth
		} else {
			break
		}
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
	srcBytes := ([]byte)(src)
	replBytes := ([]byte)(repl)
	replaced := re.replaceAll(srcBytes, replBytes, fillCapturedValues)
	return string(replaced)
}

func (re *Regexp) GsubFunc(src string, replFunc func(string, map[string]string) string) string {
	srcBytes := ([]byte)(src)
	replaced := re.replaceAll(srcBytes, nil, func(_ []byte, matchBytes []byte, capturedBytes map[string][]byte) []byte {
		capturedStrings := make(map[string]string)
		for name, capBytes := range capturedBytes {
			capturedStrings[name] = string(capBytes)
		}
		matchString := string(matchBytes)
		return ([]byte)(replFunc(matchString, capturedStrings))
	})
	return string(replaced)
}
