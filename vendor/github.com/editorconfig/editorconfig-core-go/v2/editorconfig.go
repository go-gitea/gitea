package editorconfig

import (
	"bytes"
	"io"
	"os"
	"strings"

	"gopkg.in/ini.v1"
)

const (
	// ConfigNameDefault represents the name of the configuration file
	ConfigNameDefault = ".editorconfig"
	// UnsetValue is the value that unsets a preexisting variable
	UnsetValue = "unset"
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

// Limits for section name, properties, and values.
const (
	MaxPropertyLength = 50
	MaxSectionLength  = 4096
	MaxValueLength    = 255
)

// Editorconfig represents a .editorconfig file.
//
// It is composed by a "root" property, plus the definitions defined in the
// file.
type Editorconfig struct {
	Root        bool
	Definitions []*Definition
	config      *Config
}

// newEditorconfig builds the configuration from an INI file.
func newEditorconfig(iniFile *ini.File) (*Editorconfig, error) {
	editorConfig := &Editorconfig{}

	// Consider mixed-case values for true and false.
	rootKey := iniFile.Section(ini.DefaultSection).Key("root")
	rootKey.SetValue(strings.ToLower(rootKey.Value()))
	editorConfig.Root = rootKey.MustBool(false)

	for _, sectionStr := range iniFile.SectionStrings() {
		if sectionStr == ini.DefaultSection || len(sectionStr) > MaxSectionLength {
			continue
		}

		iniSection := iniFile.Section(sectionStr)
		definition := &Definition{}
		raw := make(map[string]string)

		if err := iniSection.MapTo(&definition); err != nil {
			return nil, err
		}

		// Shallow copy all the properties
		for k, v := range iniSection.KeysHash() {
			if len(k) > MaxPropertyLength || len(v) > MaxValueLength {
				continue
			}

			raw[strings.ToLower(k)] = v
		}

		definition.Raw = raw
		definition.Selector = sectionStr

		if err := definition.normalize(); err != nil {
			return nil, err
		}

		editorConfig.Definitions = append(editorConfig.Definitions, definition)
	}

	return editorConfig, nil
}

// GetDefinitionForFilename returns a definition for the given filename.
//
// The result is a merge of the selectors that matched the file.
// The last section has preference over the priors.
func (e *Editorconfig) GetDefinitionForFilename(name string) (*Definition, error) {
	def := &Definition{
		Raw: make(map[string]string),
	}

	// The last section has preference over the priors.
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

		ok, err := e.FnmatchCase(selector, name)
		if err != nil {
			return nil, err
		}

		if ok {
			def.merge(actualDef)
		}
	}

	return def, nil
}

// FnmatchCase calls the matcher from the config's parser or the vanilla's.
func (e *Editorconfig) FnmatchCase(selector string, filename string) (bool, error) {
	if e.config != nil && e.config.Parser != nil {
		return e.config.Parser.FnmatchCase(selector, filename)
	}

	return FnmatchCase(selector, filename)
}

// Serialize converts the Editorconfig to a slice of bytes, containing the
// content of the file in the INI format.
func (e *Editorconfig) Serialize() ([]byte, error) {
	buffer := bytes.NewBuffer(nil)

	err := e.Write(buffer)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// Write writes the Editorconfig to the Writer in a compatible INI file.
func (e *Editorconfig) Write(w io.Writer) error {
	var (
		iniFile = ini.Empty()
	)

	iniFile.Section(ini.DefaultSection).Comment = "https://editorconfig.org"

	if e.Root {
		iniFile.Section(ini.DefaultSection).Key("root").SetValue(boolToString(e.Root))
	}

	for _, d := range e.Definitions {
		d.InsertToIniFile(iniFile)
	}

	_, err := iniFile.WriteTo(w)

	return err
}

// Save saves the Editorconfig to a compatible INI file.
func (e *Editorconfig) Save(filename string) error {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	return e.Write(f)
}

func boolToString(b bool) string {
	if b {
		return "true"
	}

	return "false"
}

// Parse parses from a reader.
func Parse(r io.Reader) (*Editorconfig, error) {
	iniFile, err := ini.Load(r)
	if err != nil {
		return nil, err
	}

	return newEditorconfig(iniFile)
}

// ParseBytes parses from a slice of bytes.
//
// Deprecated: use Parse instead.
func ParseBytes(data []byte) (*Editorconfig, error) {
	iniFile, err := ini.Load(data)
	if err != nil {
		return nil, err
	}

	return newEditorconfig(iniFile)
}

// ParseFile parses from a file.
//
// Deprecated: use Parse instead.
func ParseFile(path string) (*Editorconfig, error) {
	iniFile, err := ini.Load(path)
	if err != nil {
		return nil, err
	}

	return newEditorconfig(iniFile)
}

// GetDefinitionForFilename given a filename, searches
// for .editorconfig files, starting from the file folder,
// walking through the previous folders, until it reaches a
// folder with `root = true`, and returns the right editorconfig
// definition for the given file.
func GetDefinitionForFilename(filename string) (*Definition, error) {
	config := new(Config)
	return config.Load(filename)
}

// GetDefinitionForFilenameWithConfigname given a filename and a configname,
// searches for configname files, starting from the file folder,
// walking through the previous folders, until it reaches a
// folder with `root = true`, and returns the right editorconfig
// definition for the given file.
func GetDefinitionForFilenameWithConfigname(filename string, configname string) (*Definition, error) {
	config := &Config{
		Name: configname,
	}

	return config.Load(filename)
}
