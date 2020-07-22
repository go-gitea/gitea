package parser

import (
	"fmt"

	"github.com/graphql-go/graphql/gqlerrors"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/lexer"
	"github.com/graphql-go/graphql/language/source"
)

type parseFn func(parser *Parser) (interface{}, error)

// parse operation, fragment, typeSystem{schema, type..., extension, directives} definition
type parseDefinitionFn func(parser *Parser) (ast.Node, error)

var tokenDefinitionFn map[string]parseDefinitionFn

func init() {
	tokenDefinitionFn = make(map[string]parseDefinitionFn)
	{
		// for sign
		tokenDefinitionFn[lexer.BRACE_L.String()] = parseOperationDefinition
		tokenDefinitionFn[lexer.STRING.String()] = parseTypeSystemDefinition
		tokenDefinitionFn[lexer.BLOCK_STRING.String()] = parseTypeSystemDefinition
		tokenDefinitionFn[lexer.NAME.String()] = parseTypeSystemDefinition
		// for NAME
		tokenDefinitionFn[lexer.FRAGMENT] = parseFragmentDefinition
		tokenDefinitionFn[lexer.QUERY] = parseOperationDefinition
		tokenDefinitionFn[lexer.MUTATION] = parseOperationDefinition
		tokenDefinitionFn[lexer.SUBSCRIPTION] = parseOperationDefinition
		tokenDefinitionFn[lexer.SCHEMA] = parseSchemaDefinition
		tokenDefinitionFn[lexer.SCALAR] = parseScalarTypeDefinition
		tokenDefinitionFn[lexer.TYPE] = parseObjectTypeDefinition
		tokenDefinitionFn[lexer.INTERFACE] = parseInterfaceTypeDefinition
		tokenDefinitionFn[lexer.UNION] = parseUnionTypeDefinition
		tokenDefinitionFn[lexer.ENUM] = parseEnumTypeDefinition
		tokenDefinitionFn[lexer.INPUT] = parseInputObjectTypeDefinition
		tokenDefinitionFn[lexer.EXTEND] = parseTypeExtensionDefinition
		tokenDefinitionFn[lexer.DIRECTIVE] = parseDirectiveDefinition
	}
}

type ParseOptions struct {
	NoLocation bool
	NoSource   bool
}

type ParseParams struct {
	Source  interface{}
	Options ParseOptions
}

type Parser struct {
	LexToken lexer.Lexer
	Source   *source.Source
	Options  ParseOptions
	PrevEnd  int
	Token    lexer.Token
}

func Parse(p ParseParams) (*ast.Document, error) {
	var sourceObj *source.Source
	switch src := p.Source.(type) {
	case *source.Source:
		sourceObj = src
	default:
		body, _ := p.Source.(string)
		sourceObj = source.NewSource(&source.Source{Body: []byte(body)})
	}
	parser, err := makeParser(sourceObj, p.Options)
	if err != nil {
		return nil, err
	}
	doc, err := parseDocument(parser)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

// TODO: test and expose parseValue as a public
func parseValue(p ParseParams) (ast.Value, error) {
	var value ast.Value
	var sourceObj *source.Source
	switch src := p.Source.(type) {
	case *source.Source:
		sourceObj = src
	default:
		body, _ := p.Source.(string)
		sourceObj = source.NewSource(&source.Source{Body: []byte(body)})
	}
	parser, err := makeParser(sourceObj, p.Options)
	if err != nil {
		return value, err
	}
	value, err = parseValueLiteral(parser, false)
	if err != nil {
		return value, err
	}
	return value, nil
}

// Converts a name lex token into a name parse node.
func parseName(parser *Parser) (*ast.Name, error) {
	token, err := expect(parser, lexer.NAME)
	if err != nil {
		return nil, err
	}
	return ast.NewName(&ast.Name{
		Value: token.Value,
		Loc:   loc(parser, token.Start),
	}), nil
}

func makeParser(s *source.Source, opts ParseOptions) (*Parser, error) {
	lexToken := lexer.Lex(s)
	token, err := lexToken(0)
	if err != nil {
		return &Parser{}, err
	}
	return &Parser{
		LexToken: lexToken,
		Source:   s,
		Options:  opts,
		PrevEnd:  0,
		Token:    token,
	}, nil
}

/* Implements the parsing rules in the Document section. */

func parseDocument(parser *Parser) (*ast.Document, error) {
	var (
		nodes []ast.Node
		node  ast.Node
		item  parseDefinitionFn
		err   error
	)
	start := parser.Token.Start
	for {
		if skp, err := skip(parser, lexer.EOF); err != nil {
			return nil, err
		} else if skp {
			break
		}
		switch kind := parser.Token.Kind; kind {
		case lexer.BRACE_L, lexer.NAME, lexer.STRING, lexer.BLOCK_STRING:
			item = tokenDefinitionFn[kind.String()]
		default:
			return nil, unexpected(parser, lexer.Token{})
		}
		if node, err = item(parser); err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return ast.NewDocument(&ast.Document{
		Loc:         loc(parser, start),
		Definitions: nodes,
	}), nil
}

/* Implements the parsing rules in the Operations section. */

/**
 * OperationDefinition :
 *  - SelectionSet
 *  - OperationType Name? VariableDefinitions? Directives? SelectionSet
 */
func parseOperationDefinition(parser *Parser) (ast.Node, error) {
	var (
		operation           string
		variableDefinitions []*ast.VariableDefinition
		name                *ast.Name
		directives          []*ast.Directive
		selectionSet        *ast.SelectionSet
		err                 error
	)
	start := parser.Token.Start
	if peek(parser, lexer.BRACE_L) {
		selectionSet, err := parseSelectionSet(parser)
		if err != nil {
			return nil, err
		}
		return ast.NewOperationDefinition(&ast.OperationDefinition{
			Operation:    ast.OperationTypeQuery,
			Directives:   []*ast.Directive{},
			SelectionSet: selectionSet,
			Loc:          loc(parser, start),
		}), nil
	}
	if operation, err = parseOperationType(parser); err != nil {
		return nil, err
	}

	if peek(parser, lexer.NAME) {
		if name, err = parseName(parser); err != nil {
			return nil, err
		}
	}
	if variableDefinitions, err = parseVariableDefinitions(parser); err != nil {
		return nil, err
	}
	if directives, err = parseDirectives(parser); err != nil {
		return nil, err
	}
	if selectionSet, err = parseSelectionSet(parser); err != nil {
		return nil, err
	}
	return ast.NewOperationDefinition(&ast.OperationDefinition{
		Operation:           operation,
		Name:                name,
		VariableDefinitions: variableDefinitions,
		Directives:          directives,
		SelectionSet:        selectionSet,
		Loc:                 loc(parser, start),
	}), nil
}

/**
 * OperationType : one of query mutation subscription
 */
func parseOperationType(parser *Parser) (string, error) {
	operationToken, err := expect(parser, lexer.NAME)
	if err != nil {
		return "", err
	}
	switch operationToken.Value {
	case ast.OperationTypeQuery:
		return operationToken.Value, nil
	case ast.OperationTypeMutation:
		return operationToken.Value, nil
	case ast.OperationTypeSubscription:
		return operationToken.Value, nil
	default:
		return "", unexpected(parser, operationToken)
	}
}

/**
 * VariableDefinitions : ( VariableDefinition+ )
 */
func parseVariableDefinitions(parser *Parser) ([]*ast.VariableDefinition, error) {
	variableDefinitions := []*ast.VariableDefinition{}
	if !peek(parser, lexer.PAREN_L) {
		return variableDefinitions, nil
	}
	if vdefs, err := reverse(parser,
		lexer.PAREN_L, parseVariableDefinition, lexer.PAREN_R,
		true,
	); err != nil {
		return variableDefinitions, err
	} else {
		for _, vdef := range vdefs {
			variableDefinitions = append(variableDefinitions, vdef.(*ast.VariableDefinition))
		}
	}
	return variableDefinitions, nil
}

/**
 * VariableDefinition : Variable : Type DefaultValue?
 */
func parseVariableDefinition(parser *Parser) (interface{}, error) {
	var (
		variable *ast.Variable
		ttype    ast.Type
		err      error
	)
	start := parser.Token.Start
	if variable, err = parseVariable(parser); err != nil {
		return nil, err
	}
	if _, err = expect(parser, lexer.COLON); err != nil {
		return nil, err
	}
	if ttype, err = parseType(parser); err != nil {
		return nil, err
	}
	var defaultValue ast.Value
	if skp, err := skip(parser, lexer.EQUALS); err != nil {
		return nil, err
	} else if skp {
		if defaultValue, err = parseValueLiteral(parser, true); err != nil {
			return nil, err
		}
	}
	return ast.NewVariableDefinition(&ast.VariableDefinition{
		Variable:     variable,
		Type:         ttype,
		DefaultValue: defaultValue,
		Loc:          loc(parser, start),
	}), nil
}

/**
 * Variable : $ Name
 */
func parseVariable(parser *Parser) (*ast.Variable, error) {
	var (
		err  error
		name *ast.Name
	)
	start := parser.Token.Start
	if _, err = expect(parser, lexer.DOLLAR); err != nil {
		return nil, err
	}
	if name, err = parseName(parser); err != nil {
		return nil, err
	}
	return ast.NewVariable(&ast.Variable{
		Name: name,
		Loc:  loc(parser, start),
	}), nil
}

/**
 * SelectionSet : { Selection+ }
 */
func parseSelectionSet(parser *Parser) (*ast.SelectionSet, error) {
	start := parser.Token.Start
	selections := []ast.Selection{}
	if iSelections, err := reverse(parser,
		lexer.BRACE_L, parseSelection, lexer.BRACE_R,
		true,
	); err != nil {
		return nil, err
	} else {
		for _, iSelection := range iSelections {
			selections = append(selections, iSelection.(ast.Selection))
		}
	}

	return ast.NewSelectionSet(&ast.SelectionSet{
		Selections: selections,
		Loc:        loc(parser, start),
	}), nil
}

/**
 * Selection :
 *   - Field
 *   - FragmentSpread
 *   - InlineFragment
 */
func parseSelection(parser *Parser) (interface{}, error) {
	if peek(parser, lexer.SPREAD) {
		return parseFragment(parser)
	}
	return parseField(parser)
}

/**
 * Field : Alias? Name Arguments? Directives? SelectionSet?
 *
 * Alias : Name :
 */
func parseField(parser *Parser) (*ast.Field, error) {
	var (
		name       *ast.Name
		alias      *ast.Name
		arguments  []*ast.Argument
		directives []*ast.Directive
		err        error
	)
	start := parser.Token.Start
	if name, err = parseName(parser); err != nil {
		return nil, err
	}
	if skp, err := skip(parser, lexer.COLON); err != nil {
		return nil, err
	} else if skp {
		alias = name
		if name, err = parseName(parser); err != nil {
			return nil, err
		}
	}
	if arguments, err = parseArguments(parser); err != nil {
		return nil, err
	}
	if directives, err = parseDirectives(parser); err != nil {
		return nil, err
	}
	var selectionSet *ast.SelectionSet
	if peek(parser, lexer.BRACE_L) {
		if selectionSet, err = parseSelectionSet(parser); err != nil {
			return nil, err
		}
	}
	return ast.NewField(&ast.Field{
		Alias:        alias,
		Name:         name,
		Arguments:    arguments,
		Directives:   directives,
		SelectionSet: selectionSet,
		Loc:          loc(parser, start),
	}), nil
}

/**
 * Arguments : ( Argument+ )
 */
func parseArguments(parser *Parser) ([]*ast.Argument, error) {
	arguments := []*ast.Argument{}
	if peek(parser, lexer.PAREN_L) {
		if iArguments, err := reverse(parser,
			lexer.PAREN_L, parseArgument, lexer.PAREN_R,
			true,
		); err != nil {
			return arguments, err
		} else {
			for _, iArgument := range iArguments {
				arguments = append(arguments, iArgument.(*ast.Argument))
			}
		}
	}
	return arguments, nil
}

/**
 * Argument : Name : Value
 */
func parseArgument(parser *Parser) (interface{}, error) {
	var (
		err   error
		name  *ast.Name
		value ast.Value
	)
	start := parser.Token.Start
	if name, err = parseName(parser); err != nil {
		return nil, err
	}
	if _, err = expect(parser, lexer.COLON); err != nil {
		return nil, err
	}
	if value, err = parseValueLiteral(parser, false); err != nil {
		return nil, err
	}
	return ast.NewArgument(&ast.Argument{
		Name:  name,
		Value: value,
		Loc:   loc(parser, start),
	}), nil
}

/* Implements the parsing rules in the Fragments section. */

/**
 * Corresponds to both FragmentSpread and InlineFragment in the spec.
 *
 * FragmentSpread : ... FragmentName Directives?
 *
 * InlineFragment : ... TypeCondition? Directives? SelectionSet
 */
func parseFragment(parser *Parser) (interface{}, error) {
	var (
		err error
	)
	start := parser.Token.Start
	if _, err = expect(parser, lexer.SPREAD); err != nil {
		return nil, err
	}
	if peek(parser, lexer.NAME) && parser.Token.Value != "on" {
		name, err := parseFragmentName(parser)
		if err != nil {
			return nil, err
		}
		directives, err := parseDirectives(parser)
		if err != nil {
			return nil, err
		}
		return ast.NewFragmentSpread(&ast.FragmentSpread{
			Name:       name,
			Directives: directives,
			Loc:        loc(parser, start),
		}), nil
	}
	var typeCondition *ast.Named
	if parser.Token.Value == "on" {
		if err := advance(parser); err != nil {
			return nil, err
		}
		name, err := parseNamed(parser)
		if err != nil {
			return nil, err
		}
		typeCondition = name

	}
	directives, err := parseDirectives(parser)
	if err != nil {
		return nil, err
	}
	selectionSet, err := parseSelectionSet(parser)
	if err != nil {
		return nil, err
	}
	return ast.NewInlineFragment(&ast.InlineFragment{
		TypeCondition: typeCondition,
		Directives:    directives,
		SelectionSet:  selectionSet,
		Loc:           loc(parser, start),
	}), nil
}

/**
 * FragmentDefinition :
 *   - fragment FragmentName on TypeCondition Directives? SelectionSet
 *
 * TypeCondition : NamedType
 */
func parseFragmentDefinition(parser *Parser) (ast.Node, error) {
	start := parser.Token.Start
	_, err := expectKeyWord(parser, lexer.FRAGMENT)
	if err != nil {
		return nil, err
	}
	name, err := parseFragmentName(parser)
	if err != nil {
		return nil, err
	}
	_, err = expectKeyWord(parser, "on")
	if err != nil {
		return nil, err
	}
	typeCondition, err := parseNamed(parser)
	if err != nil {
		return nil, err
	}
	directives, err := parseDirectives(parser)
	if err != nil {
		return nil, err
	}
	selectionSet, err := parseSelectionSet(parser)
	if err != nil {
		return nil, err
	}
	return ast.NewFragmentDefinition(&ast.FragmentDefinition{
		Name:          name,
		TypeCondition: typeCondition,
		Directives:    directives,
		SelectionSet:  selectionSet,
		Loc:           loc(parser, start),
	}), nil
}

/**
 * FragmentName : Name but not `on`
 */
func parseFragmentName(parser *Parser) (*ast.Name, error) {
	if parser.Token.Value == "on" {
		return nil, unexpected(parser, lexer.Token{})
	}
	return parseName(parser)
}

/* Implements the parsing rules in the Values section. */

/**
 * Value[Const] :
 *   - [~Const] Variable
 *   - IntValue
 *   - FloatValue
 *   - StringValue
 *   - BooleanValue
 *   - EnumValue
 *   - ListValue[?Const]
 *   - ObjectValue[?Const]
 *
 * BooleanValue : one of `true` `false`
 *
 * EnumValue : Name but not `true`, `false` or `null`
 */
func parseValueLiteral(parser *Parser, isConst bool) (ast.Value, error) {
	token := parser.Token
	switch token.Kind {
	case lexer.BRACKET_L:
		return parseList(parser, isConst)
	case lexer.BRACE_L:
		return parseObject(parser, isConst)
	case lexer.INT:
		if err := advance(parser); err != nil {
			return nil, err
		}
		return ast.NewIntValue(&ast.IntValue{
			Value: token.Value,
			Loc:   loc(parser, token.Start),
		}), nil
	case lexer.FLOAT:
		if err := advance(parser); err != nil {
			return nil, err
		}
		return ast.NewFloatValue(&ast.FloatValue{
			Value: token.Value,
			Loc:   loc(parser, token.Start),
		}), nil
	case lexer.BLOCK_STRING, lexer.STRING:
		return parseStringLiteral(parser)
	case lexer.NAME:
		if token.Value == "true" || token.Value == "false" {
			if err := advance(parser); err != nil {
				return nil, err
			}
			value := true
			if token.Value == "false" {
				value = false
			}
			return ast.NewBooleanValue(&ast.BooleanValue{
				Value: value,
				Loc:   loc(parser, token.Start),
			}), nil
		} else if token.Value != "null" {
			if err := advance(parser); err != nil {
				return nil, err
			}
			return ast.NewEnumValue(&ast.EnumValue{
				Value: token.Value,
				Loc:   loc(parser, token.Start),
			}), nil
		}
	case lexer.DOLLAR:
		if !isConst {
			return parseVariable(parser)
		}
	}

	return nil, unexpected(parser, lexer.Token{})
}

func parseConstValue(parser *Parser) (interface{}, error) {
	value, err := parseValueLiteral(parser, true)
	if err != nil {
		return value, err
	}
	return value, nil
}

func parseValueValue(parser *Parser) (interface{}, error) {
	return parseValueLiteral(parser, false)
}

/**
 * ListValue[Const] :
 *   - [ ]
 *   - [ Value[?Const]+ ]
 */
func parseList(parser *Parser, isConst bool) (*ast.ListValue, error) {
	start := parser.Token.Start
	var item parseFn = parseValueValue
	if isConst {
		item = parseConstValue
	}
	values := []ast.Value{}
	if iValues, err := reverse(parser,
		lexer.BRACKET_L, item, lexer.BRACKET_R,
		false,
	); err != nil {
		return nil, err
	} else {
		for _, iValue := range iValues {
			values = append(values, iValue.(ast.Value))
		}
	}
	return ast.NewListValue(&ast.ListValue{
		Values: values,
		Loc:    loc(parser, start),
	}), nil
}

/**
 * ObjectValue[Const] :
 *   - { }
 *   - { ObjectField[?Const]+ }
 */
func parseObject(parser *Parser, isConst bool) (*ast.ObjectValue, error) {
	start := parser.Token.Start
	if _, err := expect(parser, lexer.BRACE_L); err != nil {
		return nil, err
	}
	fields := []*ast.ObjectField{}
	for {
		if skp, err := skip(parser, lexer.BRACE_R); err != nil {
			return nil, err
		} else if skp {
			break
		}
		if field, err := parseObjectField(parser, isConst); err != nil {
			return nil, err
		} else {
			fields = append(fields, field)
		}
	}
	return ast.NewObjectValue(&ast.ObjectValue{
		Fields: fields,
		Loc:    loc(parser, start),
	}), nil
}

/**
 * ObjectField[Const] : Name : Value[?Const]
 */
func parseObjectField(parser *Parser, isConst bool) (*ast.ObjectField, error) {
	var (
		name  *ast.Name
		value ast.Value
		err   error
	)
	start := parser.Token.Start
	if name, err = parseName(parser); err != nil {
		return nil, err
	}
	if _, err = expect(parser, lexer.COLON); err != nil {
		return nil, err
	}
	if value, err = parseValueLiteral(parser, isConst); err != nil {
		return nil, err
	}
	return ast.NewObjectField(&ast.ObjectField{
		Name:  name,
		Value: value,
		Loc:   loc(parser, start),
	}), nil
}

/* Implements the parsing rules in the Directives section. */

/**
 * Directives : Directive+
 */
func parseDirectives(parser *Parser) ([]*ast.Directive, error) {
	directives := []*ast.Directive{}
	for peek(parser, lexer.AT) {
		if directive, err := parseDirective(parser); err != nil {
			return directives, err
		} else {
			directives = append(directives, directive)
		}
	}
	return directives, nil
}

/**
 * Directive : @ Name Arguments?
 */
func parseDirective(parser *Parser) (*ast.Directive, error) {
	var (
		err  error
		name *ast.Name
		args []*ast.Argument
	)
	start := parser.Token.Start
	if _, err = expect(parser, lexer.AT); err != nil {
		return nil, err
	}
	if name, err = parseName(parser); err != nil {
		return nil, err
	}
	if args, err = parseArguments(parser); err != nil {
		return nil, err
	}
	return ast.NewDirective(&ast.Directive{
		Name:      name,
		Arguments: args,
		Loc:       loc(parser, start),
	}), nil
}

/* Implements the parsing rules in the Types section. */

/**
 * Type :
 *   - NamedType
 *   - ListType
 *   - NonNullType
 */
func parseType(parser *Parser) (ttype ast.Type, err error) {
	token := parser.Token
	// [ String! ]!
	switch token.Kind {
	case lexer.BRACKET_L:
		if err = advance(parser); err != nil {
			return nil, err
		}
		if ttype, err = parseType(parser); err != nil {
			return nil, err
		}
		fallthrough
	case lexer.BRACKET_R:
		if err = advance(parser); err != nil {
			return nil, err
		}
		ttype = ast.NewList(&ast.List{
			Type: ttype,
			Loc:  loc(parser, token.Start),
		})
	case lexer.NAME:
		if ttype, err = parseNamed(parser); err != nil {
			return nil, err
		}
	}

	// BANG must be executed
	if skp, err := skip(parser, lexer.BANG); err != nil {
		return nil, err
	} else if skp {
		ttype = ast.NewNonNull(&ast.NonNull{
			Type: ttype,
			Loc:  loc(parser, token.Start),
		})
	}
	return ttype, nil
}

/**
 * NamedType : Name
 */
func parseNamed(parser *Parser) (*ast.Named, error) {
	start := parser.Token.Start
	name, err := parseName(parser)
	if err != nil {
		return nil, err
	}
	return ast.NewNamed(&ast.Named{
		Name: name,
		Loc:  loc(parser, start),
	}), nil
}

/* Implements the parsing rules in the Type Definition section. */

/**
 * TypeSystemDefinition :
 *   - SchemaDefinition
 *   - TypeDefinition
 *   - TypeExtension
 *   - DirectiveDefinition
 *
 * TypeDefinition :
 *   - ScalarTypeDefinition
 *   - ObjectTypeDefinition
 *   - InterfaceTypeDefinition
 *   - UnionTypeDefinition
 *   - EnumTypeDefinition
 *   - InputObjectTypeDefinition
 */
func parseTypeSystemDefinition(parser *Parser) (ast.Node, error) {
	var (
		item parseDefinitionFn
		err  error
	)
	// Many definitions begin with a description and require a lookahead.
	keywordToken := parser.Token
	if peekDescription(parser) {
		if keywordToken, err = lookahead(parser); err != nil {
			return nil, err
		}
	}

	if keywordToken.Kind != lexer.NAME {
		return nil, unexpected(parser, keywordToken)
	}
	var ok bool
	if item, ok = tokenDefinitionFn[keywordToken.Value]; !ok {
		return nil, unexpected(parser, keywordToken)
	}
	return item(parser)
}

/**
 * SchemaDefinition : schema Directives? { OperationTypeDefinition+ }
 *
 * OperationTypeDefinition : OperationType : NamedType
 */
func parseSchemaDefinition(parser *Parser) (ast.Node, error) {
	start := parser.Token.Start
	_, err := expectKeyWord(parser, "schema")
	if err != nil {
		return nil, err
	}
	directives, err := parseDirectives(parser)
	if err != nil {
		return nil, err
	}
	operationTypesI, err := reverse(
		parser,
		lexer.BRACE_L, parseOperationTypeDefinition, lexer.BRACE_R,
		true,
	)
	if err != nil {
		return nil, err
	}
	operationTypes := []*ast.OperationTypeDefinition{}
	for _, op := range operationTypesI {
		if op, ok := op.(*ast.OperationTypeDefinition); ok {
			operationTypes = append(operationTypes, op)
		}
	}
	return ast.NewSchemaDefinition(&ast.SchemaDefinition{
		OperationTypes: operationTypes,
		Directives:     directives,
		Loc:            loc(parser, start),
	}), nil
}

func parseOperationTypeDefinition(parser *Parser) (interface{}, error) {
	start := parser.Token.Start
	operation, err := parseOperationType(parser)
	if err != nil {
		return nil, err
	}
	_, err = expect(parser, lexer.COLON)
	if err != nil {
		return nil, err
	}
	ttype, err := parseNamed(parser)
	if err != nil {
		return nil, err
	}
	return ast.NewOperationTypeDefinition(&ast.OperationTypeDefinition{
		Operation: operation,
		Type:      ttype,
		Loc:       loc(parser, start),
	}), nil
}

/**
 * ScalarTypeDefinition : Description? scalar Name Directives?
 */
func parseScalarTypeDefinition(parser *Parser) (ast.Node, error) {
	start := parser.Token.Start
	description, err := parseDescription(parser)
	if err != nil {
		return nil, err
	}
	_, err = expectKeyWord(parser, lexer.SCALAR)
	if err != nil {
		return nil, err
	}
	name, err := parseName(parser)
	if err != nil {
		return nil, err
	}
	directives, err := parseDirectives(parser)
	if err != nil {
		return nil, err
	}
	def := ast.NewScalarDefinition(&ast.ScalarDefinition{
		Name:        name,
		Description: description,
		Directives:  directives,
		Loc:         loc(parser, start),
	})
	return def, nil
}

/**
 * ObjectTypeDefinition :
 *   Description?
 *   type Name ImplementsInterfaces? Directives? { FieldDefinition+ }
 */
func parseObjectTypeDefinition(parser *Parser) (ast.Node, error) {
	start := parser.Token.Start
	description, err := parseDescription(parser)
	if err != nil {
		return nil, err
	}
	_, err = expectKeyWord(parser, lexer.TYPE)
	if err != nil {
		return nil, err
	}
	name, err := parseName(parser)
	if err != nil {
		return nil, err
	}
	interfaces, err := parseImplementsInterfaces(parser)
	if err != nil {
		return nil, err
	}
	directives, err := parseDirectives(parser)
	if err != nil {
		return nil, err
	}
	iFields, err := reverse(parser,
		lexer.BRACE_L, parseFieldDefinition, lexer.BRACE_R,
		false,
	)
	if err != nil {
		return nil, err
	}
	fields := []*ast.FieldDefinition{}
	for _, iField := range iFields {
		if iField != nil {
			fields = append(fields, iField.(*ast.FieldDefinition))
		}
	}
	return ast.NewObjectDefinition(&ast.ObjectDefinition{
		Name:        name,
		Description: description,
		Loc:         loc(parser, start),
		Interfaces:  interfaces,
		Directives:  directives,
		Fields:      fields,
	}), nil
}

/**
 * ImplementsInterfaces :
 *   - implements `&`? NamedType
 *   - ImplementsInterfaces & NamedType
 */
func parseImplementsInterfaces(parser *Parser) ([]*ast.Named, error) {
	types := []*ast.Named{}
	if parser.Token.Value == "implements" {
		if err := advance(parser); err != nil {
			return nil, err
		}
		// optional leading ampersand
		skip(parser, lexer.AMP)
		for {
			ttype, err := parseNamed(parser)
			if err != nil {
				return types, err
			}
			types = append(types, ttype)
			if skipped, err := skip(parser, lexer.AMP); !skipped {
				break
			} else if err != nil {
				return types, err
			}
		}
	}
	return types, nil
}

/**
 * FieldDefinition : Description? Name ArgumentsDefinition? : Type Directives?
 */
func parseFieldDefinition(parser *Parser) (interface{}, error) {
	start := parser.Token.Start
	description, err := parseDescription(parser)
	if err != nil {
		return nil, err
	}
	name, err := parseName(parser)
	if err != nil {
		return nil, err
	}
	args, err := parseArgumentDefs(parser)
	if err != nil {
		return nil, err
	}
	_, err = expect(parser, lexer.COLON)
	if err != nil {
		return nil, err
	}
	ttype, err := parseType(parser)
	if err != nil {
		return nil, err
	}
	directives, err := parseDirectives(parser)
	if err != nil {
		return nil, err
	}
	return ast.NewFieldDefinition(&ast.FieldDefinition{
		Name:        name,
		Description: description,
		Arguments:   args,
		Type:        ttype,
		Directives:  directives,
		Loc:         loc(parser, start),
	}), nil
}

/**
 * ArgumentsDefinition : ( InputValueDefinition+ )
 */
func parseArgumentDefs(parser *Parser) ([]*ast.InputValueDefinition, error) {
	inputValueDefinitions := []*ast.InputValueDefinition{}

	if !peek(parser, lexer.PAREN_L) {
		return inputValueDefinitions, nil
	}
	iInputValueDefinitions, err := reverse(parser,
		lexer.PAREN_L, parseInputValueDef, lexer.PAREN_R,
		true,
	)
	if err != nil {
		return inputValueDefinitions, err
	}
	for _, iInputValueDefinition := range iInputValueDefinitions {
		if iInputValueDefinition != nil {
			inputValueDefinitions = append(inputValueDefinitions, iInputValueDefinition.(*ast.InputValueDefinition))
		}
	}
	return inputValueDefinitions, err
}

/**
 * InputValueDefinition : Description? Name : Type DefaultValue? Directives?
 */
func parseInputValueDef(parser *Parser) (interface{}, error) {
	var (
		description *ast.StringValue
		name        *ast.Name
		ttype       ast.Type
		directives  []*ast.Directive
		err         error
	)
	start := parser.Token.Start
	if description, err = parseDescription(parser); err != nil {
		return nil, err
	}
	if name, err = parseName(parser); err != nil {
		return nil, err
	}
	if _, err = expect(parser, lexer.COLON); err != nil {
		return nil, err
	}
	if ttype, err = parseType(parser); err != nil {
		return nil, err
	}
	var defaultValue ast.Value
	if skp, err := skip(parser, lexer.EQUALS); err != nil {
		return nil, err
	} else if skp {
		val, err := parseConstValue(parser)
		if err != nil {
			return nil, err
		}
		if val, ok := val.(ast.Value); ok {
			defaultValue = val
		}
	}
	if directives, err = parseDirectives(parser); err != nil {
		return nil, err
	}
	return ast.NewInputValueDefinition(&ast.InputValueDefinition{
		Name:         name,
		Description:  description,
		Type:         ttype,
		DefaultValue: defaultValue,
		Directives:   directives,
		Loc:          loc(parser, start),
	}), nil
}

/**
 * InterfaceTypeDefinition :
 *   Description?
 *   interface Name Directives? { FieldDefinition+ }
 */
func parseInterfaceTypeDefinition(parser *Parser) (ast.Node, error) {
	start := parser.Token.Start
	description, err := parseDescription(parser)
	if err != nil {
		return nil, err
	}
	_, err = expectKeyWord(parser, lexer.INTERFACE)
	if err != nil {
		return nil, err
	}
	name, err := parseName(parser)
	if err != nil {
		return nil, err
	}
	directives, err := parseDirectives(parser)
	if err != nil {
		return nil, err
	}
	iFields, err := reverse(parser,
		lexer.BRACE_L, parseFieldDefinition, lexer.BRACE_R,
		false,
	)
	if err != nil {
		return nil, err
	}
	fields := []*ast.FieldDefinition{}
	for _, iField := range iFields {
		if iField != nil {
			fields = append(fields, iField.(*ast.FieldDefinition))
		}
	}
	return ast.NewInterfaceDefinition(&ast.InterfaceDefinition{
		Name:        name,
		Description: description,
		Directives:  directives,
		Loc:         loc(parser, start),
		Fields:      fields,
	}), nil
}

/**
 * UnionTypeDefinition : Description? union Name Directives? = UnionMembers
 */
func parseUnionTypeDefinition(parser *Parser) (ast.Node, error) {
	start := parser.Token.Start
	description, err := parseDescription(parser)
	if err != nil {
		return nil, err
	}
	_, err = expectKeyWord(parser, lexer.UNION)
	if err != nil {
		return nil, err
	}
	name, err := parseName(parser)
	if err != nil {
		return nil, err
	}
	directives, err := parseDirectives(parser)
	if err != nil {
		return nil, err
	}
	_, err = expect(parser, lexer.EQUALS)
	if err != nil {
		return nil, err
	}
	types, err := parseUnionMembers(parser)
	if err != nil {
		return nil, err
	}
	return ast.NewUnionDefinition(&ast.UnionDefinition{
		Name:        name,
		Description: description,
		Directives:  directives,
		Loc:         loc(parser, start),
		Types:       types,
	}), nil
}

/**
 * UnionMembers :
 *   - NamedType
 *   - UnionMembers | NamedType
 */
func parseUnionMembers(parser *Parser) ([]*ast.Named, error) {
	members := []*ast.Named{}
	for {
		member, err := parseNamed(parser)
		if err != nil {
			return members, err
		}
		members = append(members, member)
		if skp, err := skip(parser, lexer.PIPE); err != nil {
			return nil, err
		} else if !skp {
			break
		}
	}
	return members, nil
}

/**
 * EnumTypeDefinition : Description? enum Name Directives? { EnumValueDefinition+ }
 */
func parseEnumTypeDefinition(parser *Parser) (ast.Node, error) {
	start := parser.Token.Start
	description, err := parseDescription(parser)
	if err != nil {
		return nil, err
	}
	_, err = expectKeyWord(parser, lexer.ENUM)
	if err != nil {
		return nil, err
	}
	name, err := parseName(parser)
	if err != nil {
		return nil, err
	}
	directives, err := parseDirectives(parser)
	if err != nil {
		return nil, err
	}
	iEnumValueDefs, err := reverse(parser,
		lexer.BRACE_L, parseEnumValueDefinition, lexer.BRACE_R,
		false,
	)
	if err != nil {
		return nil, err
	}
	values := []*ast.EnumValueDefinition{}
	for _, iEnumValueDef := range iEnumValueDefs {
		if iEnumValueDef != nil {
			values = append(values, iEnumValueDef.(*ast.EnumValueDefinition))
		}
	}
	return ast.NewEnumDefinition(&ast.EnumDefinition{
		Name:        name,
		Description: description,
		Directives:  directives,
		Loc:         loc(parser, start),
		Values:      values,
	}), nil
}

/**
 * EnumValueDefinition : Description? EnumValue Directives?
 *
 * EnumValue : Name
 */
func parseEnumValueDefinition(parser *Parser) (interface{}, error) {
	start := parser.Token.Start
	description, err := parseDescription(parser)
	if err != nil {
		return nil, err
	}
	name, err := parseName(parser)
	if err != nil {
		return nil, err
	}
	directives, err := parseDirectives(parser)
	if err != nil {
		return nil, err
	}
	return ast.NewEnumValueDefinition(&ast.EnumValueDefinition{
		Name:        name,
		Description: description,
		Directives:  directives,
		Loc:         loc(parser, start),
	}), nil
}

/**
 * InputObjectTypeDefinition :
 *   - Description? input Name Directives? { InputValueDefinition+ }
 */
func parseInputObjectTypeDefinition(parser *Parser) (ast.Node, error) {
	start := parser.Token.Start
	description, err := parseDescription(parser)
	if err != nil {
		return nil, err
	}
	_, err = expectKeyWord(parser, lexer.INPUT)
	if err != nil {
		return nil, err
	}
	name, err := parseName(parser)
	if err != nil {
		return nil, err
	}
	directives, err := parseDirectives(parser)
	if err != nil {
		return nil, err
	}
	iInputValueDefinitions, err := reverse(parser,
		lexer.BRACE_L, parseInputValueDef, lexer.BRACE_R,
		false,
	)
	if err != nil {
		return nil, err
	}
	fields := []*ast.InputValueDefinition{}
	for _, iInputValueDefinition := range iInputValueDefinitions {
		if iInputValueDefinition != nil {
			fields = append(fields, iInputValueDefinition.(*ast.InputValueDefinition))
		}
	}
	return ast.NewInputObjectDefinition(&ast.InputObjectDefinition{
		Name:        name,
		Description: description,
		Directives:  directives,
		Loc:         loc(parser, start),
		Fields:      fields,
	}), nil
}

/**
 * TypeExtensionDefinition : extend ObjectTypeDefinition
 */
func parseTypeExtensionDefinition(parser *Parser) (ast.Node, error) {
	start := parser.Token.Start
	_, err := expectKeyWord(parser, lexer.EXTEND)
	if err != nil {
		return nil, err
	}

	definition, err := parseObjectTypeDefinition(parser)
	if err != nil {
		return nil, err
	}
	return ast.NewTypeExtensionDefinition(&ast.TypeExtensionDefinition{
		Loc:        loc(parser, start),
		Definition: definition.(*ast.ObjectDefinition),
	}), nil
}

/**
 * DirectiveDefinition :
 *   - directive @ Name ArgumentsDefinition? on DirectiveLocations
 */
func parseDirectiveDefinition(parser *Parser) (ast.Node, error) {
	var (
		err         error
		description *ast.StringValue
		name        *ast.Name
		args        []*ast.InputValueDefinition
		locations   []*ast.Name
	)
	start := parser.Token.Start
	if description, err = parseDescription(parser); err != nil {
		return nil, err
	}
	if _, err = expectKeyWord(parser, lexer.DIRECTIVE); err != nil {
		return nil, err
	}
	if _, err = expect(parser, lexer.AT); err != nil {
		return nil, err
	}
	if name, err = parseName(parser); err != nil {
		return nil, err
	}
	if args, err = parseArgumentDefs(parser); err != nil {
		return nil, err
	}
	if _, err = expectKeyWord(parser, "on"); err != nil {
		return nil, err
	}
	if locations, err = parseDirectiveLocations(parser); err != nil {
		return nil, err
	}

	return ast.NewDirectiveDefinition(&ast.DirectiveDefinition{
		Loc:         loc(parser, start),
		Name:        name,
		Description: description,
		Arguments:   args,
		Locations:   locations,
	}), nil
}

/**
 * DirectiveLocations :
 *   - Name
 *   - DirectiveLocations | Name
 */
func parseDirectiveLocations(parser *Parser) ([]*ast.Name, error) {
	locations := []*ast.Name{}
	for {
		if name, err := parseName(parser); err != nil {
			return locations, err
		} else {
			locations = append(locations, name)
		}

		if hasPipe, err := skip(parser, lexer.PIPE); err != nil {
			return locations, err
		} else if !hasPipe {
			break
		}
	}
	return locations, nil
}

func parseStringLiteral(parser *Parser) (*ast.StringValue, error) {
	token := parser.Token
	if err := advance(parser); err != nil {
		return nil, err
	}
	return ast.NewStringValue(&ast.StringValue{
		Value: token.Value,
		Loc:   loc(parser, token.Start),
	}), nil
}

/**
 * Description : StringValue
 */
func parseDescription(parser *Parser) (*ast.StringValue, error) {
	if peekDescription(parser) {
		return parseStringLiteral(parser)
	}
	return nil, nil
}

/* Core parsing utility functions */

// Returns a location object, used to identify the place in
// the source that created a given parsed object.
func loc(parser *Parser, start int) *ast.Location {
	if parser.Options.NoLocation {
		return nil
	}
	if parser.Options.NoSource {
		return ast.NewLocation(&ast.Location{
			Start: start,
			End:   parser.PrevEnd,
		})
	}
	return ast.NewLocation(&ast.Location{
		Start:  start,
		End:    parser.PrevEnd,
		Source: parser.Source,
	})
}

// Moves the internal parser object to the next lexed token.
func advance(parser *Parser) error {
	parser.PrevEnd = parser.Token.End
	token, err := parser.LexToken(parser.PrevEnd)
	if err != nil {
		return err
	}
	parser.Token = token
	return nil
}

// lookahead retrieves the next token
func lookahead(parser *Parser) (lexer.Token, error) {
	return parser.LexToken(parser.Token.End)
}

// Determines if the next token is of a given kind
func peek(parser *Parser, Kind lexer.TokenKind) bool {
	return parser.Token.Kind == Kind
}

// peekDescription determines if the next token is a string value
func peekDescription(parser *Parser) bool {
	return peek(parser, lexer.STRING) || peek(parser, lexer.BLOCK_STRING)
}

// If the next token is of the given kind, return true after advancing
// the parser. Otherwise, do not change the parser state and return false.
func skip(parser *Parser, Kind lexer.TokenKind) (bool, error) {
	if parser.Token.Kind == Kind {
		return true, advance(parser)
	}
	return false, nil
}

// If the next token is of the given kind, return that token after advancing
// the parser. Otherwise, do not change the parser state and return error.
func expect(parser *Parser, kind lexer.TokenKind) (lexer.Token, error) {
	token := parser.Token
	if token.Kind == kind {
		return token, advance(parser)
	}
	descp := fmt.Sprintf("Expected %s, found %s", kind, lexer.GetTokenDesc(token))
	return token, gqlerrors.NewSyntaxError(parser.Source, token.Start, descp)
}

// If the next token is a keyword with the given value, return that token after
// advancing the parser. Otherwise, do not change the parser state and return false.
func expectKeyWord(parser *Parser, value string) (lexer.Token, error) {
	token := parser.Token
	if token.Kind == lexer.NAME && token.Value == value {
		return token, advance(parser)
	}
	descp := fmt.Sprintf("Expected \"%s\", found %s", value, lexer.GetTokenDesc(token))
	return token, gqlerrors.NewSyntaxError(parser.Source, token.Start, descp)
}

// Helper function for creating an error when an unexpected lexed token
// is encountered.
func unexpected(parser *Parser, atToken lexer.Token) error {
	var token = atToken
	if (atToken == lexer.Token{}) {
		token = parser.Token
	}
	description := fmt.Sprintf("Unexpected %v", lexer.GetTokenDesc(token))
	return gqlerrors.NewSyntaxError(parser.Source, token.Start, description)
}

func unexpectedEmpty(parser *Parser, beginLoc int, openKind, closeKind lexer.TokenKind) error {
	description := fmt.Sprintf("Unexpected empty IN %s%s", openKind, closeKind)
	return gqlerrors.NewSyntaxError(parser.Source, beginLoc, description)
}

//  Returns list of parse nodes, determined by
// the parseFn. This list begins with a lex token of openKind
// and ends with a lex token of closeKind. Advances the parser
// to the next lex token after the closing token.
// if zinteger is true, len(nodes) > 0
func reverse(parser *Parser, openKind lexer.TokenKind, parseFn parseFn, closeKind lexer.TokenKind, zinteger bool) ([]interface{}, error) {
	token, err := expect(parser, openKind)
	if err != nil {
		return nil, err
	}
	var nodes []interface{}
	for {
		if skp, err := skip(parser, closeKind); err != nil {
			return nil, err
		} else if skp {
			break
		}
		node, err := parseFn(parser)
		if err != nil {
			return nodes, err
		}
		nodes = append(nodes, node)
	}
	if zinteger && len(nodes) == 0 {
		return nodes, unexpectedEmpty(parser, token.Start, openKind, closeKind)
	}
	return nodes, nil
}
