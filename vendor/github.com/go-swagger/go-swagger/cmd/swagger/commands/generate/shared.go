package generate

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/swag"
	"github.com/go-swagger/go-swagger/generator"
	flags "github.com/jessevdk/go-flags"
	"github.com/spf13/viper"
)

// FlattenCmdOptions determines options to the flatten spec preprocessing
type FlattenCmdOptions struct {
	WithExpand  bool     `long:"with-expand" description:"expands all $ref's in spec prior to generation (shorthand to --with-flatten=expand)"`
	WithFlatten []string `long:"with-flatten" description:"flattens all $ref's in spec prior to generation" choice:"minimal" choice:"full" choice:"expand" choice:"verbose" choice:"noverbose" choice:"remove-unused" default:"minimal" default:"verbose"`
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
	//removeUnusedIsSet := false
	expandIsSet := false
	if f.WithExpand {
		res.Expand = true
		expandIsSet = true
	}
	for _, opt := range f.WithFlatten {
		if opt == "verbose" {
			res.Verbose = true
			verboseIsSet = true
		}
		if opt == "noverbose" && !verboseIsSet {
			// verbose flag takes precedence
			res.Verbose = false
			verboseIsSet = true
		}
		if opt == "remove-unused" {
			res.RemoveUnused = true
			//removeUnusedIsSet = true
		}
		if opt == "expand" {
			res.Expand = true
			expandIsSet = true
		}
		if opt == "full" && !minimalIsSet && !expandIsSet {
			// minimal flag takes precedence
			res.Minimal = false
			minimalIsSet = true
		}
		if opt == "minimal" && !expandIsSet {
			// expand flag takes precedence
			res.Minimal = true
			minimalIsSet = true
		}
	}
	return
}

type shared struct {
	Spec                  flags.Filename `long:"spec" short:"f" description:"the spec file to use (default swagger.{json,yml,yaml})"`
	APIPackage            string         `long:"api-package" short:"a" description:"the package to save the operations" default:"operations"`
	ModelPackage          string         `long:"model-package" short:"m" description:"the package to save the models" default:"models"`
	ServerPackage         string         `long:"server-package" short:"s" description:"the package to save the server specific code" default:"restapi"`
	ClientPackage         string         `long:"client-package" short:"c" description:"the package to save the client specific code" default:"client"`
	Target                flags.Filename `long:"target" short:"t" default:"./" description:"the base directory for generating the files"`
	Template              string         `long:"template" description:"Load contributed templates" choice:"stratoscale"`
	TemplateDir           flags.Filename `long:"template-dir" short:"T" description:"alternative template override directory"`
	ConfigFile            flags.Filename `long:"config-file" short:"C" description:"configuration file to use for overriding template options"`
	CopyrightFile         flags.Filename `long:"copyright-file" short:"r" description:"copyright file used to add copyright header"`
	ExistingModels        string         `long:"existing-models" description:"use pre-generated models e.g. github.com/foobar/model"`
	AdditionalInitialisms []string       `long:"additional-initialism" description:"consecutive capitals that should be considered intialisms"`
	AllowTemplateOverride bool           `long:"allow-template-override" description:"allows overriding protected templates"`
	FlattenCmdOptions
}

type sharedCommand interface {
	getOpts() (*generator.GenOpts, error)
	getShared() *shared
	getConfigFile() flags.Filename
	getAdditionalInitialisms() []string
	generate(*generator.GenOpts) error
	log(string)
}

func (s *shared) getConfigFile() flags.Filename {
	return s.ConfigFile
}

func (s *shared) getAdditionalInitialisms() []string {
	return s.AdditionalInitialisms
}

func (s *shared) setCopyright() (string, error) {
	var copyrightstr string
	copyrightfile := string(s.CopyrightFile)
	if copyrightfile != "" {
		//Read the Copyright from file path in opts
		bytebuffer, err := ioutil.ReadFile(copyrightfile)
		if err != nil {
			return "", err
		}
		copyrightstr = string(bytebuffer)
	} else {
		copyrightstr = ""
	}
	return copyrightstr, nil
}

func createSwagger(s sharedCommand) error {
	cfg, erc := readConfig(string(s.getConfigFile()))
	if erc != nil {
		return erc
	}
	setDebug(cfg)

	opts, ero := s.getOpts()
	if ero != nil {
		return ero
	}

	if opts.Template != "" {
		contribOptionsOverride(opts)
	}

	if err := opts.EnsureDefaults(); err != nil {
		return err
	}

	if err := configureOptsFromConfig(cfg, opts); err != nil {
		return err
	}

	swag.AddInitialisms(s.getAdditionalInitialisms()...)

	if sharedOpts := s.getShared(); sharedOpts != nil {
		// process shared options
		opts.FlattenOpts = sharedOpts.FlattenCmdOptions.SetFlattenOptions(opts.FlattenOpts)

		copyrightStr, erc := sharedOpts.setCopyright()
		if erc != nil {
			return erc
		}
		opts.Copyright = copyrightStr
	}

	if err := s.generate(opts); err != nil {
		return err
	}

	basepath, era := filepath.Abs(".")
	if era != nil {
		return era
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
	if os.Getenv("DEBUG") != "" || os.Getenv("SWAGGER_DEBUG") != "" {
		if cfg != nil {
			cfg.Debug()
		} else {
			log.Println("NO config read")
		}
	}
}
