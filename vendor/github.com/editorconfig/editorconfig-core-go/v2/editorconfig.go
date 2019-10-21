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

const (
	// ConfigNameDefault represents the name of the configuration file
	ConfigNameDefault = ".editorconfig"
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
	CharsetUTF8BOM = "utf-8 bom"
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

	Raw map[string]string `ini:"-" json:"-"`
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
			raw        = make(map[string]string)
		)
		err := iniSection.MapTo(&definition)
		if err != nil {
			return nil, err
		}

		// Shallow copy all properties
		for k, v := range iniSection.KeysHash() {
			raw[strings.ToLower(k)] = v
		}

		definition.Selector = sectionStr
		definition.Raw = raw
		definition.normalize()
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

// normalize fixes some values to their lowercaes value
func (d *Definition) normalize() {
	d.Charset = strings.ToLower(d.Charset)
	d.EndOfLine = strings.ToLower(d.EndOfLine)
	d.IndentStyle = strings.ToLower(d.IndentStyle)

	// tab_width defaults to indent_size:
	// https://github.com/editorconfig/editorconfig/wiki/EditorConfig-Properties#tab_width
	num, err := strconv.Atoi(d.IndentSize)
	if err == nil && d.TabWidth <= 0 {
		d.TabWidth = num
	}
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

	for k, v := range md.Raw {
		if _, ok := d.Raw[k]; !ok {
			d.Raw[k] = v
		}
	}
}

// InsertToIniFile ... TODO
func (d *Definition) InsertToIniFile(iniFile *ini.File) {
	iniSec := iniFile.Section(d.Selector)
	for k, v := range d.Raw {
		if k == "insert_final_newline" {
			iniSec.Key(k).SetValue(strconv.FormatBool(d.InsertFinalNewline))
		} else if k == "trim_trailing_whitespace" {
			iniSec.Key(k).SetValue(strconv.FormatBool(d.TrimTrailingWhitespace))
		} else if k == "charset" {
			iniSec.Key(k).SetValue(d.Charset)
		} else if k == "end_of_line" {
			iniSec.Key(k).SetValue(d.EndOfLine)
		} else if k == "indent_style" {
			iniSec.Key(k).SetValue(d.IndentStyle)
		} else if k == "tab_width" {
			iniSec.Key(k).SetValue(strconv.Itoa(d.TabWidth))
		} else if k == "indent_size" {
			iniSec.Key(k).SetValue(d.IndentSize)
		} else {
			iniSec.Key(k).SetValue(v)
		}
	}
	if _, ok := d.Raw["indent_size"]; !ok {
		if d.TabWidth > 0 {
			iniSec.Key("indent_size").SetValue(strconv.Itoa(d.TabWidth))
		} else if d.IndentStyle == IndentStyleTab {
			iniSec.Key("indent_size").SetValue(IndentStyleTab)
		}
	}

	if _, ok := d.Raw["tab_width"]; !ok && len(d.IndentSize) > 0 {
		if _, err := strconv.Atoi(d.IndentSize); err == nil {
			iniSec.Key("tab_width").SetValue(d.IndentSize)
		}
	}
}

// GetDefinitionForFilename returns a definition for the given filename.
// The result is a merge of the selectors that matched the file.
// The last section has preference over the priors.
func (e *Editorconfig) GetDefinitionForFilename(name string) (*Definition, error) {
	def := &Definition{}
	def.Raw = make(map[string]string)
	for i := len(e.Definitions) - 1; i >= 0; i-- {
		actualDef := e.Definitions[i]
		selector := actualDef.Selector
		if !strings.HasPrefix(selector, "/") {
			if strings.ContainsRune(selector, '/') {
				selector = "/" + selector
			} else {
				selector = "/**/" + selector
			}
		}
		if !strings.HasPrefix(name, "/") {
			name = "/" + name
		}
		ok, err := FnmatchCase(selector, name)
		if err != nil {
			return nil, err
		}
		if ok {
			def.merge(actualDef)
		}
	}
	return def, nil
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
		d.InsertToIniFile(iniFile)
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
	return GetDefinitionForFilenameWithConfigname(filename, ConfigNameDefault)
}

func GetDefinitionForFilenameWithConfigname(filename string, configname string) (*Definition, error) {
	abs, err := filepath.Abs(filename)
	if err != nil {
		return nil, err
	}
	definition := &Definition{}
	definition.Raw = make(map[string]string)

	dir := abs
	for dir != filepath.Dir(dir) {
		dir = filepath.Dir(dir)
		ecFile := filepath.Join(dir, configname)
		if _, err := os.Stat(ecFile); os.IsNotExist(err) {
			continue
		}
		ec, err := ParseFile(ecFile)
		if err != nil {
			return nil, err
		}

		relativeFilename := filename
		if len(dir) < len(abs) {
			relativeFilename = abs[len(dir):]
		}

		def, err := ec.GetDefinitionForFilename(relativeFilename)
		if err != nil {
			return nil, err
		}

		definition.merge(def)

		if ec.Root {
			break
		}
	}

	return definition, nil
}
