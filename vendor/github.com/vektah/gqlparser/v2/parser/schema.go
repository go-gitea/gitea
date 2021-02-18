package parser

import (
	. "github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/lexer"
)

func ParseSchema(source *Source) (*SchemaDocument, *gqlerror.Error) {
	p := parser{
		lexer: lexer.New(source),
	}
	ast, err := p.parseSchemaDocument(), p.err
	if err != nil {
		return nil, err
	}

	for _, def := range ast.Definitions {
		def.BuiltIn = source.BuiltIn
	}
	for _, def := range ast.Extensions {
		def.BuiltIn = source.BuiltIn
	}

	return ast, nil
}

func ParseSchemas(inputs ...*Source) (*SchemaDocument, *gqlerror.Error) {
	ast := &SchemaDocument{}
	for _, input := range inputs {
		inputAst, err := ParseSchema(input)
		if err != nil {
			return nil, err
		}
		ast.Merge(inputAst)
	}
	return ast, nil
}

func (p *parser) parseSchemaDocument() *SchemaDocument {
	var doc SchemaDocument
	doc.Position = p.peekPos()
	for p.peek().Kind != lexer.EOF {
		if p.err != nil {
			return nil
		}

		var description string
		if p.peek().Kind == lexer.BlockString || p.peek().Kind == lexer.String {
			description = p.parseDescription()
		}

		if p.peek().Kind != lexer.Name {
			p.unexpectedError()
			break
		}

		switch p.peek().Value {
		case "scalar", "type", "interface", "union", "enum", "input":
			doc.Definitions = append(doc.Definitions, p.parseTypeSystemDefinition(description))
		case "schema":
			doc.Schema = append(doc.Schema, p.parseSchemaDefinition(description))
		case "directive":
			doc.Directives = append(doc.Directives, p.parseDirectiveDefinition(description))
		case "extend":
			if description != "" {
				p.unexpectedToken(p.prev)
			}
			p.parseTypeSystemExtension(&doc)
		default:
			p.unexpectedError()
			return nil
		}
	}

	return &doc
}

func (p *parser) parseDescription() string {
	token := p.peek()

	if token.Kind != lexer.BlockString && token.Kind != lexer.String {
		return ""
	}

	return p.next().Value
}

func (p *parser) parseTypeSystemDefinition(description string) *Definition {
	tok := p.peek()
	if tok.Kind != lexer.Name {
		p.unexpectedError()
		return nil
	}

	switch tok.Value {
	case "scalar":
		return p.parseScalarTypeDefinition(description)
	case "type":
		return p.parseObjectTypeDefinition(description)
	case "interface":
		return p.parseInterfaceTypeDefinition(description)
	case "union":
		return p.parseUnionTypeDefinition(description)
	case "enum":
		return p.parseEnumTypeDefinition(description)
	case "input":
		return p.parseInputObjectTypeDefinition(description)
	default:
		p.unexpectedError()
		return nil
	}
}

func (p *parser) parseSchemaDefinition(description string) *SchemaDefinition {
	p.expectKeyword("schema")

	def := SchemaDefinition{Description: description}
	def.Position = p.peekPos()
	def.Description = description
	def.Directives = p.parseDirectives(true)

	p.some(lexer.BraceL, lexer.BraceR, func() {
		def.OperationTypes = append(def.OperationTypes, p.parseOperationTypeDefinition())
	})
	return &def
}

func (p *parser) parseOperationTypeDefinition() *OperationTypeDefinition {
	var op OperationTypeDefinition
	op.Position = p.peekPos()
	op.Operation = p.parseOperationType()
	p.expect(lexer.Colon)
	op.Type = p.parseName()
	return &op
}

func (p *parser) parseScalarTypeDefinition(description string) *Definition {
	p.expectKeyword("scalar")

	var def Definition
	def.Position = p.peekPos()
	def.Kind = Scalar
	def.Description = description
	def.Name = p.parseName()
	def.Directives = p.parseDirectives(true)
	return &def
}

func (p *parser) parseObjectTypeDefinition(description string) *Definition {
	p.expectKeyword("type")

	var def Definition
	def.Position = p.peekPos()
	def.Kind = Object
	def.Description = description
	def.Name = p.parseName()
	def.Interfaces = p.parseImplementsInterfaces()
	def.Directives = p.parseDirectives(true)
	def.Fields = p.parseFieldsDefinition()
	return &def
}

func (p *parser) parseImplementsInterfaces() []string {
	var types []string
	if p.peek().Value == "implements" {
		p.next()
		// optional leading ampersand
		p.skip(lexer.Amp)

		types = append(types, p.parseName())
		for p.skip(lexer.Amp) && p.err == nil {
			types = append(types, p.parseName())
		}
	}
	return types
}

func (p *parser) parseFieldsDefinition() FieldList {
	var defs FieldList
	p.some(lexer.BraceL, lexer.BraceR, func() {
		defs = append(defs, p.parseFieldDefinition())
	})
	return defs
}

func (p *parser) parseFieldDefinition() *FieldDefinition {
	var def FieldDefinition
	def.Position = p.peekPos()
	def.Description = p.parseDescription()
	def.Name = p.parseName()
	def.Arguments = p.parseArgumentDefs()
	p.expect(lexer.Colon)
	def.Type = p.parseTypeReference()
	def.Directives = p.parseDirectives(true)

	return &def
}

func (p *parser) parseArgumentDefs() ArgumentDefinitionList {
	var args ArgumentDefinitionList
	p.some(lexer.ParenL, lexer.ParenR, func() {
		args = append(args, p.parseArgumentDef())
	})
	return args
}

func (p *parser) parseArgumentDef() *ArgumentDefinition {
	var def ArgumentDefinition
	def.Position = p.peekPos()
	def.Description = p.parseDescription()
	def.Name = p.parseName()
	p.expect(lexer.Colon)
	def.Type = p.parseTypeReference()
	if p.skip(lexer.Equals) {
		def.DefaultValue = p.parseValueLiteral(true)
	}
	def.Directives = p.parseDirectives(true)
	return &def
}

func (p *parser) parseInputValueDef() *FieldDefinition {
	var def FieldDefinition
	def.Position = p.peekPos()
	def.Description = p.parseDescription()
	def.Name = p.parseName()
	p.expect(lexer.Colon)
	def.Type = p.parseTypeReference()
	if p.skip(lexer.Equals) {
		def.DefaultValue = p.parseValueLiteral(true)
	}
	def.Directives = p.parseDirectives(true)
	return &def
}

func (p *parser) parseInterfaceTypeDefinition(description string) *Definition {
	p.expectKeyword("interface")

	var def Definition
	def.Position = p.peekPos()
	def.Kind = Interface
	def.Description = description
	def.Name = p.parseName()
	def.Directives = p.parseDirectives(true)
	def.Fields = p.parseFieldsDefinition()
	return &def
}

func (p *parser) parseUnionTypeDefinition(description string) *Definition {
	p.expectKeyword("union")

	var def Definition
	def.Position = p.peekPos()
	def.Kind = Union
	def.Description = description
	def.Name = p.parseName()
	def.Directives = p.parseDirectives(true)
	def.Types = p.parseUnionMemberTypes()
	return &def
}

func (p *parser) parseUnionMemberTypes() []string {
	var types []string
	if p.skip(lexer.Equals) {
		// optional leading pipe
		p.skip(lexer.Pipe)

		types = append(types, p.parseName())
		for p.skip(lexer.Pipe) && p.err == nil {
			types = append(types, p.parseName())
		}
	}
	return types
}

func (p *parser) parseEnumTypeDefinition(description string) *Definition {
	p.expectKeyword("enum")

	var def Definition
	def.Position = p.peekPos()
	def.Kind = Enum
	def.Description = description
	def.Name = p.parseName()
	def.Directives = p.parseDirectives(true)
	def.EnumValues = p.parseEnumValuesDefinition()
	return &def
}

func (p *parser) parseEnumValuesDefinition() EnumValueList {
	var values EnumValueList
	p.some(lexer.BraceL, lexer.BraceR, func() {
		values = append(values, p.parseEnumValueDefinition())
	})
	return values
}

func (p *parser) parseEnumValueDefinition() *EnumValueDefinition {
	return &EnumValueDefinition{
		Position:    p.peekPos(),
		Description: p.parseDescription(),
		Name:        p.parseName(),
		Directives:  p.parseDirectives(true),
	}
}

func (p *parser) parseInputObjectTypeDefinition(description string) *Definition {
	p.expectKeyword("input")

	var def Definition
	def.Position = p.peekPos()
	def.Kind = InputObject
	def.Description = description
	def.Name = p.parseName()
	def.Directives = p.parseDirectives(true)
	def.Fields = p.parseInputFieldsDefinition()
	return &def
}

func (p *parser) parseInputFieldsDefinition() FieldList {
	var values FieldList
	p.some(lexer.BraceL, lexer.BraceR, func() {
		values = append(values, p.parseInputValueDef())
	})
	return values
}

func (p *parser) parseTypeSystemExtension(doc *SchemaDocument) {
	p.expectKeyword("extend")

	switch p.peek().Value {
	case "schema":
		doc.SchemaExtension = append(doc.SchemaExtension, p.parseSchemaExtension())
	case "scalar":
		doc.Extensions = append(doc.Extensions, p.parseScalarTypeExtension())
	case "type":
		doc.Extensions = append(doc.Extensions, p.parseObjectTypeExtension())
	case "interface":
		doc.Extensions = append(doc.Extensions, p.parseInterfaceTypeExtension())
	case "union":
		doc.Extensions = append(doc.Extensions, p.parseUnionTypeExtension())
	case "enum":
		doc.Extensions = append(doc.Extensions, p.parseEnumTypeExtension())
	case "input":
		doc.Extensions = append(doc.Extensions, p.parseInputObjectTypeExtension())
	default:
		p.unexpectedError()
	}
}

func (p *parser) parseSchemaExtension() *SchemaDefinition {
	p.expectKeyword("schema")

	var def SchemaDefinition
	def.Position = p.peekPos()
	def.Directives = p.parseDirectives(true)
	p.some(lexer.BraceL, lexer.BraceR, func() {
		def.OperationTypes = append(def.OperationTypes, p.parseOperationTypeDefinition())
	})
	if len(def.Directives) == 0 && len(def.OperationTypes) == 0 {
		p.unexpectedError()
	}
	return &def
}

func (p *parser) parseScalarTypeExtension() *Definition {
	p.expectKeyword("scalar")

	var def Definition
	def.Position = p.peekPos()
	def.Kind = Scalar
	def.Name = p.parseName()
	def.Directives = p.parseDirectives(true)
	if len(def.Directives) == 0 {
		p.unexpectedError()
	}
	return &def
}

func (p *parser) parseObjectTypeExtension() *Definition {
	p.expectKeyword("type")

	var def Definition
	def.Position = p.peekPos()
	def.Kind = Object
	def.Name = p.parseName()
	def.Interfaces = p.parseImplementsInterfaces()
	def.Directives = p.parseDirectives(true)
	def.Fields = p.parseFieldsDefinition()
	if len(def.Interfaces) == 0 && len(def.Directives) == 0 && len(def.Fields) == 0 {
		p.unexpectedError()
	}
	return &def
}

func (p *parser) parseInterfaceTypeExtension() *Definition {
	p.expectKeyword("interface")

	var def Definition
	def.Position = p.peekPos()
	def.Kind = Interface
	def.Name = p.parseName()
	def.Directives = p.parseDirectives(true)
	def.Fields = p.parseFieldsDefinition()
	if len(def.Directives) == 0 && len(def.Fields) == 0 {
		p.unexpectedError()
	}
	return &def
}

func (p *parser) parseUnionTypeExtension() *Definition {
	p.expectKeyword("union")

	var def Definition
	def.Position = p.peekPos()
	def.Kind = Union
	def.Name = p.parseName()
	def.Directives = p.parseDirectives(true)
	def.Types = p.parseUnionMemberTypes()

	if len(def.Directives) == 0 && len(def.Types) == 0 {
		p.unexpectedError()
	}
	return &def
}

func (p *parser) parseEnumTypeExtension() *Definition {
	p.expectKeyword("enum")

	var def Definition
	def.Position = p.peekPos()
	def.Kind = Enum
	def.Name = p.parseName()
	def.Directives = p.parseDirectives(true)
	def.EnumValues = p.parseEnumValuesDefinition()
	if len(def.Directives) == 0 && len(def.EnumValues) == 0 {
		p.unexpectedError()
	}
	return &def
}

func (p *parser) parseInputObjectTypeExtension() *Definition {
	p.expectKeyword("input")

	var def Definition
	def.Position = p.peekPos()
	def.Kind = InputObject
	def.Name = p.parseName()
	def.Directives = p.parseDirectives(false)
	def.Fields = p.parseInputFieldsDefinition()
	if len(def.Directives) == 0 && len(def.Fields) == 0 {
		p.unexpectedError()
	}
	return &def
}

func (p *parser) parseDirectiveDefinition(description string) *DirectiveDefinition {
	p.expectKeyword("directive")
	p.expect(lexer.At)

	var def DirectiveDefinition
	def.Position = p.peekPos()
	def.Description = description
	def.Name = p.parseName()
	def.Arguments = p.parseArgumentDefs()

	p.expectKeyword("on")
	def.Locations = p.parseDirectiveLocations()
	return &def
}

func (p *parser) parseDirectiveLocations() []DirectiveLocation {
	p.skip(lexer.Pipe)

	locations := []DirectiveLocation{p.parseDirectiveLocation()}

	for p.skip(lexer.Pipe) && p.err == nil {
		locations = append(locations, p.parseDirectiveLocation())
	}

	return locations
}

func (p *parser) parseDirectiveLocation() DirectiveLocation {
	name := p.expect(lexer.Name)

	switch name.Value {
	case `QUERY`:
		return LocationQuery
	case `MUTATION`:
		return LocationMutation
	case `SUBSCRIPTION`:
		return LocationSubscription
	case `FIELD`:
		return LocationField
	case `FRAGMENT_DEFINITION`:
		return LocationFragmentDefinition
	case `FRAGMENT_SPREAD`:
		return LocationFragmentSpread
	case `INLINE_FRAGMENT`:
		return LocationInlineFragment
	case `SCHEMA`:
		return LocationSchema
	case `SCALAR`:
		return LocationScalar
	case `OBJECT`:
		return LocationObject
	case `FIELD_DEFINITION`:
		return LocationFieldDefinition
	case `ARGUMENT_DEFINITION`:
		return LocationArgumentDefinition
	case `INTERFACE`:
		return LocationInterface
	case `UNION`:
		return LocationUnion
	case `ENUM`:
		return LocationEnum
	case `ENUM_VALUE`:
		return LocationEnumValue
	case `INPUT_OBJECT`:
		return LocationInputObject
	case `INPUT_FIELD_DEFINITION`:
		return LocationInputFieldDefinition
	}

	p.unexpectedToken(name)
	return ""
}
