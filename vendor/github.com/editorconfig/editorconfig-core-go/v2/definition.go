package editorconfig

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
	"gopkg.in/ini.v1"
)

// Definition represents a definition inside the .editorconfig file.
// E.g. a section of the file.
// The definition is composed of the selector ("*", "*.go", "*.{js.css}", etc),
// plus the properties of the selected files.
type Definition struct {
	Selector string `ini:"-" json:"-"`

	Charset                string            `ini:"charset" json:"charset,omitempty"`
	IndentStyle            string            `ini:"indent_style" json:"indent_style,omitempty"`
	IndentSize             string            `ini:"indent_size" json:"indent_size,omitempty"`
	TabWidth               int               `ini:"-" json:"-"`
	EndOfLine              string            `ini:"end_of_line" json:"end_of_line,omitempty"`
	TrimTrailingWhitespace *bool             `ini:"-" json:"-"`
	InsertFinalNewline     *bool             `ini:"-" json:"-"`
	Raw                    map[string]string `ini:"-" json:"-"`
	version                string
}

// NewDefinition builds a definition from a given config.
func NewDefinition(config Config) (*Definition, error) {
	return config.Load(config.Path)
}

// normalize fixes some values to their lowercase value.
func (d *Definition) normalize() error {
	d.Charset = strings.ToLower(d.Charset)
	d.EndOfLine = strings.ToLower(d.Raw["end_of_line"])
	d.IndentStyle = strings.ToLower(d.Raw["indent_style"])

	trimTrailingWhitespace, ok := d.Raw["trim_trailing_whitespace"]
	if ok && trimTrailingWhitespace != UnsetValue {
		trim, err := strconv.ParseBool(trimTrailingWhitespace)
		if err != nil {
			return fmt.Errorf("trim_trailing_whitespace=%s is not an acceptable value. %w", trimTrailingWhitespace, err)
		}

		d.TrimTrailingWhitespace = &trim
	}

	insertFinalNewline, ok := d.Raw["insert_final_newline"]
	if ok && insertFinalNewline != UnsetValue {
		insert, err := strconv.ParseBool(insertFinalNewline)
		if err != nil {
			return fmt.Errorf("insert_final_newline=%s is not an acceptable value. %w", insertFinalNewline, err)
		}

		d.InsertFinalNewline = &insert
	}

	// tab_width from Raw
	tabWidth, ok := d.Raw["tab_width"]
	if ok && tabWidth != UnsetValue {
		num, err := strconv.Atoi(tabWidth)
		if err != nil {
			return fmt.Errorf("tab_width=%s is not an acceptable value. %w", tabWidth, err)
		}

		d.TabWidth = num
	}

	// tab_width defaults to indent_size:
	// https://github.com/editorconfig/editorconfig/wiki/EditorConfig-Properties#tab_width
	num, err := strconv.Atoi(d.IndentSize)
	if err == nil && d.TabWidth <= 0 {
		d.TabWidth = num
	}

	return nil
}

// merge the parent definition into the child definition.
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

	if trimTrailingWhitespace, ok := d.Raw["trim_trailing_whitespace"]; !ok || trimTrailingWhitespace != UnsetValue {
		if d.TrimTrailingWhitespace == nil {
			d.TrimTrailingWhitespace = md.TrimTrailingWhitespace
		}
	}

	if insertFinalNewline, ok := d.Raw["insert_final_newline"]; !ok || insertFinalNewline != UnsetValue {
		if d.InsertFinalNewline == nil {
			d.InsertFinalNewline = md.InsertFinalNewline
		}
	}

	for k, v := range md.Raw {
		if _, ok := d.Raw[k]; !ok {
			d.Raw[k] = v
		}
	}
}

// InsertToIniFile writes the definition into a ini file.
func (d *Definition) InsertToIniFile(iniFile *ini.File) { // nolint: funlen,gocognit
	iniSec := iniFile.Section(d.Selector)

	for k, v := range d.Raw {
		switch k {
		case "insert_final_newline":
			if d.InsertFinalNewline != nil {
				v = strconv.FormatBool(*d.InsertFinalNewline)
			} else {
				insertFinalNewline, ok := d.Raw["insert_final_newline"]
				if !ok {
					break
				}

				v = strings.ToLower(insertFinalNewline)
			}
		case "trim_trailing_whitespace":
			if d.TrimTrailingWhitespace != nil {
				v = strconv.FormatBool(*d.TrimTrailingWhitespace)
			} else {
				trimTrailingWhitespace, ok := d.Raw["trim_trailing_whitespace"]
				if !ok {
					break
				}

				v = strings.ToLower(trimTrailingWhitespace)
			}
		case "charset":
			v = d.Charset
		case "end_of_line":
			v = d.EndOfLine
		case "indent_style":
			v = d.IndentStyle
		case "tab_width":
			tabWidth, ok := d.Raw["tab_width"]
			if ok && tabWidth == UnsetValue {
				v = tabWidth
			} else {
				v = strconv.Itoa(d.TabWidth)
			}
		case "indent_size":
			v = d.IndentSize
		}

		iniSec.NewKey(k, v) // nolint: errcheck
	}

	if _, ok := d.Raw["indent_size"]; !ok {
		tabWidth, ok := d.Raw["tab_width"]

		switch {
		case ok && tabWidth == UnsetValue:
			// do nothing
		case d.TabWidth > 0:
			iniSec.NewKey("indent_size", strconv.Itoa(d.TabWidth)) // nolint: errcheck
		case d.IndentStyle == IndentStyleTab && (d.version == "" || semver.Compare(d.version, "v0.9.0") >= 0):
			iniSec.NewKey("indent_size", IndentStyleTab) // nolint: errcheck
		}
	}

	if _, ok := d.Raw["tab_width"]; !ok {
		if d.IndentSize == UnsetValue {
			iniSec.NewKey("tab_width", d.IndentSize) // nolint: errcheck
		} else {
			_, err := strconv.Atoi(d.IndentSize)
			if err == nil {
				iniSec.NewKey("tab_width", d.Raw["indent_size"]) // nolint: errcheck
			}
		}
	}
}
