package codescan

import (
	"github.com/go-openapi/spec"
)

func newSpecBuilder(input *spec.Swagger, sc *scanCtx, scanModels bool) *specBuilder {
	if input == nil {
		input = new(spec.Swagger)
		input.Swagger = "2.0"
	}

	if input.Paths == nil {
		input.Paths = new(spec.Paths)
	}
	if input.Definitions == nil {
		input.Definitions = make(map[string]spec.Schema)
	}
	if input.Responses == nil {
		input.Responses = make(map[string]spec.Response)
	}
	if input.Extensions == nil {
		input.Extensions = make(spec.Extensions)
	}

	return &specBuilder{
		ctx:         sc,
		input:       input,
		scanModels:  scanModels,
		operations:  collectOperationsFromInput(input),
		definitions: input.Definitions,
		responses:   input.Responses,
	}
}

type specBuilder struct {
	scanModels  bool
	input       *spec.Swagger
	ctx         *scanCtx
	discovered  []*entityDecl
	definitions map[string]spec.Schema
	responses   map[string]spec.Response
	operations  map[string]*spec.Operation
}

func (s *specBuilder) Build() (*spec.Swagger, error) {
	if err := s.buildModels(); err != nil {
		return nil, err
	}

	if err := s.buildParameters(); err != nil {
		return nil, err
	}

	if err := s.buildRespones(); err != nil {
		return nil, err
	}

	// build definitions dictionary
	if err := s.buildDiscovered(); err != nil {
		return nil, err
	}

	if err := s.buildRoutes(); err != nil {
		return nil, err
	}

	if err := s.buildOperations(); err != nil {
		return nil, err
	}

	if err := s.buildMeta(); err != nil {
		return nil, err
	}

	if s.input.Swagger == "" {
		s.input.Swagger = "2.0"
	}

	return s.input, nil
}

func (s *specBuilder) buildDiscovered() error {
	// loop over discovered until all the items are in definitions
	keepGoing := len(s.discovered) > 0
	for keepGoing {
		var queue []*entityDecl
		for _, d := range s.discovered {
			nm, _ := d.Names()
			if _, ok := s.definitions[nm]; !ok {
				queue = append(queue, d)
			}
		}
		s.discovered = nil
		for _, sd := range queue {
			if err := s.buildDiscoveredSchema(sd); err != nil {
				return err
			}
		}
		keepGoing = len(s.discovered) > 0
	}

	return nil
}

func (s *specBuilder) buildDiscoveredSchema(decl *entityDecl) error {
	sb := &schemaBuilder{
		ctx:        s.ctx,
		decl:       decl,
		discovered: s.discovered,
	}
	if err := sb.Build(s.definitions); err != nil {
		return err
	}
	s.discovered = append(s.discovered, sb.postDecls...)
	return nil
}

func (s *specBuilder) buildMeta() error {
	// build swagger object
	for _, decl := range s.ctx.app.Meta {
		if err := newMetaParser(s.input).Parse(decl.Comments); err != nil {
			return err
		}
	}
	return nil
}

func (s *specBuilder) buildOperations() error {
	for _, pp := range s.ctx.app.Operations {
		ob := &operationsBuilder{
			operations: s.operations,
			ctx:        s.ctx,
			path:       pp,
		}
		if err := ob.Build(s.input.Paths); err != nil {
			return err
		}
	}
	return nil
}

func (s *specBuilder) buildRoutes() error {
	// build paths dictionary
	for _, pp := range s.ctx.app.Routes {
		rb := &routesBuilder{
			ctx:         s.ctx,
			route:       pp,
			responses:   s.responses,
			operations:  s.operations,
			definitions: s.definitions,
		}
		if err := rb.Build(s.input.Paths); err != nil {
			return err
		}
	}

	return nil
}

func (s *specBuilder) buildRespones() error {
	// build responses dictionary
	for _, decl := range s.ctx.app.Responses {
		rb := &responseBuilder{
			ctx:  s.ctx,
			decl: decl,
		}
		if err := rb.Build(s.responses); err != nil {
			return err
		}
		s.discovered = append(s.discovered, rb.postDecls...)
	}
	return nil
}

func (s *specBuilder) buildParameters() error {
	// build parameters dictionary
	for _, decl := range s.ctx.app.Parameters {
		pb := &parameterBuilder{
			ctx:  s.ctx,
			decl: decl,
		}
		if err := pb.Build(s.operations); err != nil {
			return err
		}
		s.discovered = append(s.discovered, pb.postDecls...)
	}
	return nil
}

func (s *specBuilder) buildModels() error {
	// build models dictionary
	if !s.scanModels {
		return nil
	}
	for _, decl := range s.ctx.app.Models {
		if err := s.buildDiscoveredSchema(decl); err != nil {
			return err
		}
	}
	return nil
}

func collectOperationsFromInput(input *spec.Swagger) map[string]*spec.Operation {
	operations := make(map[string]*spec.Operation)
	if input != nil && input.Paths != nil {
		for _, pth := range input.Paths.Paths {
			if pth.Get != nil {
				operations[pth.Get.ID] = pth.Get
			}
			if pth.Post != nil {
				operations[pth.Post.ID] = pth.Post
			}
			if pth.Put != nil {
				operations[pth.Put.ID] = pth.Put
			}
			if pth.Patch != nil {
				operations[pth.Patch.ID] = pth.Patch
			}
			if pth.Delete != nil {
				operations[pth.Delete.ID] = pth.Delete
			}
			if pth.Head != nil {
				operations[pth.Head.ID] = pth.Head
			}
			if pth.Options != nil {
				operations[pth.Options.ID] = pth.Options
			}
		}
	}
	return operations
}
