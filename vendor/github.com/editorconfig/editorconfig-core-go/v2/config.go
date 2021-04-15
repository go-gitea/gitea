package editorconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/semver"
)

// ErrInvalidVersion represents a standard error with the semantic version.
var ErrInvalidVersion = errors.New("invalid semantic version")

// Config holds the configuration.
type Config struct {
	Path    string
	Name    string
	Version string
	Parser  Parser
}

// Load loads definition of a given file.
func (config *Config) Load(filename string) (*Definition, error) { // nolint: funlen
	// idiomatic go allows empty struct
	if config.Parser == nil {
		config.Parser = new(SimpleParser)
	}

	filename, err := filepath.Abs(filename)
	if err != nil {
		return nil, fmt.Errorf("cannot get absolute path for %q: %w", filename, err)
	}

	ecFile := config.Name
	if ecFile == "" {
		ecFile = ConfigNameDefault
	}

	definition := &Definition{}
	definition.Raw = make(map[string]string)

	if config.Version != "" {
		version := config.Version
		if !strings.HasPrefix(version, "v") {
			version = "v" + version
		}

		if ok := semver.IsValid(version); !ok {
			return nil, fmt.Errorf("version %s error: %w", config.Version, ErrInvalidVersion)
		}

		definition.version = version
	}

	dir := filename
	for dir != filepath.Dir(dir) {
		dir = filepath.Dir(dir)

		ec, err := config.Parser.ParseIni(filepath.Join(dir, ecFile))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}

			return nil, fmt.Errorf("cannot parse the ini file %q: %w", ecFile, err)
		}

		// give it the current config.
		ec.config = config

		relativeFilename := filename
		if len(dir) < len(relativeFilename) {
			relativeFilename = relativeFilename[len(dir):]
		}

		def, err := ec.GetDefinitionForFilename(relativeFilename)
		if err != nil {
			return nil, fmt.Errorf("cannot get definition for %q: %w", relativeFilename, err)
		}

		definition.merge(def)

		if ec.Root {
			break
		}
	}

	return definition, nil
}
