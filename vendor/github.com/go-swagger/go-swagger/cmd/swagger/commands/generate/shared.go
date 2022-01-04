package generate

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/swag"
	"github.com/go-swagger/go-swagger/generator"
	flags "github.com/jessevdk/go-flags"
	"github.com/spf13/viper"
)

// FlattenCmdOptions determines options to the flatten spec preprocessing
type FlattenCmdOptions struct {
	WithExpand  bool     `long:"with-expand" description:"expands all $ref's in spec prior to generation (shorthand to --with-flatten=expand)"  group:"shared"`
	WithFlatten []string `long:"with-flatten" description:"flattens all $ref's in spec prior to generation" choice:"minimal" choice:"full" choice:"expand" choice:"verbose" choice:"noverbose" choice:"remove-unused" default:"minimal" default:"verbose" group:"shared"` // nolint: staticcheck
}

// SetFlattenOptions builds flatten options from command line args
func (f *FlattenCmdOptions) SetFlattenOptions(dflt *analysis.FlattenOpts) (res *analysis.FlattenOpts) {
	res = &analysis.FlattenOpts{}
	if dflt != nil {
		*res = *dflt
	}
	if f == nil {
		return
	}
	verboseIsSet := false
	minimalIsSet := false
	expandIsSet := false
	if f.WithExpand {
		res.Expand = true
		expandIsSet = true
	}
	for _, opt := range f.WithFlatten {
		switch opt {
		case "verbose":
			res.Verbose = true
			verboseIsSet = true
		case "noverbose":
			if !verboseIsSet {
				// verbose flag takes precedence
				res.Verbose = false
				verboseIsSet = true
			}
		case "remove-unused":
			res.RemoveUnused = true
		case "expand":
			res.Expand = true
			expandIsSet = true
		case "full":
			if !minimalIsSet && !expandIsSet {
				// minimal flag takes precedence
				res.Minimal = false
				minimalIsSet = true
			}
		case "minimal":
			if !expandIsSet {
				// expand flag takes precedence
				res.Minimal = true
				minimalIsSet = true
			}
		}
	}
	return
}

type sharedCommand interface {
	apply(*generator.GenOpts)
	getConfigFile() string
	generate(*generator.GenOpts) error
	log(string)
}

type schemeOptions struct {
	Principal     string `short:"P" long:"principal" description:"the model to use for the security principal"`
	DefaultScheme string `long:"default-scheme" description:"the default scheme for this API" default:"http"`

	PrincipalIface bool `long:"principal-is-interface" description:"the security principal provided is an interface, not a struct"`
}

func (so schemeOptions) apply(opts *generator.GenOpts) {
	opts.Principal = so.Principal
	opts.PrincipalCustomIface = so.PrincipalIface
	opts.DefaultScheme = so.DefaultScheme
}

type mediaOptions struct {
	DefaultProduces string `long:"default-produces" description:"the default mime type that API operations produce" default:"application/json"`
	DefaultConsumes string `long:"default-consumes" description:"the default mime type that API operations consume" default:"application/json"`
}

func (m mediaOptions) apply(opts *generator.GenOpts) {
	opts.DefaultProduces = m.DefaultProduces
	opts.DefaultConsumes = m.DefaultConsumes

	const xmlIdentifier = "xml"
	opts.WithXML = strings.Contains(opts.DefaultProduces, xmlIdentifier) || strings.Contains(opts.DefaultConsumes, xmlIdentifier)
}

// WithShared adds the shared options group
type WithShared struct {
	Shared sharedOptions `group:"Options common to all code generation commands"`
}

func (w WithShared) getConfigFile() string {
	return string(w.Shared.ConfigFile)
}

type sharedOptions struct {
	Spec                  flags.Filename `long:"spec" short:"f" description:"the spec file to use (default swagger.{json,yml,yaml})" group:"shared"`
	Target                flags.Filename `long:"target" short:"t" default:"./" description:"the base directory for generating the files" group:"shared"`
	Template              string         `long:"template" description:"load contributed templates" choice:"stratoscale" group:"shared"`
	TemplateDir           flags.Filename `long:"template-dir" short:"T" description:"alternative template override directory" group:"shared"`
	ConfigFile            flags.Filename `long:"config-file" short:"C" description:"configuration file to use for overriding template options" group:"shared"`
	CopyrightFile         flags.Filename `long:"copyright-file" short:"r" description:"copyright file used to add copyright header" group:"shared"`
	AdditionalInitialisms []string       `long:"additional-initialism" description:"consecutive capitals that should be considered intialisms" group:"shared"`
	AllowTemplateOverride bool           `long:"allow-template-override" description:"allows overriding protected templates" group:"shared"`
	SkipValidation        bool           `long:"skip-validation" description:"skips validation of spec prior to generation" group:"shared"`
	DumpData              bool           `long:"dump-data" description:"when present dumps the json for the template generator instead of generating files" group:"shared"`
	StrictResponders      bool           `long:"strict-responders" description:"Use strict type for the handler return value"`
	FlattenCmdOptions
}

func (s sharedOptions) apply(opts *generator.GenOpts) {
	opts.Spec = string(s.Spec)
	opts.Target = string(s.Target)
	opts.Template = s.Template
	opts.TemplateDir = string(s.TemplateDir)
	opts.AllowTemplateOverride = s.AllowTemplateOverride
	opts.ValidateSpec = !s.SkipValidation
	opts.DumpData = s.DumpData
	opts.FlattenOpts = s.FlattenCmdOptions.SetFlattenOptions(opts.FlattenOpts)
	opts.Copyright = string(s.CopyrightFile)
	opts.StrictResponders = s.StrictResponders

	swag.AddInitialisms(s.AdditionalInitialisms...)
}

func setCopyright(copyrightFile string) (string, error) {
	// read the Copyright from file path in opts
	if copyrightFile == "" {
		return "", nil
	}
	bytebuffer, err := ioutil.ReadFile(copyrightFile)
	if err != nil {
		return "", err
	}
	return string(bytebuffer), nil
}

func createSwagger(s sharedCommand) error {
	cfg, err := readConfig(s.getConfigFile())
	if err != nil {
		return err
	}
	setDebug(cfg) // viper config Debug

	opts := new(generator.GenOpts)
	s.apply(opts)

	opts.Copyright, err = setCopyright(opts.Copyright)
	if err != nil {
		return fmt.Errorf("could not load copyright file: %v", err)
	}

	if opts.Template != "" {
		contribOptionsOverride(opts)
	}

	if err = opts.EnsureDefaults(); err != nil {
		return err
	}

	if err = configureOptsFromConfig(cfg, opts); err != nil {
		return err
	}

	if err = s.generate(opts); err != nil {
		return err
	}

	basepath, err := filepath.Abs(".")
	if err != nil {
		return err
	}

	targetAbs, err := filepath.Abs(opts.Target)
	if err != nil {
		return err
	}
	rp, err := filepath.Rel(basepath, targetAbs)
	if err != nil {
		return err
	}

	s.log(rp)

	return nil
}

func readConfig(filename string) (*viper.Viper, error) {
	if filename == "" {
		return nil, nil
	}

	abspath, err := filepath.Abs(filename)
	if err != nil {
		return nil, err
	}
	log.Println("trying to read config from", abspath)
	return generator.ReadConfig(abspath)
}

func configureOptsFromConfig(cfg *viper.Viper, opts *generator.GenOpts) error {
	if cfg == nil {
		return nil
	}

	var def generator.LanguageDefinition
	if err := cfg.Unmarshal(&def); err != nil {
		return err
	}
	return def.ConfigureOpts(opts)
}

func setDebug(cfg *viper.Viper) {
	// viper config debug
	if os.Getenv("DEBUG") != "" || os.Getenv("SWAGGER_DEBUG") != "" {
		if cfg != nil {
			cfg.Debug()
		} else {
			log.Println("No config read")
		}
	}
}
