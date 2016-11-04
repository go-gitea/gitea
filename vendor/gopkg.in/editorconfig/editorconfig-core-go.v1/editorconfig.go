// Package editorconfig can be used to parse and generate editorconfig files.
// For more information about editorconfig, see http://editorconfig.org/
package editorconfig

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/ini.v1"
)

// IndentStyle possible values
const (
	IndentStyleTab    = "tab"
	IndentStyleSpaces = "space"
)

// EndOfLine possible values
const (
	EndOfLineLf   = "lf"
	EndOfLineCr   = "cr"
	EndOfLineCrLf = "crlf"
)

// Charset possible values
const (
	CharsetLatin1  = "latin1"
	CharsetUTF8    = "utf-8"
	CharsetUTF16BE = "utf-16be"
	CharsetUTF16LE = "utf-16le"
)

// Definition represents a definition inside the .editorconfig file.
// E.g. a section of the file.
// The definition is composed of the selector ("*", "*.go", "*.{js.css}", etc),
// plus the properties of the selected files.
type Definition struct {
	Selector string `ini:"-" json:"-"`

	Charset                string `ini:"charset" json:"charset,omitempty"`
	IndentStyle            string `ini:"indent_style" json:"indent_style,omitempty"`
	IndentSize             string `ini:"indent_size" json:"indent_size,omitempty"`
	TabWidth               int    `ini:"tab_width" json:"tab_width,omitempty"`
	EndOfLine              string `ini:"end_of_line" json:"end_of_line,omitempty"`
	TrimTrailingWhitespace bool   `ini:"trim_trailing_whitespace" json:"trim_trailing_whitespace,omitempty"`
	InsertFinalNewline     bool   `ini:"insert_final_newline" json:"insert_final_newline,omitempty"`
}

// Editorconfig represents a .editorconfig file.
// It is composed by a "root" property, plus the definitions defined in the
// file.
type Editorconfig struct {
	Root        bool
	Definitions []*Definition
}

// ParseBytes parses from a slice of bytes.
func ParseBytes(data []byte) (*Editorconfig, error) {
	iniFile, err := ini.Load(data)
	if err != nil {
		return nil, err
	}

	editorConfig := &Editorconfig{}
	editorConfig.Root = iniFile.Section(ini.DEFAULT_SECTION).Key("root").MustBool(false)
	for _, sectionStr := range iniFile.SectionStrings() {
		if sectionStr == ini.DEFAULT_SECTION {
			continue
		}
		var (
			iniSection = iniFile.Section(sectionStr)
			definition = &Definition{}
		)
		err := iniSection.MapTo(&definition)
		if err != nil {
			return nil, err
		}

		// tab_width defaults to indent_size:
		// https://github.com/editorconfig/editorconfig/wiki/EditorConfig-Properties#tab_width
		if definition.TabWidth <= 0 {
			if num, err := strconv.Atoi(definition.IndentSize); err == nil {
				definition.TabWidth = num
			}
		}

		definition.Selector = sectionStr
		editorConfig.Definitions = append(editorConfig.Definitions, definition)
	}
	return editorConfig, nil
}

// ParseFile parses from a file.
func ParseFile(f string) (*Editorconfig, error) {
	data, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, err
	}

	return ParseBytes(data)
}

var (
	regexpBraces = regexp.MustCompile("{.*}")
)

func filenameMatches(pattern, name string) bool {
	// basic match
	matched, _ := filepath.Match(pattern, name)
	if matched {
		return true
	}
	// foo/bar/main.go should match main.go
	matched, _ = filepath.Match(pattern, filepath.Base(name))
	if matched {
		return true
	}
	// foo should match foo/main.go
	matched, _ = filepath.Match(filepath.Join(pattern, "*"), name)
	if matched {
		return true
	}
	// *.{js,go} should match main.go
	if str := regexpBraces.FindString(pattern); len(str) > 0 {
		// remote initial "{" and final "}"
		str = strings.TrimPrefix(str, "{")
		str = strings.TrimSuffix(str, "}")

		// testing for empty brackets: "{}"
		if len(str) == 0 {
			patt := regexpBraces.ReplaceAllString(pattern, "*")
			matched, _ = filepath.Match(patt, filepath.Base(name))
			return matched
		}

		for _, patt := range strings.Split(str, ",") {
			patt = regexpBraces.ReplaceAllString(pattern, patt)
			matched, _ = filepath.Match(patt, filepath.Base(name))
			if matched {
				return true
			}
		}
	}
	return false
}

func (d *Definition) merge(md *Definition) {
	if len(d.Charset) == 0 {
		d.Charset = md.Charset
	}
	if len(d.IndentStyle) == 0 {
		d.IndentStyle = md.IndentStyle
	}
	if len(d.IndentSize) == 0 {
		d.IndentSize = md.IndentSize
	}
	if d.TabWidth <= 0 {
		d.TabWidth = md.TabWidth
	}
	if len(d.EndOfLine) == 0 {
		d.EndOfLine = md.EndOfLine
	}
	if !d.TrimTrailingWhitespace {
		d.TrimTrailingWhitespace = md.TrimTrailingWhitespace
	}
	if !d.InsertFinalNewline {
		d.InsertFinalNewline = md.InsertFinalNewline
	}
}

// GetDefinitionForFilename returns a definition for the given filename.
// The result is a merge of the selectors that matched the file.
// The last section has preference over the priors.
func (e *Editorconfig) GetDefinitionForFilename(name string) *Definition {
	def := &Definition{}
	for i := len(e.Definitions) - 1; i >= 0; i-- {
		actualDef := e.Definitions[i]
		if filenameMatches(actualDef.Selector, name) {
			def.merge(actualDef)
		}
	}
	return def
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// Serialize converts the Editorconfig to a slice of bytes, containing the
// content of the file in the INI format.
func (e *Editorconfig) Serialize() ([]byte, error) {
	var (
		iniFile = ini.Empty()
		buffer  = bytes.NewBuffer(nil)
	)
	iniFile.Section(ini.DEFAULT_SECTION).Comment = "http://editorconfig.org"
	if e.Root {
		iniFile.Section(ini.DEFAULT_SECTION).Key("root").SetValue(boolToString(e.Root))
	}
	for _, d := range e.Definitions {
		iniSec := iniFile.Section(d.Selector)
		if len(d.Charset) > 0 {
			iniSec.Key("charset").SetValue(d.Charset)
		}
		if len(d.IndentStyle) > 0 {
			iniSec.Key("indent_style").SetValue(d.IndentStyle)
		}
		if len(d.IndentSize) > 0 {
			iniSec.Key("indent_size").SetValue(d.IndentSize)
		}
		if d.TabWidth > 0 && strconv.Itoa(d.TabWidth) != d.IndentSize {
			iniSec.Key("tab_width").SetValue(strconv.Itoa(d.TabWidth))
		}
		if len(d.EndOfLine) > 0 {
			iniSec.Key("end_of_line").SetValue(d.EndOfLine)
		}
		if d.TrimTrailingWhitespace {
			iniSec.Key("trim_trailing_whitespace").SetValue(boolToString(d.TrimTrailingWhitespace))
		}
		if d.InsertFinalNewline {
			iniSec.Key("insert_final_newline").SetValue(boolToString(d.InsertFinalNewline))
		}
	}
	_, err := iniFile.WriteTo(buffer)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// Save saves the Editorconfig to a compatible INI file.
func (e *Editorconfig) Save(filename string) error {
	data, err := e.Serialize()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, data, 0666)
}

// GetDefinitionForFilename given a filename, searches
// for .editorconfig files, starting from the file folder,
// walking through the previous folders, until it reaches a
// folder with `root = true`, and returns the right editorconfig
// definition for the given file.
func GetDefinitionForFilename(filename string) (*Definition, error) {
	abs, err := filepath.Abs(filename)
	if err != nil {
		return nil, err
	}
	definition := &Definition{}

	dir := abs
	for dir != filepath.Dir(dir) {
		dir = filepath.Dir(dir)
		ecFile := filepath.Join(dir, ".editorconfig")
		if _, err := os.Stat(ecFile); os.IsNotExist(err) {
			continue
		}
		ec, err := ParseFile(ecFile)
		if err != nil {
			return nil, err
		}
		definition.merge(ec.GetDefinitionForFilename(filename))
		if ec.Root {
			break
		}
	}
	return definition, nil
}
