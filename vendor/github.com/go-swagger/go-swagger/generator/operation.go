// Copyright 2015 go-swagger maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package generator

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/swag"
)

type respSort struct {
	Code     int
	Response spec.Response
}

type responses []respSort

func (s responses) Len() int           { return len(s) }
func (s responses) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s responses) Less(i, j int) bool { return s[i].Code < s[j].Code }

// sortedResponses produces a sorted list of responses.
// TODO: this is redundant with the definition given in struct.go
func sortedResponses(input map[int]spec.Response) responses {
	var res responses
	for k, v := range input {
		if k > 0 {
			res = append(res, respSort{k, v})
		}
	}
	sort.Sort(res)
	return res
}

// GenerateServerOperation generates a parameter model, parameter validator, http handler implementations for a given operation.
//
// It also generates an operation handler interface that uses the parameter model for handling a valid request.
// Allows for specifying a list of tags to include only certain tags for the generation
func GenerateServerOperation(operationNames []string, opts *GenOpts) error {
	if err := opts.CheckOpts(); err != nil {
		return err
	}

	if err := opts.setTemplates(); err != nil {
		return err
	}

	specDoc, analyzed, err := opts.analyzeSpec()
	if err != nil {
		return err
	}

	ops := gatherOperations(analyzed, operationNames)

	if len(ops) == 0 {
		return errors.New("no operations were selected")
	}

	for operationName, opRef := range ops {
		method, path, operation := opRef.Method, opRef.Path, opRef.Op

		serverPackage := opts.LanguageOpts.ManglePackagePath(opts.ServerPackage, defaultServerTarget)
		generator := operationGenerator{
			Name:                 operationName,
			Method:               method,
			Path:                 path,
			BasePath:             specDoc.BasePath(),
			APIPackage:           opts.LanguageOpts.ManglePackagePath(opts.APIPackage, defaultOperationsTarget),
			ModelsPackage:        opts.LanguageOpts.ManglePackagePath(opts.ModelPackage, defaultModelsTarget),
			ClientPackage:        opts.LanguageOpts.ManglePackagePath(opts.ClientPackage, defaultClientTarget),
			ServerPackage:        serverPackage,
			Operation:            *operation,
			SecurityRequirements: analyzed.SecurityRequirementsFor(operation),
			SecurityDefinitions:  analyzed.SecurityDefinitionsFor(operation),
			Principal:            opts.PrincipalAlias(),
			Target:               filepath.Join(opts.Target, filepath.FromSlash(serverPackage)),
			Base:                 opts.Target,
			Tags:                 opts.Tags,
			IncludeHandler:       opts.IncludeHandler,
			IncludeParameters:    opts.IncludeParameters,
			IncludeResponses:     opts.IncludeResponses,
			IncludeValidator:     opts.IncludeValidator,
			DumpData:             opts.DumpData,
			DefaultScheme:        opts.DefaultScheme,
			DefaultProduces:      opts.DefaultProduces,
			DefaultConsumes:      opts.DefaultConsumes,
			Doc:                  specDoc,
			Analyzed:             analyzed,
			GenOpts:              opts,
		}
		if err := generator.Generate(); err != nil {
			return err
		}
	}
	return nil
}

type operationGenerator struct {
	Authorized        bool
	IncludeHandler    bool
	IncludeParameters bool
	IncludeResponses  bool
	IncludeValidator  bool
	DumpData          bool

	Principal            string
	Target               string
	Base                 string
	Name                 string
	Method               string
	Path                 string
	BasePath             string
	APIPackage           string
	ModelsPackage        string
	ServerPackage        string
	ClientPackage        string
	Operation            spec.Operation
	SecurityRequirements [][]analysis.SecurityRequirement
	SecurityDefinitions  map[string]spec.SecurityScheme
	Tags                 []string
	DefaultScheme        string
	DefaultProduces      string
	DefaultConsumes      string
	Doc                  *loads.Document
	Analyzed             *analysis.Spec
	GenOpts              *GenOpts
}

// Generate a single operation
func (o *operationGenerator) Generate() error {

	defaultImports := o.GenOpts.defaultImports()

	apiPackage := o.GenOpts.LanguageOpts.ManglePackagePath(o.GenOpts.APIPackage, defaultOperationsTarget)
	imports := o.GenOpts.initImports(
		filepath.Join(o.GenOpts.LanguageOpts.ManglePackagePath(o.GenOpts.ServerPackage, defaultServerTarget), apiPackage))

	bldr := codeGenOpBuilder{
		ModelsPackage:       o.ModelsPackage,
		Principal:           o.GenOpts.PrincipalAlias(),
		Target:              o.Target,
		DefaultImports:      defaultImports,
		Imports:             imports,
		DefaultScheme:       o.DefaultScheme,
		Doc:                 o.Doc,
		Analyzed:            o.Analyzed,
		BasePath:            o.BasePath,
		GenOpts:             o.GenOpts,
		Name:                o.Name,
		Operation:           o.Operation,
		Method:              o.Method,
		Path:                o.Path,
		IncludeValidator:    o.IncludeValidator,
		APIPackage:          o.APIPackage, // defaults to main operations package
		DefaultProduces:     o.DefaultProduces,
		DefaultConsumes:     o.DefaultConsumes,
		Authed:              len(o.Analyzed.SecurityRequirementsFor(&o.Operation)) > 0,
		Security:            o.Analyzed.SecurityRequirementsFor(&o.Operation),
		SecurityDefinitions: o.Analyzed.SecurityDefinitionsFor(&o.Operation),
		RootAPIPackage:      o.GenOpts.LanguageOpts.ManglePackageName(o.ServerPackage, defaultServerTarget),
	}

	_, tags, _ := bldr.analyzeTags()

	op, err := bldr.MakeOperation()
	if err != nil {
		return err
	}

	op.Tags = tags
	operations := make(GenOperations, 0, 1)
	operations = append(operations, op)
	sort.Sort(operations)

	for _, pp := range operations {
		op := pp
		if o.GenOpts.DumpData {
			_ = dumpData(swag.ToDynamicJSON(op))
			continue
		}
		if err := o.GenOpts.renderOperation(&op); err != nil {
			return err
		}
	}

	return nil
}

type codeGenOpBuilder struct {
	Authed           bool
	IncludeValidator bool

	Name                string
	Method              string
	Path                string
	BasePath            string
	APIPackage          string
	APIPackageAlias     string
	RootAPIPackage      string
	ModelsPackage       string
	Principal           string
	Target              string
	Operation           spec.Operation
	Doc                 *loads.Document
	PristineDoc         *loads.Document
	Analyzed            *analysis.Spec
	DefaultImports      map[string]string
	Imports             map[string]string
	DefaultScheme       string
	DefaultProduces     string
	DefaultConsumes     string
	Security            [][]analysis.SecurityRequirement
	SecurityDefinitions map[string]spec.SecurityScheme
	ExtraSchemas        map[string]GenSchema
	GenOpts             *GenOpts
}

// paramMappings yields a map of safe parameter names for an operation
func paramMappings(params map[string]spec.Parameter) (map[string]map[string]string, string) {
	idMapping := map[string]map[string]string{
		"query":    make(map[string]string, len(params)),
		"path":     make(map[string]string, len(params)),
		"formData": make(map[string]string, len(params)),
		"header":   make(map[string]string, len(params)),
		"body":     make(map[string]string, len(params)),
	}

	// In order to avoid unstable generation, adopt same naming convention
	// for all parameters with same name across locations.
	seenIds := make(map[string]interface{}, len(params))
	for id, p := range params {
		if val, ok := seenIds[p.Name]; ok {
			previous := val.(struct{ id, in string })
			idMapping[p.In][p.Name] = swag.ToGoName(id)
			// rewrite the previously found one
			idMapping[previous.in][p.Name] = swag.ToGoName(previous.id)
		} else {
			idMapping[p.In][p.Name] = swag.ToGoName(p.Name)
		}
		seenIds[strings.ToLower(idMapping[p.In][p.Name])] = struct{ id, in string }{id: id, in: p.In}
	}

	// pick a deconflicted private name for timeout for this operation
	timeoutName := renameTimeout(seenIds, "timeout")

	return idMapping, timeoutName
}

// renameTimeout renames the variable in use by client template to avoid conflicting
// with param names.
//
// NOTE: this merely protects the timeout field in the client parameter struct,
// fields "Context" and "HTTPClient" remain exposed to name conflicts.
func renameTimeout(seenIds map[string]interface{}, timeoutName string) string {
	if seenIds == nil {
		return timeoutName
	}
	current := strings.ToLower(timeoutName)
	if _, ok := seenIds[current]; !ok {
		return timeoutName
	}
	var next string
	switch current {
	case "timeout":
		next = "requestTimeout"
	case "requesttimeout":
		next = "httpRequestTimeout"
	case "httprequesttimeout":
		next = "swaggerTimeout"
	case "swaggertimeout":
		next = "operationTimeout"
	case "operationtimeout":
		next = "opTimeout"
	case "optimeout":
		next = "operTimeout"
	default:
		next = timeoutName + "1"
	}
	return renameTimeout(seenIds, next)
}

func (b *codeGenOpBuilder) MakeOperation() (GenOperation, error) {
	debugLog("[%s %s] parsing operation (id: %q)", b.Method, b.Path, b.Operation.ID)
	// NOTE: we assume flatten is enabled by default (i.e. complex constructs are resolved from the models package),
	// but do not assume the spec is necessarily fully flattened (i.e. all schemas moved to definitions).
	//
	// Fully flattened means that all complex constructs are present as
	// definitions and models produced accordingly in ModelsPackage,
	// whereas minimal flatten simply ensures that there are no weird $ref's in the spec.
	//
	// When some complex anonymous constructs are specified, extra schemas are produced in the operations package.
	//
	// In all cases, resetting definitions to the _original_ (untransformed) spec is not an option:
	// we take from there the spec possibly already transformed by the GenDefinitions stage.
	resolver := newTypeResolver(b.GenOpts.LanguageOpts.ManglePackageName(b.ModelsPackage, defaultModelsTarget), b.DefaultImports[b.ModelsPackage], b.Doc)
	receiver := "o"

	operation := b.Operation
	var params, qp, pp, hp, fp GenParameters
	var hasQueryParams, hasPathParams, hasHeaderParams, hasFormParams, hasFileParams, hasFormValueParams, hasBodyParams bool
	paramsForOperation := b.Analyzed.ParamsFor(b.Method, b.Path)

	idMapping, timeoutName := paramMappings(paramsForOperation)

	for _, p := range paramsForOperation {
		cp, err := b.MakeParameter(receiver, resolver, p, idMapping)

		if err != nil {
			return GenOperation{}, err
		}
		if cp.IsQueryParam() {
			hasQueryParams = true
			qp = append(qp, cp)
		}
		if cp.IsFormParam() {
			if p.Type == file {
				hasFileParams = true
			}
			if p.Type != file {
				hasFormValueParams = true
			}
			hasFormParams = true
			fp = append(fp, cp)
		}
		if cp.IsPathParam() {
			hasPathParams = true
			pp = append(pp, cp)
		}
		if cp.IsHeaderParam() {
			hasHeaderParams = true
			hp = append(hp, cp)
		}
		if cp.IsBodyParam() {
			hasBodyParams = true
		}
		params = append(params, cp)
	}
	sort.Sort(params)
	sort.Sort(qp)
	sort.Sort(pp)
	sort.Sort(hp)
	sort.Sort(fp)

	var srs responses
	if operation.Responses != nil {
		srs = sortedResponses(operation.Responses.StatusCodeResponses)
	}
	responses := make([]GenResponse, 0, len(srs))
	var defaultResponse *GenResponse
	var successResponses []GenResponse
	if operation.Responses != nil {
		for _, v := range srs {
			name, ok := v.Response.Extensions.GetString(xGoName)
			if !ok {
				// look for name of well-known codes
				name = runtime.Statuses[v.Code]
				if name == "" {
					// non-standard codes deserve some name
					name = fmt.Sprintf("Status %d", v.Code)
				}
			}
			name = swag.ToJSONName(b.Name + " " + name)
			isSuccess := v.Code/100 == 2
			gr, err := b.MakeResponse(receiver, name, isSuccess, resolver, v.Code, v.Response)
			if err != nil {
				return GenOperation{}, err
			}
			if isSuccess {
				successResponses = append(successResponses, gr)
			}
			responses = append(responses, gr)
		}

		if operation.Responses.Default != nil {
			gr, err := b.MakeResponse(receiver, b.Name+" default", false, resolver, -1, *operation.Responses.Default)
			if err != nil {
				return GenOperation{}, err
			}
			defaultResponse = &gr
		}
	}

	// Always render a default response, even when no responses were defined
	if operation.Responses == nil || (operation.Responses.Default == nil && len(srs) == 0) {
		gr, err := b.MakeResponse(receiver, b.Name+" default", false, resolver, -1, spec.Response{})
		if err != nil {
			return GenOperation{}, err
		}
		defaultResponse = &gr
	}

	swsp := resolver.Doc.Spec()

	schemes, extraSchemes := gatherURISchemes(swsp, operation)
	originalSchemes := operation.Schemes
	originalExtraSchemes := getExtraSchemes(operation.Extensions)

	produces := producesOrDefault(operation.Produces, swsp.Produces, b.DefaultProduces)
	sort.Strings(produces)

	consumes := producesOrDefault(operation.Consumes, swsp.Consumes, b.DefaultConsumes)
	sort.Strings(consumes)

	var successResponse *GenResponse
	for _, resp := range successResponses {
		sr := resp
		if sr.IsSuccess {
			successResponse = &sr
			break
		}
	}

	var hasStreamingResponse bool
	if defaultResponse != nil && defaultResponse.Schema != nil && defaultResponse.Schema.IsStream {
		hasStreamingResponse = true
	}

	if !hasStreamingResponse {
		for _, sr := range successResponses {
			if !hasStreamingResponse && sr.Schema != nil && sr.Schema.IsStream {
				hasStreamingResponse = true
				break
			}
		}
	}

	if !hasStreamingResponse {
		for _, r := range responses {
			if r.Schema != nil && r.Schema.IsStream {
				hasStreamingResponse = true
				break
			}
		}
	}

	return GenOperation{
		GenCommon: GenCommon{
			Copyright:        b.GenOpts.Copyright,
			TargetImportPath: b.GenOpts.LanguageOpts.baseImport(b.GenOpts.Target),
		},
		Package:              b.GenOpts.LanguageOpts.ManglePackageName(b.APIPackage, defaultOperationsTarget),
		PackageAlias:         b.APIPackageAlias,
		RootPackage:          b.RootAPIPackage,
		Name:                 b.Name,
		Method:               b.Method,
		Path:                 b.Path,
		BasePath:             b.BasePath,
		Tags:                 operation.Tags,
		UseTags:              len(operation.Tags) > 0 && !b.GenOpts.SkipTagPackages,
		Description:          trimBOM(operation.Description),
		ReceiverName:         receiver,
		DefaultImports:       b.DefaultImports,
		Imports:              b.Imports,
		Params:               params,
		Summary:              trimBOM(operation.Summary),
		QueryParams:          qp,
		PathParams:           pp,
		HeaderParams:         hp,
		FormParams:           fp,
		HasQueryParams:       hasQueryParams,
		HasPathParams:        hasPathParams,
		HasHeaderParams:      hasHeaderParams,
		HasFormParams:        hasFormParams,
		HasFormValueParams:   hasFormValueParams,
		HasFileParams:        hasFileParams,
		HasBodyParams:        hasBodyParams,
		HasStreamingResponse: hasStreamingResponse,
		Authorized:           b.Authed,
		Security:             b.makeSecurityRequirements(receiver), // resolved security requirements, for codegen
		SecurityDefinitions:  b.makeSecuritySchemes(receiver),
		SecurityRequirements: securityRequirements(operation.Security), // raw security requirements, for doc
		Principal:            b.Principal,
		Responses:            responses,
		DefaultResponse:      defaultResponse,
		SuccessResponse:      successResponse,
		SuccessResponses:     successResponses,
		ExtraSchemas:         gatherExtraSchemas(b.ExtraSchemas),
		Schemes:              schemeOrDefault(schemes, b.DefaultScheme),
		SchemeOverrides:      originalSchemes,      // raw operation schemes, for doc
		ProducesMediaTypes:   produces,             // resolved produces, for codegen
		ConsumesMediaTypes:   consumes,             // resolved consumes, for codegen
		Produces:             operation.Produces,   // for doc
		Consumes:             operation.Consumes,   // for doc
		ExtraSchemes:         extraSchemes,         // resolved schemes, for codegen
		ExtraSchemeOverrides: originalExtraSchemes, // raw operation extra schemes, for doc
		TimeoutName:          timeoutName,
		Extensions:           operation.Extensions,
		StrictResponders:     b.GenOpts.StrictResponders,

		PrincipalIsNullable: b.GenOpts.PrincipalIsNullable(),
		ExternalDocs:        trimExternalDoc(operation.ExternalDocs),
	}, nil
}

func producesOrDefault(produces []string, fallback []string, defaultProduces string) []string {
	if len(produces) > 0 {
		return produces
	}
	if len(fallback) > 0 {
		return fallback
	}
	return []string{defaultProduces}
}

func schemeOrDefault(schemes []string, defaultScheme string) []string {
	if len(schemes) == 0 {
		return []string{defaultScheme}
	}
	return schemes
}

func (b *codeGenOpBuilder) MakeResponse(receiver, name string, isSuccess bool, resolver *typeResolver, code int, resp spec.Response) (GenResponse, error) {
	debugLog("[%s %s] making id %q", b.Method, b.Path, b.Operation.ID)

	// assume minimal flattening has been carried on, so there is not $ref in response (but some may remain in response schema)
	examples := make(GenResponseExamples, 0, len(resp.Examples))
	for k, v := range resp.Examples {
		examples = append(examples, GenResponseExample{MediaType: k, Example: v})
	}
	sort.Sort(examples)

	res := GenResponse{
		Package:          b.GenOpts.LanguageOpts.ManglePackageName(b.APIPackage, defaultOperationsTarget),
		ModelsPackage:    b.ModelsPackage,
		ReceiverName:     receiver,
		Name:             name,
		Description:      trimBOM(resp.Description),
		DefaultImports:   b.DefaultImports,
		Imports:          b.Imports,
		IsSuccess:        isSuccess,
		Code:             code,
		Method:           b.Method,
		Path:             b.Path,
		Extensions:       resp.Extensions,
		StrictResponders: b.GenOpts.StrictResponders,
		OperationName:    b.Name,
		Examples:         examples,
	}

	// prepare response headers
	for hName, header := range resp.Headers {
		hdr, err := b.MakeHeader(receiver, hName, header)
		if err != nil {
			return GenResponse{}, err
		}
		res.Headers = append(res.Headers, hdr)
	}
	sort.Sort(res.Headers)

	if resp.Schema != nil {
		// resolve schema model
		schema, ers := b.buildOperationSchema(fmt.Sprintf("%q", name), name+"Body", swag.ToGoName(name+"Body"), receiver, "i", resp.Schema, resolver)
		if ers != nil {
			return GenResponse{}, ers
		}
		res.Schema = &schema
	}
	return res, nil
}

func (b *codeGenOpBuilder) MakeHeader(receiver, name string, hdr spec.Header) (GenHeader, error) {
	tpe := simpleResolvedType(hdr.Type, hdr.Format, hdr.Items, &hdr.CommonValidations)

	id := swag.ToGoName(name)
	res := GenHeader{
		sharedValidations: sharedValidations{
			Required:          true,
			SchemaValidations: hdr.Validations(), // NOTE: Required is not defined by the Swagger schema for header. Set arbitrarily to true for convenience in templates.
		},
		resolvedType:     tpe,
		Package:          b.GenOpts.LanguageOpts.ManglePackageName(b.APIPackage, defaultOperationsTarget),
		ReceiverName:     receiver,
		ID:               id,
		Name:             name,
		Path:             fmt.Sprintf("%q", name),
		ValueExpression:  fmt.Sprintf("%s.%s", receiver, id),
		Description:      trimBOM(hdr.Description),
		Default:          hdr.Default,
		HasDefault:       hdr.Default != nil,
		Converter:        stringConverters[tpe.GoType],
		Formatter:        stringFormatters[tpe.GoType],
		ZeroValue:        tpe.Zero(),
		CollectionFormat: hdr.CollectionFormat,
		IndexVar:         "i",
	}
	res.HasValidations, res.HasSliceValidations = b.HasValidations(hdr.CommonValidations, res.resolvedType)

	hasChildValidations := false
	if hdr.Items != nil {
		pi, err := b.MakeHeaderItem(receiver, name+" "+res.IndexVar, res.IndexVar+"i", "fmt.Sprintf(\"%s.%v\", \"header\", "+res.IndexVar+")", res.Name+"I", hdr.Items, nil)
		if err != nil {
			return GenHeader{}, err
		}
		res.Child = &pi
		hasChildValidations = pi.HasValidations
	}
	// we feed the GenHeader structure the same way as we do for
	// GenParameter, even though there is currently no actual validation
	// for response headers.
	res.HasValidations = res.HasValidations || hasChildValidations

	return res, nil
}

func (b *codeGenOpBuilder) MakeHeaderItem(receiver, paramName, indexVar, path, valueExpression string, items, parent *spec.Items) (GenItems, error) {
	var res GenItems
	res.resolvedType = simpleResolvedType(items.Type, items.Format, items.Items, &items.CommonValidations)

	res.sharedValidations = sharedValidations{
		Required:          false,
		SchemaValidations: items.Validations(),
	}
	res.Name = paramName
	res.Path = path
	res.Location = "header"
	res.ValueExpression = swag.ToVarName(valueExpression)
	res.CollectionFormat = items.CollectionFormat
	res.Converter = stringConverters[res.GoType]
	res.Formatter = stringFormatters[res.GoType]
	res.IndexVar = indexVar
	res.HasValidations, res.HasSliceValidations = b.HasValidations(items.CommonValidations, res.resolvedType)
	res.IsEnumCI = b.GenOpts.AllowEnumCI || hasEnumCI(items.Extensions)

	if items.Items != nil {
		// Recursively follows nested arrays
		// IMPORTANT! transmitting a ValueExpression consistent with the parent's one
		hi, err := b.MakeHeaderItem(receiver, paramName+" "+indexVar, indexVar+"i", "fmt.Sprintf(\"%s.%v\", \"header\", "+indexVar+")", res.ValueExpression+"I", items.Items, items)
		if err != nil {
			return GenItems{}, err
		}
		res.Child = &hi
		hi.Parent = &res
		// Propagates HasValidations flag to outer Items definition (currently not in use: done to remain consistent with parameters)
		res.HasValidations = res.HasValidations || hi.HasValidations
	}

	return res, nil
}

// HasValidations resolves the validation status for simple schema objects
func (b *codeGenOpBuilder) HasValidations(sh spec.CommonValidations, rt resolvedType) (hasValidations bool, hasSliceValidations bool) {
	hasSliceValidations = sh.HasArrayValidations() || sh.HasEnum()
	hasValidations = sh.HasNumberValidations() || sh.HasStringValidations() || hasSliceValidations || hasFormatValidation(rt)
	return
}

func (b *codeGenOpBuilder) MakeParameterItem(receiver, paramName, indexVar, path, valueExpression, location string, resolver *typeResolver, items, parent *spec.Items) (GenItems, error) {
	debugLog("making parameter item recv=%s param=%s index=%s valueExpr=%s path=%s location=%s", receiver, paramName, indexVar, valueExpression, path, location)
	var res GenItems
	res.resolvedType = simpleResolvedType(items.Type, items.Format, items.Items, &items.CommonValidations)

	res.sharedValidations = sharedValidations{
		Required:          false,
		SchemaValidations: items.Validations(),
	}
	res.Name = paramName
	res.Path = path
	res.Location = location
	res.ValueExpression = swag.ToVarName(valueExpression)
	res.CollectionFormat = items.CollectionFormat
	res.Converter = stringConverters[res.GoType]
	res.Formatter = stringFormatters[res.GoType]
	res.IndexVar = indexVar

	res.HasValidations, res.HasSliceValidations = b.HasValidations(items.CommonValidations, res.resolvedType)
	res.IsEnumCI = b.GenOpts.AllowEnumCI || hasEnumCI(items.Extensions)
	res.NeedsIndex = res.HasValidations || res.Converter != "" || (res.IsCustomFormatter && !res.SkipParse)

	if items.Items != nil {
		// Recursively follows nested arrays
		// IMPORTANT! transmitting a ValueExpression consistent with the parent's one
		pi, err := b.MakeParameterItem(receiver, paramName+" "+indexVar, indexVar+"i", "fmt.Sprintf(\"%s.%v\", "+path+", "+indexVar+")", res.ValueExpression+"I", location, resolver, items.Items, items)
		if err != nil {
			return GenItems{}, err
		}
		res.Child = &pi
		pi.Parent = &res
		// Propagates HasValidations flag to outer Items definition
		res.HasValidations = res.HasValidations || pi.HasValidations
		res.NeedsIndex = res.NeedsIndex || pi.NeedsIndex
	}

	return res, nil
}

func (b *codeGenOpBuilder) MakeParameter(receiver string, resolver *typeResolver, param spec.Parameter, idMapping map[string]map[string]string) (GenParameter, error) {
	debugLog("[%s %s] making parameter %q", b.Method, b.Path, param.Name)

	// assume minimal flattening has been carried on, so there is not $ref in response (but some may remain in response schema)

	var child *GenItems
	id := swag.ToGoName(param.Name)
	if goName, ok := param.Extensions["x-go-name"]; ok {
		id, ok = goName.(string)
		if !ok {
			return GenParameter{}, fmt.Errorf(`%s %s, parameter %q: "x-go-name" field must be a string, not a %T`,
				b.Method, b.Path, param.Name, goName)
		}
	} else if len(idMapping) > 0 {
		id = idMapping[param.In][param.Name]
	}

	res := GenParameter{
		ID:               id,
		Name:             param.Name,
		ModelsPackage:    b.ModelsPackage,
		Path:             fmt.Sprintf("%q", param.Name),
		ValueExpression:  fmt.Sprintf("%s.%s", receiver, id),
		IndexVar:         "i",
		Default:          param.Default,
		HasDefault:       param.Default != nil,
		Description:      trimBOM(param.Description),
		ReceiverName:     receiver,
		CollectionFormat: param.CollectionFormat,
		Child:            child,
		Location:         param.In,
		AllowEmptyValue:  (param.In == "query" || param.In == "formData") && param.AllowEmptyValue,
		Extensions:       param.Extensions,
	}

	if param.In == "body" {
		// Process parameters declared in body (i.e. have a Schema)
		res.Required = param.Required
		if err := b.MakeBodyParameter(&res, resolver, param.Schema); err != nil {
			return GenParameter{}, err
		}
	} else {
		// Process parameters declared in other inputs: path, query, header (SimpleSchema)
		res.resolvedType = simpleResolvedType(param.Type, param.Format, param.Items, &param.CommonValidations)
		res.sharedValidations = sharedValidations{
			Required:          param.Required,
			SchemaValidations: param.Validations(),
		}

		res.ZeroValue = res.resolvedType.Zero()

		hasChildValidations := false
		if param.Items != nil {
			// Follow Items definition for array parameters
			pi, err := b.MakeParameterItem(receiver, param.Name+" "+res.IndexVar, res.IndexVar+"i", "fmt.Sprintf(\"%s.%v\", "+res.Path+", "+res.IndexVar+")", res.Name+"I", param.In, resolver, param.Items, nil)
			if err != nil {
				return GenParameter{}, err
			}
			res.Child = &pi
			// Propagates HasValidations from from child array
			hasChildValidations = pi.HasValidations
		}
		res.IsNullable = !param.Required && !param.AllowEmptyValue
		res.HasValidations, res.HasSliceValidations = b.HasValidations(param.CommonValidations, res.resolvedType)
		res.HasValidations = res.HasValidations || hasChildValidations
		res.IsEnumCI = b.GenOpts.AllowEnumCI || hasEnumCI(param.Extensions)
	}

	// Select codegen strategy for body param validation
	res.Converter = stringConverters[res.GoType]
	res.Formatter = stringFormatters[res.GoType]
	b.setBodyParamValidation(&res)

	return res, nil
}

// MakeBodyParameter constructs a body parameter schema
func (b *codeGenOpBuilder) MakeBodyParameter(res *GenParameter, resolver *typeResolver, sch *spec.Schema) error {
	// resolve schema model
	schema, ers := b.buildOperationSchema(res.Path, b.Operation.ID+"ParamsBody", swag.ToGoName(b.Operation.ID+" Body"), res.ReceiverName, res.IndexVar, sch, resolver)
	if ers != nil {
		return ers
	}
	res.Schema = &schema
	res.Schema.Required = res.Required // Required in body is managed independently from validations

	// build Child items for nested slices and maps
	var items *GenItems
	res.KeyVar = "k"
	res.Schema.KeyVar = "k"
	switch {
	case schema.IsMap && !schema.IsInterface:
		items = b.MakeBodyParameterItemsAndMaps(res, res.Schema.AdditionalProperties)
	case schema.IsArray:
		items = b.MakeBodyParameterItemsAndMaps(res, res.Schema.Items)
	default:
		items = new(GenItems)
	}

	// templates assume at least one .Child != nil
	res.Child = items
	schema.HasValidations = schema.HasValidations || items.HasValidations

	res.resolvedType = schema.resolvedType

	// simple and schema views share the same validations
	res.sharedValidations = schema.sharedValidations
	res.ZeroValue = schema.Zero()
	return nil
}

// MakeBodyParameterItemsAndMaps clones the .Items schema structure (resp. .AdditionalProperties) as a .GenItems structure
// for compatibility with simple param templates.
//
// Constructed children assume simple structures: any complex object is assumed to be resolved by a model or extra schema definition
func (b *codeGenOpBuilder) MakeBodyParameterItemsAndMaps(res *GenParameter, it *GenSchema) *GenItems {
	items := new(GenItems)
	if it != nil {
		var prev *GenItems
		next := items
		if res.Schema.IsArray {
			next.Path = "fmt.Sprintf(\"%s.%v\", " + res.Path + ", " + res.IndexVar + ")"
		} else if res.Schema.IsMap {
			next.Path = "fmt.Sprintf(\"%s.%v\", " + res.Path + ", " + res.KeyVar + ")"
		}
		next.Name = res.Name + " " + res.Schema.IndexVar
		next.IndexVar = res.Schema.IndexVar + "i"
		next.KeyVar = res.Schema.KeyVar + "k"
		next.ValueExpression = swag.ToVarName(res.Name + "I")
		next.Location = "body"
		for it != nil {
			next.resolvedType = it.resolvedType
			next.sharedValidations = it.sharedValidations
			next.Formatter = stringFormatters[it.SwaggerFormat]
			next.Converter = stringConverters[res.GoType]
			next.Parent = prev
			_, next.IsCustomFormatter = customFormatters[it.GoType]
			next.IsCustomFormatter = next.IsCustomFormatter && !it.IsStream

			// special instruction to avoid using CollectionFormat for body params
			next.SkipParse = true

			if prev != nil {
				if prev.IsArray {
					next.Path = "fmt.Sprintf(\"%s.%v\", " + prev.Path + ", " + prev.IndexVar + ")"
				} else if prev.IsMap {
					next.Path = "fmt.Sprintf(\"%s.%v\", " + prev.Path + ", " + prev.KeyVar + ")"
				}
				next.Name = prev.Name + prev.IndexVar
				next.IndexVar = prev.IndexVar + "i"
				next.KeyVar = prev.KeyVar + "k"
				next.ValueExpression = swag.ToVarName(prev.ValueExpression + "I")
				prev.Child = next
			}

			// found a complex or aliased thing
			// hide details from the aliased type and stop recursing
			if next.IsAliased || next.IsComplexObject {
				next.IsArray = false
				next.IsMap = false
				next.IsCustomFormatter = false
				next.IsComplexObject = true
				next.IsAliased = true
				break
			}
			if next.IsInterface || next.IsStream || next.IsBase64 {
				next.HasValidations = false
			}
			next.NeedsIndex = next.HasValidations || next.Converter != "" || (next.IsCustomFormatter && !next.SkipParse)
			prev = next
			next = new(GenItems)

			switch {
			case it.Items != nil:
				it = it.Items
			case it.AdditionalProperties != nil:
				it = it.AdditionalProperties
			default:
				it = nil
			}
		}
		// propagate HasValidations
		var propag func(child *GenItems) (bool, bool)
		propag = func(child *GenItems) (bool, bool) {
			if child == nil {
				return false, false
			}
			cValidations, cIndex := propag(child.Child)
			child.HasValidations = child.HasValidations || cValidations
			child.NeedsIndex = child.HasValidations || child.Converter != "" || (child.IsCustomFormatter && !child.SkipParse) || cIndex
			return child.HasValidations, child.NeedsIndex
		}
		items.HasValidations, items.NeedsIndex = propag(items)

		// resolve nullability conflicts when declaring body as a map of array of an anonymous complex object
		// (e.g. refer to an extra schema type, which is nullable, but not rendered as a pointer in arrays or maps)
		// Rule: outer type rules (with IsMapNullOverride), inner types are fixed
		var fixNullable func(child *GenItems) string
		fixNullable = func(child *GenItems) string {
			if !child.IsArray && !child.IsMap {
				if child.IsComplexObject {
					return child.GoType
				}
				return ""
			}
			if innerType := fixNullable(child.Child); innerType != "" {
				if child.IsMapNullOverride && child.IsArray {
					child.GoType = "[]" + innerType
					return child.GoType
				}
			}
			return ""
		}
		fixNullable(items)
	}
	return items
}

func (b *codeGenOpBuilder) setBodyParamValidation(p *GenParameter) {
	// Determine validation strategy for body param.
	//
	// Here are the distinct strategies:
	// - the body parameter is a model object => delegates
	// - the body parameter is an array of model objects => carry on slice validations, then iterate and delegate
	// - the body parameter is a map of model objects => iterate and delegate
	// - the body parameter is an array of simple objects (including maps)
	// - the body parameter is a map of simple objects (including arrays)
	if p.IsBodyParam() {
		var hasSimpleBodyParams, hasSimpleBodyItems, hasSimpleBodyMap, hasModelBodyParams, hasModelBodyItems, hasModelBodyMap bool
		s := p.Schema
		if s != nil {
			doNot := s.IsInterface || s.IsStream || s.IsBase64
			// composition of primitive fields must be properly identified: hack this through
			_, isPrimitive := primitives[s.GoType]
			_, isFormatter := customFormatters[s.GoType]
			isComposedPrimitive := s.IsPrimitive && !(isPrimitive || isFormatter)

			hasSimpleBodyParams = !s.IsComplexObject && !s.IsAliased && !isComposedPrimitive && !doNot
			hasModelBodyParams = (s.IsComplexObject || s.IsAliased || isComposedPrimitive) && !doNot

			if s.IsArray && s.Items != nil {
				it := s.Items
				doNot = it.IsInterface || it.IsStream || it.IsBase64
				hasSimpleBodyItems = !it.IsComplexObject && !(it.IsAliased || doNot)
				hasModelBodyItems = (it.IsComplexObject || it.IsAliased) && !doNot
			}
			if s.IsMap && s.AdditionalProperties != nil {
				it := s.AdditionalProperties
				hasSimpleBodyMap = !it.IsComplexObject && !(it.IsAliased || doNot)
				hasModelBodyMap = !hasSimpleBodyMap && !doNot
			}
		}
		// set validation strategy for body param
		p.HasSimpleBodyParams = hasSimpleBodyParams
		p.HasSimpleBodyItems = hasSimpleBodyItems
		p.HasModelBodyParams = hasModelBodyParams
		p.HasModelBodyItems = hasModelBodyItems
		p.HasModelBodyMap = hasModelBodyMap
		p.HasSimpleBodyMap = hasSimpleBodyMap
	}

}

// makeSecuritySchemes produces a sorted list of security schemes for this operation
func (b *codeGenOpBuilder) makeSecuritySchemes(receiver string) GenSecuritySchemes {
	return gatherSecuritySchemes(b.SecurityDefinitions, b.Name, b.Principal, receiver, b.GenOpts.PrincipalIsNullable())
}

// makeSecurityRequirements produces a sorted list of security requirements for this operation.
// As for current, these requirements are not used by codegen (sec. requirement is determined at runtime).
// We keep the order of the slice from the original spec, but sort the inner slice which comes from a map,
// as well as the map of scopes.
func (b *codeGenOpBuilder) makeSecurityRequirements(receiver string) []GenSecurityRequirements {
	if b.Security == nil {
		// nil (default requirement) is different than [] (no requirement)
		return nil
	}

	securityRequirements := make([]GenSecurityRequirements, 0, len(b.Security))
	for _, req := range b.Security {
		jointReq := make(GenSecurityRequirements, 0, len(req))
		for _, j := range req {
			scopes := j.Scopes
			sort.Strings(scopes)
			jointReq = append(jointReq, GenSecurityRequirement{
				Name:   j.Name,
				Scopes: scopes,
			})
		}
		// sort joint requirements (come from a map in spec)
		sort.Sort(jointReq)
		securityRequirements = append(securityRequirements, jointReq)
	}
	return securityRequirements
}

// cloneSchema returns a deep copy of a schema
func (b *codeGenOpBuilder) cloneSchema(schema *spec.Schema) *spec.Schema {
	savedSchema := &spec.Schema{}
	schemaRep, _ := json.Marshal(schema)
	_ = json.Unmarshal(schemaRep, savedSchema)
	return savedSchema
}

// saveResolveContext keeps a copy of known definitions and schema to properly roll back on a makeGenSchema() call
// This uses a deep clone the spec document to construct a type resolver which knows about definitions when the making of this operation started,
// and only these definitions. We are not interested in the "original spec", but in the already transformed spec.
func (b *codeGenOpBuilder) saveResolveContext(resolver *typeResolver, schema *spec.Schema) (*typeResolver, *spec.Schema) {
	if b.PristineDoc == nil {
		b.PristineDoc = b.Doc.Pristine()
	}
	rslv := newTypeResolver(b.GenOpts.LanguageOpts.ManglePackageName(resolver.ModelsPackage, defaultModelsTarget), b.DefaultImports[b.ModelsPackage], b.PristineDoc)

	return rslv, b.cloneSchema(schema)
}

// liftExtraSchemas constructs the schema for an anonymous construct with some ExtraSchemas.
//
// When some ExtraSchemas are produced from something else than a definition,
// this indicates we are not running in fully flattened mode and we need to render
// these ExtraSchemas in the operation's package.
// We need to rebuild the schema with a new type resolver to reflect this change in the
// models package.
func (b *codeGenOpBuilder) liftExtraSchemas(resolver, rslv *typeResolver, bs *spec.Schema, sc *schemaGenContext) (schema *GenSchema, err error) {
	// restore resolving state before previous call to makeGenSchema()
	sc.Schema = *bs

	pg := sc.shallowClone()
	pkg := b.GenOpts.LanguageOpts.ManglePackageName(resolver.ModelsPackage, defaultModelsTarget)

	// make a resolver for current package (i.e. operations)
	pg.TypeResolver = newTypeResolver("", b.DefaultImports[b.APIPackage], rslv.Doc).withKeepDefinitionsPackage(pkg)
	pg.ExtraSchemas = make(map[string]GenSchema, len(sc.ExtraSchemas))
	pg.UseContainerInName = true

	// rebuild schema within local package
	if err = pg.makeGenSchema(); err != nil {
		return
	}

	// lift nested extra schemas (inlined types)
	if b.ExtraSchemas == nil {
		b.ExtraSchemas = make(map[string]GenSchema, len(pg.ExtraSchemas))
	}
	for _, v := range pg.ExtraSchemas {
		vv := v
		if !v.IsStream {
			b.ExtraSchemas[vv.Name] = vv
		}
	}
	schema = &pg.GenSchema
	return
}

// buildOperationSchema constructs a schema for an operation (for body params or responses).
// It determines if the schema is readily available from the models package,
// or if a schema has to be generated in the operations package (i.e. is anonymous).
// Whenever an anonymous schema needs some extra schemas, we also determine if these extras are
// available from models or must be generated alongside the schema in the operations package.
//
// Duplicate extra schemas are pruned later on, when operations grouping in packages (e.g. from tags) takes place.
func (b *codeGenOpBuilder) buildOperationSchema(schemaPath, containerName, schemaName, receiverName, indexVar string, sch *spec.Schema, resolver *typeResolver) (GenSchema, error) {
	var schema GenSchema

	if sch == nil {
		sch = &spec.Schema{}
	}
	shallowClonedResolver := *resolver
	shallowClonedResolver.ModelsFullPkg = b.DefaultImports[b.ModelsPackage]
	rslv := &shallowClonedResolver

	sc := schemaGenContext{
		Path:                       schemaPath,
		Name:                       containerName,
		Receiver:                   receiverName,
		ValueExpr:                  receiverName,
		IndexVar:                   indexVar,
		Schema:                     *sch,
		Required:                   false,
		TypeResolver:               rslv,
		Named:                      false,
		IncludeModel:               true,
		IncludeValidator:           b.GenOpts.IncludeValidator,
		StrictAdditionalProperties: b.GenOpts.StrictAdditionalProperties,
		ExtraSchemas:               make(map[string]GenSchema),
		StructTags:                 b.GenOpts.StructTags,
	}

	var (
		br *typeResolver
		bs *spec.Schema
	)

	if sch.Ref.String() == "" {
		// backup the type resolver context
		// (not needed when the schema has a name)
		br, bs = b.saveResolveContext(rslv, sch)
	}

	if err := sc.makeGenSchema(); err != nil {
		return GenSchema{}, err
	}
	for alias, pkg := range findImports(&sc.GenSchema) {
		b.Imports[alias] = pkg
	}

	if sch.Ref.String() == "" && len(sc.ExtraSchemas) > 0 {
		newSchema, err := b.liftExtraSchemas(resolver, br, bs, &sc)
		if err != nil {
			return GenSchema{}, err
		}
		if newSchema != nil {
			schema = *newSchema
		}
	} else {
		schema = sc.GenSchema
	}

	if schema.IsAnonymous {
		// a generated name for anonymous schema
		// TODO: support x-go-name
		hasProperties := len(schema.Properties) > 0
		isAllOf := len(schema.AllOf) > 0
		isInterface := schema.IsInterface
		hasValidations := schema.HasValidations

		// TODO: remove this and find a better way to get package name for anonymous models
		// get the package that the param will be generated. Used by generate CLI
		pkg := "operations"
		if len(b.Operation.Tags) != 0 {
			pkg = b.Operation.Tags[0]
		}

		// for complex anonymous objects, produce an extra schema
		if hasProperties || isAllOf {
			if b.ExtraSchemas == nil {
				b.ExtraSchemas = make(map[string]GenSchema)
			}
			schema.Name = schemaName
			schema.GoType = schemaName
			schema.IsAnonymous = false
			schema.Pkg = pkg
			b.ExtraSchemas[schemaName] = schema

			// constructs new schema to refer to the newly created type
			schema = GenSchema{}
			schema.IsAnonymous = false
			schema.IsComplexObject = true
			schema.SwaggerType = schemaName
			schema.HasValidations = hasValidations
			schema.GoType = schemaName
			schema.Pkg = pkg
		} else if isInterface {
			schema = GenSchema{}
			schema.IsAnonymous = false
			schema.IsComplexObject = false
			schema.IsInterface = true
			schema.HasValidations = false
			schema.GoType = iface
		}
	}
	return schema, nil
}

func intersectTags(left, right []string) []string {
	// dedupe
	uniqueTags := make(map[string]struct{}, maxInt(len(left), len(right)))
	for _, l := range left {
		if len(right) == 0 || swag.ContainsStrings(right, l) {
			uniqueTags[l] = struct{}{}
		}
	}
	filtered := make([]string, 0, len(uniqueTags))
	// stable output across generations, preserving original order
	for _, k := range left {
		if _, ok := uniqueTags[k]; !ok {
			continue
		}
		filtered = append(filtered, k)
		delete(uniqueTags, k)
	}
	return filtered
}

// analyze tags for an operation
func (b *codeGenOpBuilder) analyzeTags() (string, []string, bool) {
	var (
		filter         []string
		tag            string
		hasTagOverride bool
	)
	if b.GenOpts != nil {
		filter = b.GenOpts.Tags
	}
	intersected := intersectTags(pruneEmpty(b.Operation.Tags), filter)
	if !b.GenOpts.SkipTagPackages && len(intersected) > 0 {
		// override generation with: x-go-operation-tag
		tag, hasTagOverride = b.Operation.Extensions.GetString(xGoOperationTag)
		if !hasTagOverride {
			// TODO(fred): this part should be delegated to some new TagsFor(operation) in go-openapi/analysis
			tag = intersected[0]
			gtags := b.Doc.Spec().Tags
			for _, gtag := range gtags {
				if gtag.Name != tag {
					continue
				}
				//  honor x-go-name in tag
				if name, hasGoName := gtag.Extensions.GetString(xGoName); hasGoName {
					tag = name
					break
				}
				//  honor x-go-operation-tag in tag
				if name, hasOpName := gtag.Extensions.GetString(xGoOperationTag); hasOpName {
					tag = name
					break
				}
			}
		}
	}
	if tag == b.APIPackage {
		// conflict with "operations" package is handled separately
		tag = renameOperationPackage(intersected, tag)
	}
	b.APIPackage = b.GenOpts.LanguageOpts.ManglePackageName(tag, b.APIPackage) // actual package name
	b.APIPackageAlias = deconflictTag(intersected, b.APIPackage)               // deconflicted import alias
	return tag, intersected, len(filter) == 0 || len(filter) > 0 && len(intersected) > 0
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// deconflictTag ensures generated packages for operations based on tags do not conflict
// with other imports
func deconflictTag(seenTags []string, pkg string) string {
	return deconflictPkg(pkg, func(pkg string) string { return renameOperationPackage(seenTags, pkg) })
}

// deconflictPrincipal ensures that whenever an external principal package is added, it doesn't conflict
// with standard inports
func deconflictPrincipal(pkg string) string {
	switch pkg {
	case "principal":
		return renamePrincipalPackage(pkg)
	default:
		return deconflictPkg(pkg, renamePrincipalPackage)
	}
}

// deconflictPkg renames package names which conflict with standard imports
func deconflictPkg(pkg string, renamer func(string) string) string {
	switch pkg {
	// package conflict with variables
	case "api", "httptransport", "formats", "server":
		fallthrough
	// package conflict with go-openapi imports
	case "errors", "runtime", "middleware", "security", "spec", "strfmt", "loads", "swag", "validate":
		fallthrough
	// package conflict with stdlib/other lib imports
	case "tls", "http", "fmt", "strings", "log", "flags", "pflag", "json", "time":
		return renamer(pkg)
	}
	return pkg
}

func renameOperationPackage(seenTags []string, pkg string) string {
	current := strings.ToLower(pkg) + "ops"
	if len(seenTags) == 0 {
		return current
	}
	for swag.ContainsStringsCI(seenTags, current) {
		current += "1"
	}
	return current
}

func renamePrincipalPackage(pkg string) string {
	// favors readability over perfect deconfliction
	return "auth"
}

func renameServerPackage(pkg string) string {
	// favors readability over perfect deconfliction
	return "swagger" + pkg + "srv"
}

func renameAPIPackage(pkg string) string {
	// favors readability over perfect deconfliction
	return "swagger" + pkg
}

func renameImplementationPackage(pkg string) string {
	// favors readability over perfect deconfliction
	return "swagger" + pkg + "impl"
}
