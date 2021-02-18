package parser

import (
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/lexer"

	. "github.com/vektah/gqlparser/v2/ast"
)

func ParseQuery(source *Source) (*QueryDocument, *gqlerror.Error) {
	p := parser{
		lexer: lexer.New(source),
	}
	return p.parseQueryDocument(), p.err
}

func (p *parser) parseQueryDocument() *QueryDocument {
	var doc QueryDocument
	for p.peek().Kind != lexer.EOF {
		if p.err != nil {
			return &doc
		}
		doc.Position = p.peekPos()
		switch p.peek().Kind {
		case lexer.Name:
			switch p.peek().Value {
			case "query", "mutation", "subscription":
				doc.Operations = append(doc.Operations, p.parseOperationDefinition())
			case "fragment":
				doc.Fragments = append(doc.Fragments, p.parseFragmentDefinition())
			default:
				p.unexpectedError()
			}
		case lexer.BraceL:
			doc.Operations = append(doc.Operations, p.parseOperationDefinition())
		default:
			p.unexpectedError()
		}
	}

	return &doc
}

func (p *parser) parseOperationDefinition() *OperationDefinition {
	if p.peek().Kind == lexer.BraceL {
		return &OperationDefinition{
			Position:     p.peekPos(),
			Operation:    Query,
			SelectionSet: p.parseRequiredSelectionSet(),
		}
	}

	var od OperationDefinition
	od.Position = p.peekPos()
	od.Operation = p.parseOperationType()

	if p.peek().Kind == lexer.Name {
		od.Name = p.next().Value
	}

	od.VariableDefinitions = p.parseVariableDefinitions()
	od.Directives = p.parseDirectives(false)
	od.SelectionSet = p.parseRequiredSelectionSet()

	return &od
}

func (p *parser) parseOperationType() Operation {
	tok := p.next()
	switch tok.Value {
	case "query":
		return Query
	case "mutation":
		return Mutation
	case "subscription":
		return Subscription
	}
	p.unexpectedToken(tok)
	return ""
}

func (p *parser) parseVariableDefinitions() VariableDefinitionList {
	var defs []*VariableDefinition
	p.many(lexer.ParenL, lexer.ParenR, func() {
		defs = append(defs, p.parseVariableDefinition())
	})

	return defs
}

func (p *parser) parseVariableDefinition() *VariableDefinition {
	var def VariableDefinition
	def.Position = p.peekPos()
	def.Variable = p.parseVariable()

	p.expect(lexer.Colon)

	def.Type = p.parseTypeReference()

	if p.skip(lexer.Equals) {
		def.DefaultValue = p.parseValueLiteral(true)
	}

	return &def
}

func (p *parser) parseVariable() string {
	p.expect(lexer.Dollar)
	return p.parseName()
}

func (p *parser) parseOptionalSelectionSet() SelectionSet {
	var selections []Selection
	p.some(lexer.BraceL, lexer.BraceR, func() {
		selections = append(selections, p.parseSelection())
	})

	return SelectionSet(selections)
}

func (p *parser) parseRequiredSelectionSet() SelectionSet {
	if p.peek().Kind != lexer.BraceL {
		p.error(p.peek(), "Expected %s, found %s", lexer.BraceL, p.peek().Kind.String())
		return nil
	}

	var selections []Selection
	p.some(lexer.BraceL, lexer.BraceR, func() {
		selections = append(selections, p.parseSelection())
	})

	return SelectionSet(selections)
}

func (p *parser) parseSelection() Selection {
	if p.peek().Kind == lexer.Spread {
		return p.parseFragment()
	}
	return p.parseField()
}

func (p *parser) parseField() *Field {
	var field Field
	field.Position = p.peekPos()
	field.Alias = p.parseName()

	if p.skip(lexer.Colon) {
		field.Name = p.parseName()
	} else {
		field.Name = field.Alias
	}

	field.Arguments = p.parseArguments(false)
	field.Directives = p.parseDirectives(false)
	if p.peek().Kind == lexer.BraceL {
		field.SelectionSet = p.parseOptionalSelectionSet()
	}

	return &field
}

func (p *parser) parseArguments(isConst bool) ArgumentList {
	var arguments ArgumentList
	p.many(lexer.ParenL, lexer.ParenR, func() {
		arguments = append(arguments, p.parseArgument(isConst))
	})

	return arguments
}

func (p *parser) parseArgument(isConst bool) *Argument {
	arg := Argument{}
	arg.Position = p.peekPos()
	arg.Name = p.parseName()
	p.expect(lexer.Colon)

	arg.Value = p.parseValueLiteral(isConst)
	return &arg
}

func (p *parser) parseFragment() Selection {
	p.expect(lexer.Spread)

	if peek := p.peek(); peek.Kind == lexer.Name && peek.Value != "on" {
		return &FragmentSpread{
			Position:   p.peekPos(),
			Name:       p.parseFragmentName(),
			Directives: p.parseDirectives(false),
		}
	}

	var def InlineFragment
	def.Position = p.peekPos()
	if p.peek().Value == "on" {
		p.next() // "on"

		def.TypeCondition = p.parseName()
	}

	def.Directives = p.parseDirectives(false)
	def.SelectionSet = p.parseRequiredSelectionSet()
	return &def
}

func (p *parser) parseFragmentDefinition() *FragmentDefinition {
	var def FragmentDefinition
	def.Position = p.peekPos()
	p.expectKeyword("fragment")

	def.Name = p.parseFragmentName()
	def.VariableDefinition = p.parseVariableDefinitions()

	p.expectKeyword("on")

	def.TypeCondition = p.parseName()
	def.Directives = p.parseDirectives(false)
	def.SelectionSet = p.parseRequiredSelectionSet()
	return &def
}

func (p *parser) parseFragmentName() string {
	if p.peek().Value == "on" {
		p.unexpectedError()
		return ""
	}

	return p.parseName()
}

func (p *parser) parseValueLiteral(isConst bool) *Value {
	token := p.peek()

	var kind ValueKind
	switch token.Kind {
	case lexer.BracketL:
		return p.parseList(isConst)
	case lexer.BraceL:
		return p.parseObject(isConst)
	case lexer.Dollar:
		if isConst {
			p.unexpectedError()
			return nil
		}
		return &Value{Position: &token.Pos, Raw: p.parseVariable(), Kind: Variable}
	case lexer.Int:
		kind = IntValue
	case lexer.Float:
		kind = FloatValue
	case lexer.String:
		kind = StringValue
	case lexer.BlockString:
		kind = BlockValue
	case lexer.Name:
		switch token.Value {
		case "true", "false":
			kind = BooleanValue
		case "null":
			kind = NullValue
		default:
			kind = EnumValue
		}
	default:
		p.unexpectedError()
		return nil
	}

	p.next()

	return &Value{Position: &token.Pos, Raw: token.Value, Kind: kind}
}

func (p *parser) parseList(isConst bool) *Value {
	var values ChildValueList
	pos := p.peekPos()
	p.many(lexer.BracketL, lexer.BracketR, func() {
		values = append(values, &ChildValue{Value: p.parseValueLiteral(isConst)})
	})

	return &Value{Children: values, Kind: ListValue, Position: pos}
}

func (p *parser) parseObject(isConst bool) *Value {
	var fields ChildValueList
	pos := p.peekPos()
	p.many(lexer.BraceL, lexer.BraceR, func() {
		fields = append(fields, p.parseObjectField(isConst))
	})

	return &Value{Children: fields, Kind: ObjectValue, Position: pos}
}

func (p *parser) parseObjectField(isConst bool) *ChildValue {
	field := ChildValue{}
	field.Position = p.peekPos()
	field.Name = p.parseName()

	p.expect(lexer.Colon)

	field.Value = p.parseValueLiteral(isConst)
	return &field
}

func (p *parser) parseDirectives(isConst bool) []*Directive {
	var directives []*Directive

	for p.peek().Kind == lexer.At {
		if p.err != nil {
			break
		}
		directives = append(directives, p.parseDirective(isConst))
	}
	return directives
}

func (p *parser) parseDirective(isConst bool) *Directive {
	p.expect(lexer.At)

	return &Directive{
		Position:  p.peekPos(),
		Name:      p.parseName(),
		Arguments: p.parseArguments(isConst),
	}
}

func (p *parser) parseTypeReference() *Type {
	var typ Type

	if p.skip(lexer.BracketL) {
		typ.Position = p.peekPos()
		typ.Elem = p.parseTypeReference()
		p.expect(lexer.BracketR)
	} else {
		typ.Position = p.peekPos()
		typ.NamedType = p.parseName()
	}

	if p.skip(lexer.Bang) {
		typ.Position = p.peekPos()
		typ.NonNull = true
	}
	return &typ
}

func (p *parser) parseName() string {
	token := p.expect(lexer.Name)

	return token.Value
}
