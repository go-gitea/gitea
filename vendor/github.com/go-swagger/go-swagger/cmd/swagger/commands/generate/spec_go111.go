// +build go1.11

package generate

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/go-swagger/go-swagger/codescan"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/jessevdk/go-flags"
	"gopkg.in/yaml.v2"
)

// SpecFile command to generate a swagger spec from a go application
type SpecFile struct {
	WorkDir     string         `long:"work-dir" short:"w" description:"the base path to use" default:"."`
	BuildTags   string         `long:"tags" short:"t" description:"build tags" default:""`
	ScanModels  bool           `long:"scan-models" short:"m" description:"includes models that were annotated with 'swagger:model'"`
	Compact     bool           `long:"compact" description:"when present, doesn't prettify the json"`
	Output      flags.Filename `long:"output" short:"o" description:"the file to write to"`
	Input       flags.Filename `long:"input" short:"i" description:"the file to use as input"`
	Include     []string       `long:"include" short:"c" description:"include packages matching pattern"`
	Exclude     []string       `long:"exclude" short:"x" description:"exclude packages matching pattern"`
	IncludeTags []string       `long:"include-tag" short:"" description:"include routes having specified tags (can be specified many times)"`
	ExcludeTags []string       `long:"exclude-tag" short:"" description:"exclude routes having specified tags (can be specified many times)"`
	ExcludeDeps bool           `long:"exclude-deps" short:"" description:"exclude all dependencies of project"`
}

// Execute runs this command
func (s *SpecFile) Execute(args []string) error {
	if len(args) == 0 { // by default consider all the paths under the working directory
		args = []string{"./..."}
	}

	input, err := loadSpec(string(s.Input))
	if err != nil {
		return err
	}

	var opts codescan.Options
	opts.Packages = args
	opts.WorkDir = s.WorkDir
	opts.InputSpec = input
	opts.ScanModels = s.ScanModels
	opts.BuildTags = s.BuildTags
	opts.Include = s.Include
	opts.Exclude = s.Exclude
	opts.IncludeTags = s.IncludeTags
	opts.ExcludeTags = s.ExcludeTags
	opts.ExcludeDeps = s.ExcludeDeps
	swspec, err := codescan.Run(&opts)
	if err != nil {
		return err
	}

	return writeToFile(swspec, !s.Compact, string(s.Output))
}

func loadSpec(input string) (*spec.Swagger, error) {
	if fi, err := os.Stat(input); err == nil {
		if fi.IsDir() {
			return nil, fmt.Errorf("expected %q to be a file not a directory", input)
		}
		sp, err := loads.Spec(input)
		if err != nil {
			return nil, err
		}
		return sp.Spec(), nil
	}
	return nil, nil
}

func writeToFile(swspec *spec.Swagger, pretty bool, output string) error {
	var b []byte
	var err error

	if strings.HasSuffix(output, "yml") || strings.HasSuffix(output, "yaml") {
		b, err = marshalToYAMLFormat(swspec)
	} else {
		b, err = marshalToJSONFormat(swspec, pretty)
	}

	if err != nil {
		return err
	}

	if output == "" {
		fmt.Println(string(b))
		return nil
	}
	return ioutil.WriteFile(output, b, 0644)
}

func marshalToJSONFormat(swspec *spec.Swagger, pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(swspec, "", "  ")
	}
	return json.Marshal(swspec)
}

func marshalToYAMLFormat(swspec *spec.Swagger) ([]byte, error) {
	b, err := json.Marshal(swspec)
	if err != nil {
		return nil, err
	}

	var jsonObj interface{}
	if err := yaml.Unmarshal(b, &jsonObj); err != nil {
		return nil, err
	}

	return yaml.Marshal(jsonObj)
}
