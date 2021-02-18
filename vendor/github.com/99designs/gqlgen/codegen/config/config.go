package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/99designs/gqlgen/internal/code"
	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"gopkg.in/yaml.v2"
)

type Config struct {
	SchemaFilename           StringList                 `yaml:"schema,omitempty"`
	Exec                     PackageConfig              `yaml:"exec"`
	Model                    PackageConfig              `yaml:"model,omitempty"`
	Federation               PackageConfig              `yaml:"federation,omitempty"`
	Resolver                 ResolverConfig             `yaml:"resolver,omitempty"`
	AutoBind                 []string                   `yaml:"autobind"`
	Models                   TypeMap                    `yaml:"models,omitempty"`
	StructTag                string                     `yaml:"struct_tag,omitempty"`
	Directives               map[string]DirectiveConfig `yaml:"directives,omitempty"`
	OmitSliceElementPointers bool                       `yaml:"omit_slice_element_pointers,omitempty"`
	SkipValidation           bool                       `yaml:"skip_validation,omitempty"`
	Sources                  []*ast.Source              `yaml:"-"`
	Packages                 *code.Packages             `yaml:"-"`
	Schema                   *ast.Schema                `yaml:"-"`

	// Deprecated use Federation instead. Will be removed next release
	Federated bool `yaml:"federated,omitempty"`
}

var cfgFilenames = []string{".gqlgen.yml", "gqlgen.yml", "gqlgen.yaml"}

// DefaultConfig creates a copy of the default config
func DefaultConfig() *Config {
	return &Config{
		SchemaFilename: StringList{"schema.graphql"},
		Model:          PackageConfig{Filename: "models_gen.go"},
		Exec:           PackageConfig{Filename: "generated.go"},
		Directives:     map[string]DirectiveConfig{},
		Models:         TypeMap{},
	}
}

// LoadDefaultConfig loads the default config so that it is ready to be used
func LoadDefaultConfig() (*Config, error) {
	config := DefaultConfig()

	for _, filename := range config.SchemaFilename {
		filename = filepath.ToSlash(filename)
		var err error
		var schemaRaw []byte
		schemaRaw, err = ioutil.ReadFile(filename)
		if err != nil {
			return nil, errors.Wrap(err, "unable to open schema")
		}

		config.Sources = append(config.Sources, &ast.Source{Name: filename, Input: string(schemaRaw)})
	}

	return config, nil
}

// LoadConfigFromDefaultLocations looks for a config file in the current directory, and all parent directories
// walking up the tree. The closest config file will be returned.
func LoadConfigFromDefaultLocations() (*Config, error) {
	cfgFile, err := findCfg()
	if err != nil {
		return nil, err
	}

	err = os.Chdir(filepath.Dir(cfgFile))
	if err != nil {
		return nil, errors.Wrap(err, "unable to enter config dir")
	}
	return LoadConfig(cfgFile)
}

var path2regex = strings.NewReplacer(
	`.`, `\.`,
	`*`, `.+`,
	`\`, `[\\/]`,
	`/`, `[\\/]`,
)

// LoadConfig reads the gqlgen.yml config file
func LoadConfig(filename string) (*Config, error) {
	config := DefaultConfig()

	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "unable to read config")
	}

	if err := yaml.UnmarshalStrict(b, config); err != nil {
		return nil, errors.Wrap(err, "unable to parse config")
	}

	defaultDirectives := map[string]DirectiveConfig{
		"skip":       {SkipRuntime: true},
		"include":    {SkipRuntime: true},
		"deprecated": {SkipRuntime: true},
	}

	for key, value := range defaultDirectives {
		if _, defined := config.Directives[key]; !defined {
			config.Directives[key] = value
		}
	}

	preGlobbing := config.SchemaFilename
	config.SchemaFilename = StringList{}
	for _, f := range preGlobbing {
		var matches []string

		// for ** we want to override default globbing patterns and walk all
		// subdirectories to match schema files.
		if strings.Contains(f, "**") {
			pathParts := strings.SplitN(f, "**", 2)
			rest := strings.TrimPrefix(strings.TrimPrefix(pathParts[1], `\`), `/`)
			// turn the rest of the glob into a regex, anchored only at the end because ** allows
			// for any number of dirs in between and walk will let us match against the full path name
			globRe := regexp.MustCompile(path2regex.Replace(rest) + `$`)

			if err := filepath.Walk(pathParts[0], func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if globRe.MatchString(strings.TrimPrefix(path, pathParts[0])) {
					matches = append(matches, path)
				}

				return nil
			}); err != nil {
				return nil, errors.Wrapf(err, "failed to walk schema at root %s", pathParts[0])
			}
		} else {
			matches, err = filepath.Glob(f)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to glob schema filename %s", f)
			}
		}

		for _, m := range matches {
			if config.SchemaFilename.Has(m) {
				continue
			}
			config.SchemaFilename = append(config.SchemaFilename, m)
		}
	}

	for _, filename := range config.SchemaFilename {
		filename = filepath.ToSlash(filename)
		var err error
		var schemaRaw []byte
		schemaRaw, err = ioutil.ReadFile(filename)
		if err != nil {
			return nil, errors.Wrap(err, "unable to open schema")
		}

		config.Sources = append(config.Sources, &ast.Source{Name: filename, Input: string(schemaRaw)})
	}

	return config, nil
}

func (c *Config) Init() error {
	if c.Packages == nil {
		c.Packages = &code.Packages{}
	}

	if c.Schema == nil {
		if err := c.LoadSchema(); err != nil {
			return err
		}
	}

	err := c.injectTypesFromSchema()
	if err != nil {
		return err
	}

	err = c.autobind()
	if err != nil {
		return err
	}

	c.injectBuiltins()

	// prefetch all packages in one big packages.Load call
	pkgs := []string{
		"github.com/99designs/gqlgen/graphql",
		"github.com/99designs/gqlgen/graphql/introspection",
	}
	pkgs = append(pkgs, c.Models.ReferencedPackages()...)
	pkgs = append(pkgs, c.AutoBind...)
	c.Packages.LoadAll(pkgs...)

	//  check everything is valid on the way out
	err = c.check()
	if err != nil {
		return err
	}

	return nil
}

func (c *Config) injectTypesFromSchema() error {
	c.Directives["goModel"] = DirectiveConfig{
		SkipRuntime: true,
	}

	c.Directives["goField"] = DirectiveConfig{
		SkipRuntime: true,
	}

	for _, schemaType := range c.Schema.Types {
		if schemaType == c.Schema.Query || schemaType == c.Schema.Mutation || schemaType == c.Schema.Subscription {
			continue
		}

		if bd := schemaType.Directives.ForName("goModel"); bd != nil {
			if ma := bd.Arguments.ForName("model"); ma != nil {
				if mv, err := ma.Value.Value(nil); err == nil {
					c.Models.Add(schemaType.Name, mv.(string))
				}
			}
			if ma := bd.Arguments.ForName("models"); ma != nil {
				if mvs, err := ma.Value.Value(nil); err == nil {
					for _, mv := range mvs.([]interface{}) {
						c.Models.Add(schemaType.Name, mv.(string))
					}
				}
			}
		}

		if schemaType.Kind == ast.Object || schemaType.Kind == ast.InputObject {
			for _, field := range schemaType.Fields {
				if fd := field.Directives.ForName("goField"); fd != nil {
					forceResolver := c.Models[schemaType.Name].Fields[field.Name].Resolver
					fieldName := c.Models[schemaType.Name].Fields[field.Name].FieldName

					if ra := fd.Arguments.ForName("forceResolver"); ra != nil {
						if fr, err := ra.Value.Value(nil); err == nil {
							forceResolver = fr.(bool)
						}
					}

					if na := fd.Arguments.ForName("name"); na != nil {
						if fr, err := na.Value.Value(nil); err == nil {
							fieldName = fr.(string)
						}
					}

					if c.Models[schemaType.Name].Fields == nil {
						c.Models[schemaType.Name] = TypeMapEntry{
							Model:  c.Models[schemaType.Name].Model,
							Fields: map[string]TypeMapField{},
						}
					}

					c.Models[schemaType.Name].Fields[field.Name] = TypeMapField{
						FieldName: fieldName,
						Resolver:  forceResolver,
					}
				}
			}
		}
	}

	return nil
}

type TypeMapEntry struct {
	Model  StringList              `yaml:"model"`
	Fields map[string]TypeMapField `yaml:"fields,omitempty"`
}

type TypeMapField struct {
	Resolver        bool   `yaml:"resolver"`
	FieldName       string `yaml:"fieldName"`
	GeneratedMethod string `yaml:"-"`
}

type StringList []string

func (a *StringList) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var single string
	err := unmarshal(&single)
	if err == nil {
		*a = []string{single}
		return nil
	}

	var multi []string
	err = unmarshal(&multi)
	if err != nil {
		return err
	}

	*a = multi
	return nil
}

func (a StringList) Has(file string) bool {
	for _, existing := range a {
		if existing == file {
			return true
		}
	}
	return false
}

func (c *Config) check() error {
	if c.Models == nil {
		c.Models = TypeMap{}
	}

	type FilenamePackage struct {
		Filename string
		Package  string
		Declaree string
	}

	fileList := map[string][]FilenamePackage{}

	if err := c.Models.Check(); err != nil {
		return errors.Wrap(err, "config.models")
	}
	if err := c.Exec.Check(); err != nil {
		return errors.Wrap(err, "config.exec")
	}
	fileList[c.Exec.ImportPath()] = append(fileList[c.Exec.ImportPath()], FilenamePackage{
		Filename: c.Exec.Filename,
		Package:  c.Exec.Package,
		Declaree: "exec",
	})

	if c.Model.IsDefined() {
		if err := c.Model.Check(); err != nil {
			return errors.Wrap(err, "config.model")
		}
		fileList[c.Model.ImportPath()] = append(fileList[c.Model.ImportPath()], FilenamePackage{
			Filename: c.Model.Filename,
			Package:  c.Model.Package,
			Declaree: "model",
		})
	}
	if c.Resolver.IsDefined() {
		if err := c.Resolver.Check(); err != nil {
			return errors.Wrap(err, "config.resolver")
		}
		fileList[c.Resolver.ImportPath()] = append(fileList[c.Resolver.ImportPath()], FilenamePackage{
			Filename: c.Resolver.Filename,
			Package:  c.Resolver.Package,
			Declaree: "resolver",
		})
	}
	if c.Federation.IsDefined() {
		if err := c.Federation.Check(); err != nil {
			return errors.Wrap(err, "config.federation")
		}
		fileList[c.Federation.ImportPath()] = append(fileList[c.Federation.ImportPath()], FilenamePackage{
			Filename: c.Federation.Filename,
			Package:  c.Federation.Package,
			Declaree: "federation",
		})
		if c.Federation.ImportPath() != c.Exec.ImportPath() {
			return fmt.Errorf("federation and exec must be in the same package")
		}
	}
	if c.Federated {
		return fmt.Errorf("federated has been removed, instead use\nfederation:\n    filename: path/to/federated.go")
	}

	for importPath, pkg := range fileList {
		for _, file1 := range pkg {
			for _, file2 := range pkg {
				if file1.Package != file2.Package {
					return fmt.Errorf("%s and %s define the same import path (%s) with different package names (%s vs %s)",
						file1.Declaree,
						file2.Declaree,
						importPath,
						file1.Package,
						file2.Package,
					)
				}
			}
		}
	}

	return nil
}

type TypeMap map[string]TypeMapEntry

func (tm TypeMap) Exists(typeName string) bool {
	_, ok := tm[typeName]
	return ok
}

func (tm TypeMap) UserDefined(typeName string) bool {
	m, ok := tm[typeName]
	return ok && len(m.Model) > 0
}

func (tm TypeMap) Check() error {
	for typeName, entry := range tm {
		for _, model := range entry.Model {
			if strings.LastIndex(model, ".") < strings.LastIndex(model, "/") {
				return fmt.Errorf("model %s: invalid type specifier \"%s\" - you need to specify a struct to map to", typeName, entry.Model)
			}
		}
	}
	return nil
}

func (tm TypeMap) ReferencedPackages() []string {
	var pkgs []string

	for _, typ := range tm {
		for _, model := range typ.Model {
			if model == "map[string]interface{}" || model == "interface{}" {
				continue
			}
			pkg, _ := code.PkgAndType(model)
			if pkg == "" || inStrSlice(pkgs, pkg) {
				continue
			}
			pkgs = append(pkgs, code.QualifyPackagePath(pkg))
		}
	}

	sort.Slice(pkgs, func(i, j int) bool {
		return pkgs[i] > pkgs[j]
	})
	return pkgs
}

func (tm TypeMap) Add(name string, goType string) {
	modelCfg := tm[name]
	modelCfg.Model = append(modelCfg.Model, goType)
	tm[name] = modelCfg
}

type DirectiveConfig struct {
	SkipRuntime bool `yaml:"skip_runtime"`
}

func inStrSlice(haystack []string, needle string) bool {
	for _, v := range haystack {
		if needle == v {
			return true
		}
	}

	return false
}

// findCfg searches for the config file in this directory and all parents up the tree
// looking for the closest match
func findCfg() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", errors.Wrap(err, "unable to get working dir to findCfg")
	}

	cfg := findCfgInDir(dir)

	for cfg == "" && dir != filepath.Dir(dir) {
		dir = filepath.Dir(dir)
		cfg = findCfgInDir(dir)
	}

	if cfg == "" {
		return "", os.ErrNotExist
	}

	return cfg, nil
}

func findCfgInDir(dir string) string {
	for _, cfgName := range cfgFilenames {
		path := filepath.Join(dir, cfgName)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func (c *Config) autobind() error {
	if len(c.AutoBind) == 0 {
		return nil
	}

	ps := c.Packages.LoadAll(c.AutoBind...)

	for _, t := range c.Schema.Types {
		if c.Models.UserDefined(t.Name) {
			continue
		}

		for i, p := range ps {
			if p == nil {
				return fmt.Errorf("unable to load %s - make sure you're using an import path to a package that exists", c.AutoBind[i])
			}
			if t := p.Types.Scope().Lookup(t.Name); t != nil {
				c.Models.Add(t.Name(), t.Pkg().Path()+"."+t.Name())
				break
			}
		}
	}

	for i, t := range c.Models {
		for j, m := range t.Model {
			pkg, typename := code.PkgAndType(m)

			// skip anything that looks like an import path
			if strings.Contains(pkg, "/") {
				continue
			}

			for _, p := range ps {
				if p.Name != pkg {
					continue
				}
				if t := p.Types.Scope().Lookup(typename); t != nil {
					c.Models[i].Model[j] = t.Pkg().Path() + "." + t.Name()
					break
				}
			}
		}
	}

	return nil
}

func (c *Config) injectBuiltins() {
	builtins := TypeMap{
		"__Directive":         {Model: StringList{"github.com/99designs/gqlgen/graphql/introspection.Directive"}},
		"__DirectiveLocation": {Model: StringList{"github.com/99designs/gqlgen/graphql.String"}},
		"__Type":              {Model: StringList{"github.com/99designs/gqlgen/graphql/introspection.Type"}},
		"__TypeKind":          {Model: StringList{"github.com/99designs/gqlgen/graphql.String"}},
		"__Field":             {Model: StringList{"github.com/99designs/gqlgen/graphql/introspection.Field"}},
		"__EnumValue":         {Model: StringList{"github.com/99designs/gqlgen/graphql/introspection.EnumValue"}},
		"__InputValue":        {Model: StringList{"github.com/99designs/gqlgen/graphql/introspection.InputValue"}},
		"__Schema":            {Model: StringList{"github.com/99designs/gqlgen/graphql/introspection.Schema"}},
		"Float":               {Model: StringList{"github.com/99designs/gqlgen/graphql.Float"}},
		"String":              {Model: StringList{"github.com/99designs/gqlgen/graphql.String"}},
		"Boolean":             {Model: StringList{"github.com/99designs/gqlgen/graphql.Boolean"}},
		"Int": {Model: StringList{
			"github.com/99designs/gqlgen/graphql.Int",
			"github.com/99designs/gqlgen/graphql.Int32",
			"github.com/99designs/gqlgen/graphql.Int64",
		}},
		"ID": {
			Model: StringList{
				"github.com/99designs/gqlgen/graphql.ID",
				"github.com/99designs/gqlgen/graphql.IntID",
			},
		},
	}

	for typeName, entry := range builtins {
		if !c.Models.Exists(typeName) {
			c.Models[typeName] = entry
		}
	}

	// These are additional types that are injected if defined in the schema as scalars.
	extraBuiltins := TypeMap{
		"Time":   {Model: StringList{"github.com/99designs/gqlgen/graphql.Time"}},
		"Map":    {Model: StringList{"github.com/99designs/gqlgen/graphql.Map"}},
		"Upload": {Model: StringList{"github.com/99designs/gqlgen/graphql.Upload"}},
		"Any":    {Model: StringList{"github.com/99designs/gqlgen/graphql.Any"}},
	}

	for typeName, entry := range extraBuiltins {
		if t, ok := c.Schema.Types[typeName]; !c.Models.Exists(typeName) && ok && t.Kind == ast.Scalar {
			c.Models[typeName] = entry
		}
	}
}

func (c *Config) LoadSchema() error {
	if c.Packages != nil {
		c.Packages = &code.Packages{}
	}

	if err := c.check(); err != nil {
		return err
	}

	schema, err := gqlparser.LoadSchema(c.Sources...)
	if err != nil {
		return err
	}

	if schema.Query == nil {
		schema.Query = &ast.Definition{
			Kind: ast.Object,
			Name: "Query",
		}
		schema.Types["Query"] = schema.Query
	}

	c.Schema = schema
	return nil
}

func abs(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	return filepath.ToSlash(absPath)
}
