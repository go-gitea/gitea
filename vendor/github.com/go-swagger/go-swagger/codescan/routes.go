package codescan

import (
	"fmt"

	"github.com/go-openapi/spec"
)

func opConsumesSetter(op *spec.Operation) func([]string) {
	return func(consumes []string) { op.Consumes = consumes }
}

func opProducesSetter(op *spec.Operation) func([]string) {
	return func(produces []string) { op.Produces = produces }
}

func opSchemeSetter(op *spec.Operation) func([]string) {
	return func(schemes []string) { op.Schemes = schemes }
}

func opSecurityDefsSetter(op *spec.Operation) func([]map[string][]string) {
	return func(securityDefs []map[string][]string) { op.Security = securityDefs }
}

func opResponsesSetter(op *spec.Operation) func(*spec.Response, map[int]spec.Response) {
	return func(def *spec.Response, scr map[int]spec.Response) {
		if op.Responses == nil {
			op.Responses = new(spec.Responses)
		}
		op.Responses.Default = def
		op.Responses.StatusCodeResponses = scr
	}
}

func opParamSetter(op *spec.Operation) func([]*spec.Parameter) {
	return func(params []*spec.Parameter) {
		for _, v := range params {
			op.AddParam(v)
		}
	}
}

type routesBuilder struct {
	ctx         *scanCtx
	route       parsedPathContent
	definitions map[string]spec.Schema
	operations  map[string]*spec.Operation
	responses   map[string]spec.Response
	parameters  []*spec.Parameter
}

func (r *routesBuilder) Build(tgt *spec.Paths) error {

	pthObj := tgt.Paths[r.route.Path]
	op := setPathOperation(
		r.route.Method, r.route.ID,
		&pthObj, r.operations[r.route.ID])

	op.Tags = r.route.Tags

	sp := new(sectionedParser)
	sp.setTitle = func(lines []string) { op.Summary = joinDropLast(lines) }
	sp.setDescription = func(lines []string) { op.Description = joinDropLast(lines) }
	sr := newSetResponses(r.definitions, r.responses, opResponsesSetter(op))
	spa := newSetParams(r.parameters, opParamSetter(op))
	sp.taggers = []tagParser{
		newMultiLineTagParser("Consumes", newMultilineDropEmptyParser(rxConsumes, opConsumesSetter(op)), false),
		newMultiLineTagParser("Produces", newMultilineDropEmptyParser(rxProduces, opProducesSetter(op)), false),
		newSingleLineTagParser("Schemes", newSetSchemes(opSchemeSetter(op))),
		newMultiLineTagParser("Security", newSetSecurity(rxSecuritySchemes, opSecurityDefsSetter(op)), false),
		newMultiLineTagParser("Parameters", spa, false),
		newMultiLineTagParser("Responses", sr, false),
		newSingleLineTagParser("Deprecated", &setDeprecatedOp{op}),
	}
	if err := sp.Parse(r.route.Remaining); err != nil {
		return fmt.Errorf("operation (%s): %v", op.ID, err)
	}

	if tgt.Paths == nil {
		tgt.Paths = make(map[string]spec.PathItem)
	}
	tgt.Paths[r.route.Path] = pthObj
	return nil
}
